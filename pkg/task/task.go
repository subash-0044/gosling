package task

import (
	"context"
	"time"
)

// Task represents a unit of work to be processed by the worker pool.
type Task[T any] interface {
	// Execute performs the task and returns an error if it fails.
	Execute() (T, error)

	// WithContext sets the context for the task.
	WithContext(ctx context.Context) Task[T]

	// Context returns the context for the task.
	Context() context.Context

	// IsCancelled checks if the task has been canceled.
	IsCancelled() bool

	// Deadline returns the deadline for the task.
	Deadline() (time.Time, bool)

	// Priority returns the priority of the task.
	Priority() int

	// SetPriority sets the priority of the task.
	SetPriority(priority int) Task[T]
}

// DefaultTask is a default implementation of the Task interface.
type defaultTaskItem[T any] struct {
	fn       func() (T, error)
	ctx      context.Context
	priority int
}

// NewTask creates a new task with the given function and context.
func NewTask[T any](fn func() (T, error)) *defaultTaskItem[T] {
	return &defaultTaskItem[T]{
		fn:  fn,
		ctx: context.Background(),
		priority: 0,
	}
}

// Execute performs the task and returns an error if it fails.
func (t *defaultTaskItem[T]) Execute() (T, error) {
	return t.fn()
}

// WithContext sets the context for the task.
func (t *defaultTaskItem[T]) WithContext(ctx context.Context) Task[T] {
	t.ctx = ctx
	return t
}

// Context returns the context for the task.
func (t *defaultTaskItem[T]) Context() context.Context {
	return t.ctx
}

// IsCancelled checks if the task has been canceled.
func (t *defaultTaskItem[T]) IsCancelled() bool {
	select {
	case <-t.ctx.Done():
		return true
	default:
		return false
	}
}

// Deadline returns the deadline for the task.
func (t *defaultTaskItem[T]) Deadline() (time.Time, bool) {
	return t.ctx.Deadline()
}

// Priority returns the priority of the task.
func (t *defaultTaskItem[T]) Priority() int {
	return t.priority
}

// SetPriority sets the priority of the task.
func (t *defaultTaskItem[T]) SetPriority(priority int) Task[T] {
	t.priority = priority
	return t
}
