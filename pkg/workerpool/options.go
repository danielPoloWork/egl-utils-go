package workerpool

// Option customises a Pool at construction time (functional options;
// ADR-0005).
type Option func(*Pool)

// WithNonBlockingSubmit makes Submit fail fast with ErrQueueFull when the
// queue is at capacity, instead of blocking until space frees.
func WithNonBlockingSubmit() Option {
	return func(p *Pool) { p.nonBlock = true }
}

// WithPanicHandler installs h as the pool's panic policy: a panicking task is
// recovered and h receives the recovered value, keeping the worker alive.
// Without a handler, a task panic propagates and crashes the process
// (standard Go semantics) — containment is a deliberate opt-in, never a
// silent default. A nil h panics.
func WithPanicHandler(h func(recovered any)) Option {
	if h == nil {
		panic("workerpool: nil panic handler")
	}
	return func(p *Pool) { p.onPanic = h }
}
