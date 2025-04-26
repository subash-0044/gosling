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
type DefaultTaskItem[T any] struct {
	fn       func() (T, error)
	ctx      context.Context
	priority int
}

// Execute performs the task and returns an error if it fails.
func (t *DefaultTaskItem[T]) Execute() (T, error) {
	return t.fn()
}

// WithContext sets the context for the task.
func (t *DefaultTaskItem[T]) WithContext(ctx context.Context) Task[T] {
	t.ctx = ctx
	return t
}

// Context returns the context for the task.
func (t *DefaultTaskItem[T]) Context() context.Context {
	return t.ctx
}

// IsCancelled checks if the task has been canceled.
func (t *DefaultTaskItem[T]) IsCancelled() bool {
	select {
	case <-t.ctx.Done():
		return true
	default:
		return false
	}
}

// Deadline returns the deadline for the task.
func (t *DefaultTaskItem[T]) Deadline() (time.Time, bool) {
	return t.ctx.Deadline()
}

// Priority returns the priority of the task.
func (t *DefaultTaskItem[T]) Priority() int {
	return t.priority
}

// SetPriority sets the priority of the task.
func (t *DefaultTaskItem[T]) SetPriority(priority int) Task[T] {
	t.priority = priority
	return t
}
