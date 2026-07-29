// Package workerpool provides a bounded, context-aware goroutine pool: a
// fixed set of worker goroutines consumes tasks from a bounded queue, giving
// callers backpressure — block until space frees (default) or fail fast with
// ErrQueueFull (WithNonBlockingSubmit) — instead of unbounded goroutine
// growth.
//
// Lifecycle: New starts the workers immediately; Submit admits tasks until
// Close is called; Close closes admission, drains every queued task, and joins
// the workers, honoring its context as a deadline. All types are safe for
// concurrent use. Design decisions are recorded in ADR-0005 and ADR-0048.
package workerpool

import (
	"context"
	"errors"
	"sync"
)

// Task is a unit of work executed by a worker goroutine. The context passed
// to the task is the pool's execution context, not the Submit context: it is
// canceled only when Close's deadline expires (hard stop), signalling in-flight
// tasks to abandon work. A task that ignores its context can delay or prevent
// full shutdown — that is the task's bug, not the pool's.
type Task func(ctx context.Context)

// Sentinel errors returned by Submit.
var (
	// ErrQueueFull is returned by Submit when the pool was built with
	// WithNonBlockingSubmit and the task queue is at capacity.
	ErrQueueFull = errors.New("workerpool: task queue is full")

	// ErrClosed is returned by Submit after Close has been called.
	ErrClosed = errors.New("workerpool: pool is closed")
)

// Pool is a fixed-size worker pool with a bounded task queue. The zero value
// is not usable; construct a Pool with New.
type Pool struct {
	queue    chan Task
	nonBlock bool
	onPanic  func(recovered any)

	execCtx  context.Context // passed to every task; canceled on hard stop
	hardStop context.CancelFunc

	// mu serialises Submit bodies against Close's close(queue): Close flips
	// closed under the write lock, so once it holds the lock no Submit can be
	// mid-send and closing the channel is provably race-free.
	mu     sync.RWMutex
	closed bool

	workers sync.WaitGroup
}

// New builds a Pool with exactly workers goroutines and a task queue holding
// up to queueSize pending tasks (queueSize 0 gives direct hand-off). It
// panics if workers <= 0 or queueSize < 0 — invalid sizes are programming
// errors, not runtime conditions. Workers start immediately and idle until
// tasks arrive; release them with Close.
func New(workers, queueSize int, opts ...Option) *Pool {
	if workers <= 0 {
		panic("workerpool: workers must be > 0")
	}
	if queueSize < 0 {
		panic("workerpool: queueSize must be >= 0")
	}
	execCtx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		queue:    make(chan Task, queueSize),
		execCtx:  execCtx,
		hardStop: cancel,
	}
	for _, opt := range opts {
		opt(p)
	}
	p.workers.Add(workers)
	for range workers {
		go p.worker()
	}
	return p
}

// Submit hands task to the pool. Under the default blocking policy it waits
// until queue space frees or ctx is done; under WithNonBlockingSubmit it
// returns ErrQueueFull instead of waiting. After Close it returns ErrClosed.
// Submit only ever blocks on queue admission, never on task execution. A nil
// task panics.
//
// A Submit already blocked on a full queue when Close is invoked completes its
// admission and the task is drained normally; a Submit arriving after Close is
// rejected.
func (p *Pool) Submit(ctx context.Context, task Task) error {
	if task == nil {
		panic("workerpool: nil Task")
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	if p.nonBlock {
		select {
		case p.queue <- task:
			return nil
		default:
			return ErrQueueFull
		}
	}
	select {
	case p.queue <- task:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close closes the pool: no new tasks are admitted, already-queued tasks are
// drained, and Close waits for every worker to finish. If ctx expires first,
// the pool's execution context is canceled so in-flight tasks observe the
// hard stop, and Close returns ctx.Err() without waiting further — workers
// still exit once their current task returns. Close is idempotent and safe for
// concurrent use; every caller waits for the drain.
//
// Close takes a context deliberately, unlike cache.Close and pubsub.Close: it
// is the only shutdown in the module that waits for caller-supplied work to
// finish, so the caller — not the pool — bounds that wait (ADR-0025, ADR-0048).
func (p *Pool) Close(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.queue)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
		p.hardStop() // release the execution context's resources
		return nil
	case <-ctx.Done():
		p.hardStop()
		return ctx.Err()
	}
}

// worker consumes the queue until it is closed and drained.
func (p *Pool) worker() {
	defer p.workers.Done()
	for task := range p.queue {
		p.run(task)
	}
}

// run executes one task under the pool's panic policy: with a handler
// installed the panic is recovered and handed to it, keeping the worker
// alive; without one the panic propagates untouched (standard Go semantics —
// an unobserved bug stays loud).
func (p *Pool) run(task Task) {
	if p.onPanic != nil {
		defer func() {
			if r := recover(); r != nil {
				p.onPanic(r)
			}
		}()
	}
	task(p.execCtx)
}
