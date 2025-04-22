package simplepool_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/michaellenaghan/go-simplepool"
)

func BenchmarkGetPut(b *testing.B) {
	b.Run("Sequential", func(b *testing.B) {
		pool, err := simplepool.New(
			simplepool.Config[int]{
				Count:   10,
				NewFunc: func() (int, error) { return 0, nil },
			},
		)
		if err != nil {
			b.Fatalf("Failed to create pool: %v\n", err)
		}
		defer pool.Stop()

		// Cancellable contexts can impact performance, so use one.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for b.Loop() {
			obj, err := pool.Get(ctx)
			if err != nil {
				b.Errorf("Failed to get object: %v\n", err)
				continue
			}
			pool.Put(obj)
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		pool, err := simplepool.New(
			simplepool.Config[int]{
				Count:   10,
				NewFunc: func() (int, error) { return 0, nil },
			},
		)
		if err != nil {
			b.Fatalf("Failed to create pool: %v\n", err)
		}
		defer pool.Stop()

		// Cancellable contexts can impact performance, so use one.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				obj, err := pool.Get(ctx)
				if err != nil {
					b.Errorf("Failed to get object: %v\n", err)
					continue
				}
				pool.Put(obj)
			}
		})
	})
}

func ExamplePool_concurrentGetAndPut() {
	simplepool, err := simplepool.New(
		simplepool.Config[int]{
			Count:       5,
			NewFunc:     func() (int, error) { return 0, nil },
			DestroyFunc: func(int) {}, // optional
		},
	)
	if err != nil {
		fmt.Printf("Failed to new pool: %v\n", err)
		return
	}
	defer simplepool.Stop()

	wg := sync.WaitGroup{}
	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			obj, err := simplepool.Get(context.Background())
			if err != nil {
				fmt.Printf("Failed to get object: %v\n", err)
				return
			}
			defer simplepool.Put(obj)

			time.Sleep(10 * time.Millisecond)
		}()
	}
	wg.Wait()
}

// This test verifies the pool's behavior when a context is cancelled while
// waiting for an object. It creates a scenario where:
//
//  1. The pool has only one object (min=max=1)
//  2. That object is obtained by a Get() call
//  3. A second Get() call is made with a cancelled context
//
// This ensures the pool correctly responds to context cancellation by
// returning the appropriate error rather than blocking indefinitely.
func TestPoolCancelContext(t *testing.T) {
	t.Parallel()

	p, err := simplepool.New(
		simplepool.Config[int]{
			Count:   1,
			NewFunc: func() (int, error) { return 0, nil },
		},
	)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer p.Stop()

	// Get the only available object
	obj, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}

	// Try to get another object with a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.Get(ctx)
	if err != context.Canceled {
		t.Fatalf("Expected context.Canceled error, got: %v", err)
	}

	p.Put(obj)
}

// This test verifies the pool's behavior under concurrent load.
// It launches multiple goroutines that simultaneously get objects,
// hold them briefly, and then return them to the simplepool.
// This tests thread safety and proper object management when
// multiple goroutines interact with the pool simultaneously.
func TestPoolConcurrentGetAndPut(t *testing.T) {
	t.Parallel()

	p, err := simplepool.New(
		simplepool.Config[int]{
			Count:       5,
			NewFunc:     func() (int, error) { return 0, nil },
			DestroyFunc: func(int) {},
		},
	)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer p.Stop()

	wg := sync.WaitGroup{}
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			obj, err := p.Get(context.Background())
			if err != nil {
				t.Errorf("Failed to get object: %v", err)
				return
			}
			defer p.Put(obj)

			time.Sleep(10 * time.Millisecond)
		}()
	}
	wg.Wait()
}

// This test verifies error handling when NewFunc fails.
func TestPoolNewFuncError(t *testing.T) {
	t.Parallel()

	_, err := simplepool.New(
		simplepool.Config[int]{
			Count: 5,
			NewFunc: func() (int, error) {
				return 0, fmt.Errorf("Failed to create object")
			},
		},
	)
	if err == nil {
		t.Fatal("Expected error, got: nil")
	}
	if !errors.Is(err, simplepool.ErrNew) {
		t.Fatalf("Expected ErrNew error, got: %v", err)
	}
}

// This test verifies the basic functionality of Get and Put operations
// in a sequential (non-concurrent) context. It ensures that a simple
// get-then-put operation works correctly.
func TestPoolSequentialGetAndPut(t *testing.T) {
	t.Parallel()

	p, err := simplepool.New(
		simplepool.Config[int]{
			Count:       5,
			NewFunc:     func() (int, error) { return 0, nil },
			DestroyFunc: func(int) {},
		},
	)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer p.Stop()

	obj, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
	p.Put(obj)
}

// This test verifies the pool's shutdown behavior when Stop is called
// while objects are still in use. It ensures:
//
//  1. Stop marks the pool as stopping, making Get() return an error
//  2. Objects returned after Stop is called are properly destroyed
//  3. Stop waits for all busy objects to be returned and destroyed
//
// This tests proper resource cleanup during shutdown.
func TestPoolStopWithBusyObjects(t *testing.T) {
	t.Parallel()

	destroyedIDs := make([]int, 0)

	p, err := simplepool.New(
		simplepool.Config[int]{
			Count: 5,
			NewFunc: func() (int, error) {
				return rand.Int(), nil
			},
			DestroyFunc: func(obj int) {
				destroyedIDs = append(destroyedIDs, obj)
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	// Get some objects and keep track of them
	busyObjs := make([]int, 0, 5)
	for range 5 {
		obj, err := p.Get(context.Background())
		if err != nil {
			t.Fatalf("Failed to get object: %v", err)
		}
		busyObjs = append(busyObjs, obj)
	}

	// Call Stop in a goroutine since it will block until all objects are returned
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Stop() // This will block until all objects are returned and destroyed
	}()

	// Try to get another object after Stop - should fail immediately
	// Give a small delay for Stop to mark the pool as stopping
	time.Sleep(100 * time.Millisecond)

	_, err = p.Get(context.Background())
	if err == nil {
		t.Fatal("Expected error, got: nil")
	}
	if !errors.Is(err, simplepool.ErrStoppingOrStopped) {
		t.Fatalf("Expected error ErrStoppingOrStopped, got: %v", err)
	}

	// Return busy objects, which should get destroyed
	for _, obj := range busyObjs {
		p.Put(obj)
	}

	// Wait for Stop to complete
	wg.Wait()

	// Verify all objects were destroyed
	destroyedCount := len(destroyedIDs)
	if destroyedCount != 5 {
		t.Errorf("Expected 5 destroyed objects after returning busy objects, got: %d", destroyedCount)
	}
}

// This test subjects the pool to high concurrent load to verify its
// stability and performance under stress. It:
//
//  1. Creates many concurrent goroutines (1000) that each try to get an object
//  2. Uses short timeouts to simulate real-world constraints
//  3. Allows some Get() calls to fail due to timeout
//
// This test helps identify potential deadlocks, race conditions, or resource
// leaks that might only appear under heavy load.
func TestPoolStressTest(t *testing.T) {
	t.Parallel()

	p, err := simplepool.New(
		simplepool.Config[int]{
			Count:   5,
			NewFunc: func() (int, error) { return 0, nil },
		},
	)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer p.Stop()

	wg := sync.WaitGroup{}
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			obj, err := p.Get(ctx)
			if err != nil {
				return // It's okay to fail sometimes due to timeout
			}
			defer p.Put(obj)

			time.Sleep(10 * time.Millisecond)
		}()
	}
	wg.Wait()
}
