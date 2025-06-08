package pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/subash2121/gosling/pkg/future"
	"github.com/subash2121/gosling/pkg/task"
)

type Pool[T any] interface {
	// Start initiates the worker pool
	Start() error

	// Stop gracefully shuts down the worker pool
	Stop() error

	// Submit adds a task to the pool and returns a Future
	Submit(t task.Task[T]) (future.Future[T], error)

	// SubmitBatch adds multiple tasks to the pool
	SubmitBatch(tasks []future.Future[T]) ([]future.Future[T], error)

	// Resize changes the number of workers in the pool
	Resize(size uint) error

	// Stats returns current statistics about the pool
	Stats() PoolStats
}

type poolImpl[T any] struct {
	options        PoolOptions
	tasks          chan *taskWrapper[T]
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	completedTasks atomic.Uint64
	failedTasks    atomic.Uint64
	activeWorkers  atomic.Uint64
	isRunning      bool
	mu             sync.Mutex
	// Timing metrics
	totalProcessingTime atomic.Int64 // Total time spent processing tasks in nanoseconds
	totalWaitTime       atomic.Int64 // Total time tasks spent in queue in nanoseconds
	totalIdleTime       atomic.Int64 // Total time workers spent idle in nanoseconds
}

type PoolOptions struct {
	Workers           uint          // Number of workers goroutines
	QueueSize         uint          // Size of the task queue
	WorkerIdleTimeout time.Duration // Timeout for idle workers
	TaskTimeout       time.Duration // Timeout for tasks
	EnablePriority    bool          // Enable task priority
	MaxRetries        uint          // Maximum number of retries for failed tasks
	RetryBackoff      time.Duration // Backoff time between retries
	EnableMetrics     bool          // Enable metrics collection
}

func DefaultPoolOptions() PoolOptions {
	return PoolOptions{
		Workers:           5,
		QueueSize:         100,
		WorkerIdleTimeout: 60 * time.Second,
		TaskTimeout:       30 * time.Second,
		EnablePriority:    false,
		MaxRetries:        0,
		RetryBackoff:      100 * time.Millisecond,
		EnableMetrics:     true,
	}
}

type PoolStats struct {
	ActiveWorkers   uint64
	QueuedTasks     uint64
	CompletedTasks  uint64
	FailedTasks     uint64
	ProcessingTime  time.Duration // Avg time taken to process a task
	AverageWaitTime time.Duration // Avg time spent waiting in the queue
	IdleTime        time.Duration // Avg time spent by workers in idle state
}

type Option func(options *PoolOptions)

func WithWorkers(workers uint) Option {
	return func(options *PoolOptions) {
		options.Workers = workers
	}
}

func WithQueueSize(n uint) Option {
	return func(o *PoolOptions) {
		if n > 0 {
			o.QueueSize = n
		}
	}
}

func WithTaskTimeout(d time.Duration) Option {
	return func(o *PoolOptions) {
		if d > 0 {
			o.TaskTimeout = d
		}
	}
}

func WithPriorities(enable bool) Option {
	return func(o *PoolOptions) {
		o.EnablePriority = enable
	}
}

func NewTaskPool[T any](ctx context.Context, opts ...Option) Pool[T] {
	options := DefaultPoolOptions()
	for _, opt := range opts {
		opt(&options)
	}

	ctx, cancel := context.WithCancel(ctx)
	return &poolImpl[T]{
		options: options,
		tasks:   make(chan *taskWrapper[T], options.QueueSize),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// taskWrapper combines a task with its future for result handling
type taskWrapper[T any] struct {
	task       task.Task[T]
	future     future.Future[T]
	submitTime time.Time
}

func (p *poolImpl[T]) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return errors.New("pool is already running")
	}

	p.isRunning = true

	// Start workers
	for i := uint(0); i < p.options.Workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	return nil
}

// worker runs in its own goroutine and processes tasks from the pool
func (p *poolImpl[T]) worker() {
	defer p.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// Log the panic and decrement active workers
			p.activeWorkers.Add(^uint64(0)) // Decrement by 1
			p.failedTasks.Add(1)
		}
	}()

	var lastTaskEnd time.Time
	for {
		taskStart := time.Now()
		if !lastTaskEnd.IsZero() {
			// Record idle time since last task
			idleTime := taskStart.Sub(lastTaskEnd)
			p.totalIdleTime.Add(int64(idleTime))
		}

		select {
		case <-p.ctx.Done():
			// Pool is shutting down
			return
		case wrapped, ok := <-p.tasks:
			if !ok {
				// Task channel was closed
				return
			}

			func() {
				// Use defer to ensure cleanup happens even if task panics
				defer func() {
					p.activeWorkers.Add(^uint64(0)) // Decrement by 1
				}()

				// Track active workers for metrics
				p.activeWorkers.Add(1)

				// Record wait time
				waitTime := time.Since(wrapped.submitTime)
				p.totalWaitTime.Add(int64(waitTime))

				// Create a timeout context for the task
				taskCtx, cancel := context.WithTimeout(wrapped.task.Context(), p.options.TaskTimeout)
				defer cancel()
				wrapped.task = wrapped.task.WithContext(taskCtx)

				// Execute task with panic recovery
				var result T
				var err error
				execStart := time.Now()
				func() {
					defer func() {
						if r := recover(); r != nil {
							err = fmt.Errorf("task panicked: %v", r)
						}
					}()
					result, err = wrapped.task.Execute()
				}()
				execTime := time.Since(execStart)
				p.totalProcessingTime.Add(int64(execTime))

				// Complete the future with the task result
				if f, ok := wrapped.future.(interface {
					GetFutureImpl() interface{ Complete(T, error) }
				}); ok {
					f.GetFutureImpl().Complete(result, err)
				}

				// Update pool statistics
				if err != nil {
					p.failedTasks.Add(1)
				} else {
					p.completedTasks.Add(1)
				}

				lastTaskEnd = time.Now()
			}()
		}
	}
}

// Stop gracefully shuts down the worker pool
func (p *poolImpl[T]) Stop() error {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return errors.New("pool is not running")
	}
	p.isRunning = false
	p.mu.Unlock()

	// Signal shutdown
	p.cancel()

	// Wait for all workers to finish
	p.wg.Wait()

	// Close task channel
	close(p.tasks)

	return nil
}

// Submit adds a task to the pool and returns a Future that will hold the result
func (p *poolImpl[T]) Submit(t task.Task[T]) (future.Future[T], error) {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return nil, errors.New("pool is not running")
	}
	p.mu.Unlock()

	// Create a future for the task result with cancellation support
	taskCtx, cancel := context.WithCancel(p.ctx)
	f := future.NewFuture[T](cancel)
	t = t.WithContext(taskCtx)

	// Create a wrapped task that associates the task with its future
	wrapped := &taskWrapper[T]{
		task:       t,
		future:     f,
		submitTime: time.Now(),
	}

	// Submit the task to the worker pool
	select {
	case p.tasks <- wrapped:
		// Task was queued successfully
	case <-p.ctx.Done():
		// Pool was stopped before task could be queued
		return nil, errors.New("pool stopped")
	}

	return f, nil
}

// SubmitBatch adds multiple tasks to the pool
func (p *poolImpl[T]) SubmitBatch(tasks []future.Future[T]) ([]future.Future[T], error) {
	p.mu.Lock()
	if !p.isRunning {
		p.mu.Unlock()
		return nil, errors.New("pool is not running")
	}
	p.mu.Unlock()

	results := make([]future.Future[T], 0, len(tasks))
	for _, f := range tasks {
		if t, ok := f.(task.Task[T]); ok {
			future, err := p.Submit(t)
			if err != nil {
				return results, err
			}
			results = append(results, future)
		}
	}

	return results, nil
}

// Resize changes the number of workers in the pool
func (p *poolImpl[T]) Resize(size uint) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return errors.New("pool is not running")
	}

	if size < p.options.Workers {
		return errors.New("cannot reduce workers below current count")
	}

	// Start additional workers
	for i := p.options.Workers; i < size; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	p.options.Workers = size

	return nil
}

// Stats returns current statistics about the pool
func (p *poolImpl[T]) Stats() PoolStats {
	completed := p.completedTasks.Load()
	var avgProcessingTime time.Duration
	if completed > 0 {
		avgProcessingTime = time.Duration(p.totalProcessingTime.Load() / int64(completed))
	}

	var avgWaitTime time.Duration
	if completed > 0 {
		avgWaitTime = time.Duration(p.totalWaitTime.Load() / int64(completed))
	}

	var avgIdleTime time.Duration
	if p.options.Workers > 0 {
		avgIdleTime = time.Duration(p.totalIdleTime.Load() / int64(p.options.Workers))
	}

	return PoolStats{
		ActiveWorkers:   p.activeWorkers.Load(),
		QueuedTasks:     uint64(len(p.tasks)),
		CompletedTasks:  completed,
		FailedTasks:     p.failedTasks.Load(),
		ProcessingTime:  avgProcessingTime,
		AverageWaitTime: avgWaitTime,
		IdleTime:        avgIdleTime,
	}
}
