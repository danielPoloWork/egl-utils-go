package workerpool_test

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"
)

// Submit work to a bounded pool, then close it to wait for every task.
func ExampleNew() {
	pool := workerpool.New(4, 8)

	// Four workers run these concurrently, so the channel is buffered to the
	// number of tasks — a task that blocked on an unread channel would hold a
	// worker — and the results are sorted before printing: the pool promises
	// that every task runs, not the order they finish in.
	squares := make(chan int, 5)
	for i := 1; i <= 5; i++ {
		if err := pool.Submit(context.Background(), func(context.Context) {
			squares <- i * i
		}); err != nil {
			fmt.Println("submit:", err)
			return
		}
	}

	// Close stops admission, drains what is queued and joins the workers, so
	// once it returns no task is still running.
	if err := pool.Close(context.Background()); err != nil {
		fmt.Println("close:", err)
		return
	}
	close(squares)

	got := make([]int, 0, 5)
	for v := range squares {
		got = append(got, v)
	}
	slices.Sort(got)
	fmt.Println(got)
	// Output: [1 4 9 16 25]
}

// WithNonBlockingSubmit turns a full queue into an immediate ErrQueueFull
// instead of blocking the caller until space frees.
func ExampleWithNonBlockingSubmit() {
	pool := workerpool.New(1, 1, workerpool.WithNonBlockingSubmit())

	started := make(chan struct{})
	release := make(chan struct{})

	// Occupy the only worker. Waiting for started is what makes the rest
	// deterministic without any timing assumption: it proves the worker has
	// already taken this task off the queue, so the queue is empty again.
	if err := pool.Submit(context.Background(), func(context.Context) {
		close(started)
		<-release
	}); err != nil {
		fmt.Println("submit:", err)
		return
	}
	<-started

	// Fills the single queue slot.
	if err := pool.Submit(context.Background(), func(context.Context) {}); err != nil {
		fmt.Println("submit:", err)
		return
	}

	// Worker busy, queue full: shed the work rather than park the caller.
	err := pool.Submit(context.Background(), func(context.Context) {})
	fmt.Println(errors.Is(err, workerpool.ErrQueueFull))

	close(release)
	if err := pool.Close(context.Background()); err != nil {
		fmt.Println("close:", err)
	}
	// Output: true
}
