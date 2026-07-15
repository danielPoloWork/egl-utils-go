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
//	go func() { _ = server.ListenAndServe() }()
//	lifecycle.WaitForSignals(os.Interrupt, syscall.SIGTERM)
//
// Shutdown runs every hook exactly once — a failing hook does not stop the
// ones after it (each error is collected and the combined error returned) —
// and later Shutdown calls (or concurrent ones) wait for the first to finish
// and return its result. The package owns no goroutines: WaitForSignals is a
// blocking receive on the signal channel, not a watcher goroutine.
package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// coordinator is the shutdown state machine behind the package-level API. It
// exists as a type (rather than bare package vars) so tests can run against a
// fresh instance.
type coordinator struct {
	mu       sync.Mutex
	hooks    []func(context.Context) error
	started  bool          // a Shutdown has begun; hooks are frozen
	finished chan struct{} // closed when the first Shutdown completes
	result   error         // written before finished is closed
}

func newCoordinator() *coordinator {
	return &coordinator{finished: make(chan struct{})}
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

// notifySignal and stopSignal indirect os/signal so tests can inject a fake
// signal source instead of delivering real process signals (impossible to do
// portably — Windows has no kill(2)).
var (
	notifySignal = signal.Notify
	stopSignal   = signal.Stop
)

// WaitForSignals blocks until one of the given signals is delivered, then runs
// Shutdown with a background context and returns. Any shutdown error is logged
// at Error level on slog.Default before returning. Called with no signals it
// waits for os.Interrupt and syscall.SIGTERM — the common termination pair
// (on Windows only Interrupt/Ctrl+C is ever delivered; SIGTERM is accepted but
// never fires).
//
// No timeout is imposed on the hooks: the platform's own kill escalation
// (systemd/Kubernetes SIGKILL after their grace period) is the ultimate bound.
// A consumer that wants its own bound calls Shutdown directly with a deadline
// context (e.g. via signal.NotifyContext) instead of WaitForSignals.
func WaitForSignals(sigs ...os.Signal) {
	if len(sigs) == 0 {
		sigs = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}
	ch := make(chan os.Signal, 1)
	notifySignal(ch, sigs...)
	defer stopSignal(ch)
	<-ch
	if err := Shutdown(context.Background()); err != nil {
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
