package semaphore_test

import (
	"context"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/internal/leakcheck"
	"github.com/danielPoloWork/egl-utils-go/semaphore"
)

func TestNewWeightedPanicsOnNonPositiveCapacity(t *testing.T) {
	leakcheck.Guard(t)
	for _, capacity := range []int64{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewWeighted(%d) did not panic", capacity)
				}
			}()
			semaphore.NewWeighted(capacity)
		}()
	}
}

func TestAcquireReleaseBoundsConcurrency(t *testing.T) {
	leakcheck.Guard(t)
	sem := semaphore.NewWeighted(2)
	ctx := context.Background()

	if err := sem.Acquire(ctx, 2); err != nil {
		t.Fatalf("Acquire(2) on a fresh capacity-2 semaphore: %v", err)
	}

	// Capacity is exhausted; a further Acquire must block until Release.
	admitted := make(chan struct{})
	go func() {
		if err := sem.Acquire(ctx, 1); err == nil {
			close(admitted)
		}
	}()
	select {
	case <-admitted:
		t.Fatal("Acquire admitted weight past capacity")
	case <-time.After(50 * time.Millisecond):
	}

	sem.Release(2)
	select {
	case <-admitted:
	case <-time.After(2 * time.Second):
		t.Fatal("Release did not unblock a waiting Acquire")
	}
	sem.Release(1) // leave the semaphore balanced so the leak guard sees no waiters
}

func TestAcquireHonorsContextCancellation(t *testing.T) {
	leakcheck.Guard(t)
	sem := semaphore.NewWeighted(1)
	if err := sem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("initial Acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- sem.Acquire(ctx, 1) }()

	time.Sleep(20 * time.Millisecond) // let the second Acquire block
	cancel()

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("Acquire returned nil after its context was canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire did not return after cancellation")
	}
	sem.Release(1)
}

func TestAcquirePanicsOnNonPositiveWeight(t *testing.T) {
	leakcheck.Guard(t)
	sem := semaphore.NewWeighted(1)
	defer func() {
		if recover() == nil {
			t.Fatal("Acquire with a non-positive weight did not panic")
		}
	}()
	_ = sem.Acquire(context.Background(), 0)
}

func TestReleasePanicsOnNonPositiveWeight(t *testing.T) {
	leakcheck.Guard(t)
	sem := semaphore.NewWeighted(1)
	defer func() {
		if recover() == nil {
			t.Fatal("Release with a non-positive weight did not panic")
		}
	}()
	sem.Release(0)
}
