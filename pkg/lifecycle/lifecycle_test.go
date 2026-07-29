package lifecycle

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// swapStd gives the test a fresh coordinator behind the package-level API,
// restoring the previous one afterwards. Tests using it must not run in
// parallel — std is package state.
func swapStd(t *testing.T) *coordinator {
	t.Helper()
	orig := std
	std = newCoordinator()
	t.Cleanup(func() { std = orig })
	return std
}

// swapSignals replaces the os/signal seam with a fake that records the
// subscribed signals and immediately delivers sig, so WaitForSignals proceeds
// synchronously — no real process signal, no os/signal goroutine, portable to
// Windows (which has no kill(2)).
func swapSignals(t *testing.T, sig os.Signal) (gotSigs *[]os.Signal, stopped *bool) {
	t.Helper()
	var sigs []os.Signal
	var stop bool
	origNotify, origStop := notifySignal, stopSignal
	notifySignal = func(c chan<- os.Signal, s ...os.Signal) {
		sigs = append(sigs, s...)
		c <- sig
	}
	stopSignal = func(chan<- os.Signal) { stop = true }
	t.Cleanup(func() { notifySignal, stopSignal = origNotify, origStop })
	return &sigs, &stop
}

// swapSilentSignals replaces the os/signal seam with a fake that subscribes but
// never delivers anything, so WaitForSignals can only be unblocked by Trigger.
func swapSilentSignals(t *testing.T) {
	t.Helper()
	origNotify, origStop := notifySignal, stopSignal
	notifySignal = func(chan<- os.Signal, ...os.Signal) {}
	stopSignal = func(chan<- os.Signal) {}
	t.Cleanup(func() { notifySignal, stopSignal = origNotify, origStop })
}

func TestShutdownRunsHooksInReverseOrder(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	var order []string
	for _, name := range []string{"db", "queue", "server"} {
		Register(func(context.Context) error {
			order = append(order, name)
			return nil
		})
	}
	require.NoError(t, Shutdown(context.Background()))
	require.Equal(t, []string{"server", "queue", "db"}, order,
		"hooks run LIFO: the last-registered (most derived) resource closes first")
}

func TestShutdownPassesContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "v")
	var got any
	Register(func(c context.Context) error {
		got = c.Value(key{})
		return nil
	})
	require.NoError(t, Shutdown(ctx))
	require.Equal(t, "v", got, "hooks receive the Shutdown context")
}

func TestShutdownCancelledContextStillRunsHooks(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := 0
	Register(func(c context.Context) error { ran++; return c.Err() })
	Register(func(c context.Context) error { ran++; return c.Err() })
	err := Shutdown(ctx)
	require.Equal(t, 2, ran, "a cancelled context does not skip hooks; each decides for itself")
	require.ErrorIs(t, err, context.Canceled)
}

func TestShutdownJoinsErrorsAndRunsAllHooks(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	errA := errors.New("a failed")
	errB := errors.New("b failed")
	ran := 0
	Register(func(context.Context) error { ran++; return errA }) // runs last
	Register(func(context.Context) error { ran++; return nil })
	Register(func(context.Context) error { ran++; return errB }) // runs first
	err := Shutdown(context.Background())
	require.Equal(t, 3, ran, "a failing hook never stops the remaining hooks")
	require.ErrorIs(t, err, errA)
	require.ErrorIs(t, err, errB)
}

func TestShutdownWithNoHooks(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	require.NoError(t, Shutdown(context.Background()))
}

func TestShutdownRunsHooksExactlyOnce(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	errBoom := errors.New("boom")
	var runs atomic.Int32
	Register(func(context.Context) error { runs.Add(1); return errBoom })

	first := Shutdown(context.Background())
	second := Shutdown(context.Background())
	require.ErrorIs(t, first, errBoom)
	require.Equal(t, first, second, "later calls return the first run's result")
	require.Equal(t, int32(1), runs.Load(), "hooks run exactly once")
}

func TestShutdownConcurrentCallersConverge(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	errBoom := errors.New("boom")
	var runs atomic.Int32
	Register(func(context.Context) error {
		runs.Add(1)
		time.Sleep(20 * time.Millisecond) // widen the window concurrent callers race into
		return errBoom
	})

	const callers = 8
	results := make([]error, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = Shutdown(context.Background())
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), runs.Load(), "one run, however many callers")
	for i, err := range results {
		require.ErrorIs(t, err, errBoom, "caller %d sees the completed run's result", i)
	}
}

func TestRegisterNilPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	require.PanicsWithValue(t, "lifecycle: nil hook", func() { Register(nil) })
}

func TestRegisterAfterShutdownPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	require.NoError(t, Shutdown(context.Background()))
	require.PanicsWithValue(t, "lifecycle: Register after Shutdown", func() {
		Register(func(context.Context) error { return nil })
	})
}

func TestWaitForSignalsRunsShutdownOnSignal(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	gotSigs, stopped := swapSignals(t, os.Interrupt)
	ran := false
	Register(func(context.Context) error { ran = true; return nil })

	WaitForSignals(0, os.Interrupt, syscall.SIGTERM)

	require.True(t, ran, "the signal triggers Shutdown")
	require.Equal(t, []os.Signal{os.Interrupt, syscall.SIGTERM}, *gotSigs,
		"the given signals are subscribed")
	require.True(t, *stopped, "signal delivery is stopped on the way out")
}

func TestWaitForSignalsDefaultsToInterruptAndTerm(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	gotSigs, _ := swapSignals(t, os.Interrupt)
	WaitForSignals(0)
	require.Equal(t, []os.Signal{os.Interrupt, syscall.SIGTERM}, *gotSigs,
		"no arguments defaults to the common termination pair")
}

func TestTriggerUnblocksPendingWaitForSignals(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSilentSignals(t)
	var ran atomic.Bool
	Register(func(context.Context) error { ran.Store(true); return nil })

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		WaitForSignals(0) // no signal will ever be delivered
	}()

	Trigger()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("Trigger did not unblock a pending WaitForSignals")
	}
	require.True(t, ran.Load(), "the trigger runs the hooks, exactly as a signal would")
}

func TestTriggerBeforeWaitForSignalsIsNotALostWakeup(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSilentSignals(t)
	ran := false
	Register(func(context.Context) error { ran = true; return nil })

	Trigger()
	WaitForSignals(0) // returns immediately: the request latched in the channel

	require.True(t, ran, "a Trigger that arrived first is latched, not dropped")
}

func TestTriggerIsIdempotentAndConcurrencySafe(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSilentSignals(t)

	const callers = 8
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() { defer wg.Done(); Trigger() }()
	}
	wg.Wait() // a second close of the channel would have panicked

	var runs atomic.Int32
	Register(func(context.Context) error { runs.Add(1); return nil })
	WaitForSignals(0)
	Trigger() // still a no-op after the shutdown has run
	require.Equal(t, int32(1), runs.Load(), "however many triggers, one shutdown")
}

func TestTriggerIsCoordinatorScoped(t *testing.T) {
	defer goleak.VerifyNone(t)
	first := swapStd(t)
	Trigger()
	require.NotNil(t, first)

	// A fresh coordinator starts un-triggered: the latch is per-coordinator
	// state, not a process-wide flag.
	second := swapStd(t)
	require.NotSame(t, first, second)
	select {
	case <-second.triggered:
		t.Fatal("a fresh coordinator must not inherit an earlier Trigger")
	default:
	}
}

// capturingHandler records slog records for assertion.
type capturingHandler struct {
	mu      *sync.Mutex
	records *[]slog.Record
}

func (h capturingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, r.Clone())
	return nil
}
func (h capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h capturingHandler) WithGroup(string) slog.Handler      { return h }

func TestWaitForSignalsLogsShutdownError(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSignals(t, os.Interrupt)

	records := &[]slog.Record{}
	orig := slog.Default()
	slog.SetDefault(slog.New(capturingHandler{mu: &sync.Mutex{}, records: records}))
	t.Cleanup(func() { slog.SetDefault(orig) })

	Register(func(context.Context) error { return errors.New("db close failed") })
	WaitForSignals(0)

	require.Len(t, *records, 1, "a failing shutdown is logged before WaitForSignals returns")
	require.Equal(t, "lifecycle: shutdown error", (*records)[0].Message)
	require.Equal(t, slog.LevelError, (*records)[0].Level)
}

// --- the shutdown deadline (13.7, ADR-0051) --------------------------------

// TestWaitForSignalsGivesHooksTheDeadline is the substance of the signature
// change: the timeout is not merely accepted, it reaches the hooks as their
// context's deadline.
func TestWaitForSignalsGivesHooksTheDeadline(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSignals(t, os.Interrupt)

	var (
		hadDeadline bool
		remaining   time.Duration
	)
	Register(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		hadDeadline = ok
		remaining = time.Until(deadline)
		return nil
	})

	WaitForSignals(30 * time.Second)

	require.True(t, hadDeadline, "a positive timeout must reach the hook as a deadline")
	require.Positive(t, remaining, "the deadline must still be in the future when the hook runs")
	require.LessOrEqual(t, remaining, 30*time.Second)
	// A generous floor: this asserts the deadline came from the timeout rather
	// than from something much shorter, without depending on clock resolution
	// (ADR-0037 — this box cannot measure sub-millisecond intervals).
	require.Greater(t, remaining, 20*time.Second)
}

// TestZeroTimeoutImposesNoDeadline pins the escape hatch ADR-0051 keeps from
// ADR-0025: where the operator has already configured a grace period, the
// library must not invent a second one.
func TestZeroTimeoutImposesNoDeadline(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSignals(t, os.Interrupt)

	var sawDeadline bool
	Register(func(ctx context.Context) error {
		_, sawDeadline = ctx.Deadline()
		return nil
	})

	WaitForSignals(0)

	require.False(t, sawDeadline, "timeout 0 must leave hooks with an unbounded context")
}

// TestNegativeTimeoutPanics — a negative duration cannot mean anything, and a
// silently-ignored one would look like "no deadline" while reading as a bound
// (ADR-0005: loud at the call).
func TestNegativeTimeoutPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "lifecycle: negative shutdown timeout", func() {
		WaitForSignals(-1)
	})
}

// TestDeadlineBoundsASlowHookAndLaterHooksStillRun is the behaviour the timeout
// exists for, and the half that is easy to get wrong: an expired deadline must
// not abandon the rest of the sequence. A hook that ignores its context still
// runs to completion — that is the hook's choice, per ADR-0025 — so the bound is
// cooperative, and this pins both halves.
func TestDeadlineBoundsASlowHookAndLaterHooksStillRun(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSignals(t, os.Interrupt)

	var laterHookRan bool
	// Registered first, so LIFO runs it last.
	Register(func(context.Context) error {
		laterHookRan = true
		return nil
	})
	Register(func(ctx context.Context) error {
		<-ctx.Done() // honours the deadline instead of blocking forever
		return ctx.Err()
	})

	start := time.Now()
	WaitForSignals(50 * time.Millisecond)
	elapsed := time.Since(start)

	require.True(t, laterHookRan, "an expired deadline must not skip the remaining hooks")
	require.Less(t, elapsed, 5*time.Second, "the slow hook must have been released by the deadline")
}

// TestDeadlineStartsWhenTheSignalArrives pins a detail with real consequences: a
// deadline derived at call time would spend the process's entire uptime before
// shutdown began, so a long-running service would get no grace period at all.
// The wait is unblocked by Trigger after a delay, and the hook must still see
// close to the full budget.
func TestDeadlineStartsWhenTheSignalArrives(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	swapSilentSignals(t) // only Trigger can unblock the wait

	const budget = 2 * time.Second
	remaining := make(chan time.Duration, 1)
	Register(func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		remaining <- time.Until(deadline)
		return nil
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		WaitForSignals(budget)
	}()

	// Sit in the wait for a good fraction of the budget before waking it.
	time.Sleep(500 * time.Millisecond)
	Trigger()

	select {
	case got := <-remaining:
		require.Greater(t, got, budget-400*time.Millisecond,
			"the budget must start at the signal, not at the call — waiting must not consume it")
	case <-time.After(5 * time.Second):
		t.Fatal("hook never ran")
	}
	<-done
}
