package future

import (
	"context"
	"sync"
	"time"
)

// Future represents a result of an asynchronous computation.
// It allows you to check if the computation is complete and retrieve the result.
type Future[T any] interface {
	// Get returns the result of the computation.
	// It blocks until the result is available or an error occurs.
	Get(ctx context.Context) (T, error)

	// GetWithTimeout returns the result of the computation with a timeout.
	GetWithTimeout(ctx context.Context, timeout time.Duration) (T, error)

	// IsDone checks if the computation is complete.
	IsDone() bool

	// Cancel attempts to cancel the computation.
	Cancel()
}

// DefaultFuture is a default implementation of the Future interface.
type defaultFuture[T any] struct {
	result     T
	err        error
	done       bool
	doneCh     chan struct{}
	cancelFunc context.CancelFunc
	mu         sync.RWMutex
}

// NewFuture creates a new future with the given result and error.
func NewFuture[T any](cancelFunc context.CancelFunc) *defaultFuture[T] {
	return &defaultFuture[T]{
		doneCh:     make(chan struct{}),
		cancelFunc: cancelFunc,
	}
}

// Get returns the result of the computation.
// It blocks until the result is available or an error occurs.
// If the computation is canceled, it returns an error.
// If the context is canceled, it returns the context's error.
// If the computation is already done, it returns the result immediately.
// If the computation is not done, it waits for the result.
func (f *defaultFuture[T]) Get(ctx context.Context) (T, error) {
	var zeroValue T

	// Check if the computation is already done
	f.mu.RLock()
	if f.done {
		result, err := f.result, f.err
		f.mu.RUnlock()
		return result, err
	}
	f.mu.RUnlock()

	select {
	case <-f.doneCh:
		f.mu.RLock()
		result, err := f.result, f.err
		f.mu.RUnlock()
		return result, err
	case <-ctx.Done():
		return zeroValue, ctx.Err()
	}
}

// GetWithTimeout returns the result of the computation with a timeout.
func (f *defaultFuture[T]) GetWithTimeout(ctx context.Context, timeout time.Duration) (T, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return f.Get(ctxWithTimeout)
}

// Complete sets the result of the computation and marks it as done.
// It closes the done channel to notify any waiting goroutines.
// If the computation is already done, it does nothing.
// This method is not part of the public Future interface.
// It is intended for internal use only.
func (f *defaultFuture[T]) Complete(result T, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.done {
		f.result = result
		f.err = err
		f.done = true
		close(f.doneCh)
	}
}

// IsDone checks if the computation is complete.
func (f *defaultFuture[T]) IsDone() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.done
}

// Cancel attempts to cancel the computation.
func (f *defaultFuture[T]) Cancel() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancelFunc != nil {
		f.cancelFunc()
	}
}

// GetFuture is used to get the futureImpl for internal use
// Not part of the public Future interface
func (f *defaultFuture[T]) GetFutureImpl() *defaultFuture[T] {
	return f
}
