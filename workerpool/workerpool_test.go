package workerpool_test

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/workerpool"
	"go.uber.org/goleak"
)

func TestNewPanicsOnInvalidArguments(t *testing.T) {
	cases := []struct {
		name              string
		workers, queueLen int
	}{
		{"zero workers", 0, 1},
		{"negative workers", -1, 1},
		{"negative queue", 1, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("New(%d, %d) did not panic", tc.workers, tc.queueLen)
				}
			}()
			workerpool.New(tc.workers, tc.queueLen)
		})
	}
}

func TestSubmitNilTaskPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(1, 0)
	defer func() { _ = p.Stop(context.Background()) }()
	defer func() {
		if recover() == nil {
			t.Fatal("Submit(nil) did not panic")
		}
	}()
	_ = p.Submit(context.Background(), nil)
}

func TestNilPanicHandlerPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("WithPanicHandler(nil) did not panic")
		}
	}()
	workerpool.WithPanicHandler(nil)
}

func TestSubmitRunsAllTasksAndStopDrains(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(4, 8)
	var ran atomic.Int64
	const tasks = 100
	for range tasks {
		err := p.Submit(context.Background(), func(context.Context) {
			time.Sleep(time.Millisecond) // force queue pressure
			ran.Add(1)
		})
		if err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := ran.Load(); got != tasks {
		t.Fatalf("ran %d of %d tasks — Stop must drain the queue", got, tasks)
	}
}

func TestBlockingSubmitHonorsContextWhenFull(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(1, 0)
	gate := make(chan struct{})
	started := make(chan struct{})
	err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-gate
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-started // the only worker is now busy; capacity-0 queue has no space

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = p.Submit(ctx, func(context.Context) {})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked Submit returned %v, want DeadlineExceeded", err)
	}

	close(gate)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestSubmitWithCanceledContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(1, 0)
	gate := make(chan struct{})
	started := make(chan struct{})
	if err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-gate
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-started // occupy the worker so only the ctx branch of the select is ready

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.Submit(ctx, func(context.Context) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit with canceled ctx returned %v, want Canceled", err)
	}

	close(gate)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestNonBlockingSubmitFailsFastWhenFull(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(1, 1, workerpool.WithNonBlockingSubmit())
	gate := make(chan struct{})
	started := make(chan struct{})
	var ran atomic.Int64

	// A: dequeued and running (holds the worker).
	if err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-gate
		ran.Add(1)
	}); err != nil {
		t.Fatalf("Submit A: %v", err)
	}
	<-started

	// B: fills the single queue slot.
	if err := p.Submit(context.Background(), func(context.Context) { ran.Add(1) }); err != nil {
		t.Fatalf("Submit B: %v", err)
	}

	// C: queue full — must fail fast.
	err := p.Submit(context.Background(), func(context.Context) { ran.Add(1) })
	if !errors.Is(err, workerpool.ErrQueueFull) {
		t.Fatalf("Submit C returned %v, want ErrQueueFull", err)
	}

	close(gate)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := ran.Load(); got != 2 {
		t.Fatalf("ran %d tasks, want 2 (A and B)", got)
	}
}

func TestSubmitAfterStopReturnsErrPoolClosed(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(1, 1)
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	err := p.Submit(context.Background(), func(context.Context) {})
	if !errors.Is(err, workerpool.ErrPoolClosed) {
		t.Fatalf("Submit after Stop returned %v, want ErrPoolClosed", err)
	}
}

func TestStopIsIdempotentAndConcurrent(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(2, 4)
	var ran atomic.Int64
	for range 8 {
		if err := p.Submit(context.Background(), func(context.Context) { ran.Add(1) }); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = p.Stop(context.Background())
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Stop %d: %v", i, err)
		}
	}
	if got := ran.Load(); got != 8 {
		t.Fatalf("ran %d tasks, want 8", got)
	}
}

func TestStopDeadlineCancelsExecutionContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := workerpool.New(1, 0)
	entered := make(chan struct{})
	exited := make(chan struct{})
	if err := p.Submit(context.Background(), func(ctx context.Context) {
		close(entered)
		<-ctx.Done() // wait for the pool's hard stop
		close(exited)
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := p.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop returned %v, want DeadlineExceeded", err)
	}
	select {
	case <-exited:
		// the task observed the execution-context cancellation and finished
	case <-time.After(2 * time.Second):
		t.Fatal("task never observed the hard-stop cancellation")
	}
}

func TestPanicHandlerKeepsWorkerAlive(t *testing.T) {
	defer goleak.VerifyNone(t)
	recovered := make(chan any, 1)
	p := workerpool.New(1, 1, workerpool.WithPanicHandler(func(r any) {
		recovered <- r
	}))
	var ran atomic.Int64

	if err := p.Submit(context.Background(), func(context.Context) { panic("boom") }); err != nil {
		t.Fatalf("Submit panicking task: %v", err)
	}
	select {
	case r := <-recovered:
		if r != "boom" {
			t.Fatalf("handler received %v, want \"boom\"", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("panic handler was never invoked")
	}

	// The same (sole) worker must still be alive to run this.
	if err := p.Submit(context.Background(), func(context.Context) { ran.Add(1) }); err != nil {
		t.Fatalf("Submit after panic: %v", err)
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := ran.Load(); got != 1 {
		t.Fatalf("worker did not survive the recovered panic (ran=%d)", got)
	}
}

func BenchmarkSubmit(b *testing.B) {
	p := workerpool.New(runtime.GOMAXPROCS(0), 1024)
	defer func() { _ = p.Stop(context.Background()) }()
	ctx := context.Background()
	task := workerpool.Task(func(context.Context) {})
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = p.Submit(ctx, task)
		}
	})
}
