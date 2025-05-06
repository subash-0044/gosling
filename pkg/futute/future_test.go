package task

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFutureGet(t *testing.T) {
	t.Run("returns result when already complete", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		f := NewFuture[int](cancel)
		f.Complete(42, nil)

		result, err := f.Get(ctx)

		assert.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("blocks until complete", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[string](cancel)

		go func() {
			time.Sleep(100 * time.Millisecond)
			f.Complete("result", nil)
		}()

		result, err := f.Get(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "result", result)
	})

	t.Run("returns error when complete with error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)
		expectedErr := errors.New("computation failed")
		f.Complete(0, expectedErr)

		result, err := f.Get(ctx)

		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 0, result)
	})

	t.Run("returns context error when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		f := NewFuture[int](func() {})

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		result, err := f.Get(ctx)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Zero(t, result)
	})
}

func TestFutureGetWithTimeout(t *testing.T) {
	t.Run("returns result when complete within timeout", func(t *testing.T) {
		ctx := context.Background()
		f := NewFuture[int](func() {})

		go func() {
			time.Sleep(50 * time.Millisecond)
			f.Complete(42, nil)
		}()

		result, err := f.GetWithTimeout(ctx, 200*time.Millisecond)

		assert.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("returns timeout error when not complete within timeout", func(t *testing.T) {
		ctx := context.Background()
		f := NewFuture[int](func() {})

		go func() {
			time.Sleep(200 * time.Millisecond)
			f.Complete(42, nil)
		}()

		result, err := f.GetWithTimeout(ctx, 50*time.Millisecond)

		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Zero(t, result)
	})
}

func TestFutureIsDone(t *testing.T) {
	t.Run("returns false when not complete", func(t *testing.T) {
		_, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)

		assert.False(t, f.IsDone())
	})

	t.Run("returns true when complete", func(t *testing.T) {
		_, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)
		f.Complete(42, nil)

		assert.True(t, f.IsDone())
	})
}

func TestFutureCancel(t *testing.T) {
	t.Run("calls cancel function", func(t *testing.T) {
		cancelCalled := false
		cancelFunc := func() {
			cancelCalled = true
		}

		f := NewFuture[int](cancelFunc)

		f.Cancel()

		assert.True(t, cancelCalled)
	})

	t.Run("handles nil cancel function", func(t *testing.T) {
		f := NewFuture[int](nil)

		assert.NotPanics(t, func() {
			f.Cancel()
		})
	})
}

func TestFutureComplete(t *testing.T) {
	t.Run("completes future with result", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)

		f.Complete(42, nil)

		assert.True(t, f.IsDone())
		result, err := f.Get(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("completes future with error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)
		expectedErr := errors.New("computation failed")

		f.Complete(0, expectedErr)

		assert.True(t, f.IsDone())
		result, err := f.Get(ctx)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 0, result)
	})

	t.Run("only first completion takes effect", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)

		f.Complete(42, nil)
		f.Complete(99, errors.New("second completion"))

		result, err := f.Get(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 42, result)
	})
}

func TestFutureConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent access", func(t *testing.T) {
		const goroutines = 10
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[int](cancel)

		var wg1 sync.WaitGroup
		wg1.Add(goroutines)
		
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg1.Done()
				for j := 0; j < 100; j++ {
					f.IsDone()
					// Trying to simulate race condition
					time.Sleep(time.Millisecond)
				}
			}()
		}
		
		time.Sleep(50 * time.Millisecond)
		f.Complete(42, nil)
		
		var wg2 sync.WaitGroup
		wg2.Add(goroutines)
		results := make([]int, goroutines)
		errors := make([]error, goroutines)
		
		for i := 0; i < goroutines; i++ {
			i := i
			go func() {
				defer wg2.Done()
				results[i], errors[i] = f.Get(ctx)
			}()
		}
		
		wg1.Wait()
		wg2.Wait()
		
		for i := 0; i < goroutines; i++ {
			assert.NoError(t, errors[i])
			assert.Equal(t, 42, results[i])
		}
	})
}

func TestFutureWithDifferentTypes(t *testing.T) {
	t.Run("works with string type", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[string](cancel)
		f.Complete("hello", nil)

		result, err := f.Get(ctx)

		assert.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("works with struct type", func(t *testing.T) {
		type TestStruct struct {
			Name string
			Age  int
		}
		
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[TestStruct](cancel)
		expected := TestStruct{Name: "Alice", Age: 30}
		f.Complete(expected, nil)

		result, err := f.Get(ctx)

		assert.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("works with pointer type", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := NewFuture[*int](cancel)
		val := 42
		f.Complete(&val, nil)

		result, err := f.Get(ctx)

		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 42, *result)
	})
}