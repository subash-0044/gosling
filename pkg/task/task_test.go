package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	t.Run("should execute task without error ", func(t *testing.T) {
		task := NewTask(func() (int, error) { return 21, nil })
		result, err := task.Execute()
		assert.NoError(t, err)
		assert.Equal(t, 21, result)
	})

	t.Run("should return error when task fails", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, assert.AnError
		})

		result, err := task.Execute()
		assert.Error(t, err)
		assert.Equal(t, 0, result)
	})
}

func TestWithContext(t *testing.T) {
	t.Run("should set context for task", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, nil
		})
		ctx := context.Background()

		task.WithContext(ctx)

		assert.Equal(t, ctx, task.Context())
	})
}

func TestIsCancelled(t *testing.T) {
	t.Run("should check if task is cancelled", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, nil
		})
		ctx, cancel := context.WithCancel(context.Background())
		task.WithContext(ctx)

		cancel()

		assert.True(t, task.IsCancelled())
	})
	t.Run("should return false if task is not cancelled", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, nil
		})
		ctx := context.Background()
		task.WithContext(ctx)

		assert.False(t, task.IsCancelled())
	})
}

func TestDeadline(t *testing.T) {
	t.Run("should return deadline for task", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, nil
		})
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Hour))
		defer cancel()
		task.WithContext(ctx)

		deadline, ok := task.Deadline()
		assert.True(t, ok)
		assert.Equal(t, time.Now().Add(1*time.Hour).Truncate(time.Second), deadline.Truncate(time.Second))

	})

	t.Run("should return false if no deadline is set", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, nil
		})
		ctx := context.Background()
		task.WithContext(ctx)

		deadline, ok := task.Deadline()
		assert.False(t, ok)
		assert.Equal(t, time.Time{}, deadline)
	})
}

func TestPriority(t *testing.T) {
	t.Run("should return priority for task", func(t *testing.T) {
		task := NewTask(func() (int, error) {
			return 0, nil
		})
		task.SetPriority(1)

		assert.Equal(t, 1, task.Priority())
	})
}
