// Package lifecycle coordinates the ordered shutdown of a process's resources
// — HTTP servers, database pools, queues — when a termination signal arrives
// or Shutdown is called.
//
// Components register shutdown hooks as they are wired up; on shutdown the
// hooks run one at a time in **reverse registration order** (LIFO, the defer
// idiom), so a resource is always closed before the resources it depends on:
//
//	lifecycle.Register(db.Close)                 // registered first, closed last
//	lifecycle.Register(func(ctx context.Context) error {
//		return server.Shutdown(ctx)          // registered last, closed first
//	})
//	go func() {
//		// ErrServerClosed is what Shutdown makes ListenAndServe return, so
//		// it is the clean stop; anything else must not leave the process up
//		// and silent, so it takes the same shutdown path a signal would.
//		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
//			slog.Error("listener stopped", slog.Any("error", err))
//			lifecycle.Trigger()
//		}
//	}()
//	lifecycle.WaitForSignals(10*time.Second, os.Interrupt, syscall.SIGTERM)
//
// Shutdown runs every hook exactly once — a failing hook does not stop the
// ones after it (each error is collected and the combined error returned) —
// and later Shutdown calls (or concurrent ones) wait for the first to finish
// and return its result. The package owns no goroutines: WaitForSignals is a
// blocking select over the signal channel and the Trigger channel, not a
// watcher goroutine.
//
// WaitForSignals' first argument bounds the whole shutdown sequence: it becomes
// the deadline on the context every hook receives, measured from the moment the
// signal arrives. Pass 0 to impose no deadline and leave the bound to the
// platform's kill escalation, which is the right choice when a grace period is
// already configured there (ADR-0051, superseding ADR-0025 on this point).
//
// Code that must start the shutdown itself — a fatal background error, an
// admin endpoint, a supervisor command — calls Trigger, which unblocks a
// pending WaitForSignals as a signal would.
package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// coordinator is the shutdown state machine behind the package-level API. It
// exists as a type (rather than bare package vars) so tests can run against a
// fresh instance.
type coordinator struct {
	mu          sync.Mutex
	hooks       []func(context.Context) error
	started     bool          // a Shutdown has begun; hooks are frozen
	finished    chan struct{} // closed when the first Shutdown completes
	result      error         // written before finished is closed
	triggerOnce sync.Once     // guards the close of triggered
	triggered   chan struct{} // closed by the first Trigger
}

func newCoordinator() *coordinator {
	return &coordinator{finished: make(chan struct{}), triggered: make(chan struct{})}
}

// std is the process-wide coordinator the exported functions delegate to —
// the same package-level-default shape as log/slog's Default. The spec froze
// package-level functions, so the coordinator is a documented singleton.
var std = newCoordinator()

// Register adds a shutdown hook to run when the process shuts down. Hooks run
// in reverse registration order, so register a resource before the resources
// built on top of it. Register panics on a nil fn, and once a Shutdown has
// begun — a hook registered that late would silently never run, which is a
// wiring error worth failing loudly (ADR-0005).
func Register(fn func(ctx context.Context) error) { std.register(fn) }

// Shutdown runs the registered hooks in reverse registration order, passing
// each of them ctx, and returns the combined error (errors.Join) of every hook
// that failed, or nil. A failing hook never prevents the remaining hooks from
// running. Hooks run exactly once per process: the first Shutdown call runs
// them, and any later or concurrent call waits for that run to finish and
// returns its result. Cancelling ctx does not skip hooks — each hook receives
// the cancelled context and decides for itself how to abort.
func Shutdown(ctx context.Context) error { return std.shutdown(ctx) }

// Trigger requests shutdown programmatically, unblocking a pending
// WaitForSignals exactly as a termination signal would — for code that decides
// to stop the process itself: a fatal background error, an admin endpoint, a
// supervisor command.
//
// Trigger is idempotent and safe for concurrent use: the first call arms the
// request and every later call is a no-op. It never blocks and never runs the
// hooks itself — WaitForSignals runs them when it wakes. Triggering before
// WaitForSignals is called is therefore not a lost wakeup: the request latches,
// and WaitForSignals returns as soon as it is entered. A process that never
// calls WaitForSignals sees no effect from Trigger; it should call Shutdown
// instead.
func Trigger() { std.trigger() }

// notifySignal and stopSignal indirect os/signal so tests can inject a fake
// signal source instead of delivering real process signals (impossible to do
// portably — Windows has no kill(2)).
var (
	notifySignal = signal.Notify
	stopSignal   = signal.Stop
)

// WaitForSignals blocks until one of the given signals is delivered or Trigger
// is called, then runs Shutdown and returns. Any shutdown error is logged at
// Error level on slog.Default before returning. Called with no signals it waits
// for os.Interrupt and syscall.SIGTERM — the common termination pair (on Windows
// only Interrupt/Ctrl+C is ever delivered; SIGTERM is accepted but never fires).
// A Trigger that arrived before this call returns immediately.
//
// timeout bounds the whole shutdown sequence, not each hook: it becomes the
// deadline on the context every hook receives, so hooks that honour their
// context wind up early rather than being abandoned, and the remaining hooks
// still run (a hook decides for itself what an expired context means —
// ADR-0025). Per-hook budgets are deliberately not offered.
//
//	lifecycle.WaitForSignals(10*time.Second, os.Interrupt, syscall.SIGTERM)
//
// A timeout of 0 imposes no deadline: hooks receive a background context and the
// platform's own kill escalation (systemd's TimeoutStopSec, Kubernetes' grace
// period, then SIGKILL) is the only bound. That is the right choice when the
// operator has already configured a grace period one level up, and duplicating
// it here would give two numbers free to drift apart. A negative timeout panics
// (ADR-0005: a wiring error, caught at the call).
func WaitForSignals(timeout time.Duration, sigs ...os.Signal) {
	if timeout < 0 {
		panic("lifecycle: negative shutdown timeout")
	}
	if len(sigs) == 0 {
		sigs = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}
	ch := make(chan os.Signal, 1)
	notifySignal(ch, sigs...)
	defer stopSignal(ch)
	select {
	case <-ch:
	case <-std.triggered:
	}

	ctx := context.Background()
	if timeout > 0 {
		// Derived only after a signal arrives: a deadline started at call time
		// would spend the process's entire uptime before shutdown even begins.
		timed, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = timed
	}

	if err := Shutdown(ctx); err != nil {
		slog.Default().LogAttrs(context.Background(), slog.LevelError,
			"lifecycle: shutdown error", slog.Any("error", err))
	}
}

func (c *coordinator) register(fn func(ctx context.Context) error) {
	if fn == nil {
		panic("lifecycle: nil hook")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		panic("lifecycle: Register after Shutdown")
	}
	c.hooks = append(c.hooks, fn)
}

// trigger latches the shutdown request by closing triggered. sync.Once makes
// the close idempotent under concurrency — closing an already-closed channel
// panics — and needs no coordination with mu, which guards the hook slice and
// the Shutdown state machine, neither of which trigger touches.
func (c *coordinator) trigger() {
	c.triggerOnce.Do(func() { close(c.triggered) })
}

func (c *coordinator) shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		// Another call ran (or is running) the hooks: wait for it and return
		// its result. result is written before finished is closed, so the
		// channel receive orders the read correctly.
		c.mu.Unlock()
		<-c.finished
		return c.result
	}
	c.started = true
	hooks := c.hooks
	c.mu.Unlock()

	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i](ctx); err != nil {
			errs = append(errs, err)
		}
	}
	c.result = errors.Join(errs...)
	close(c.finished)
	return c.result
}
