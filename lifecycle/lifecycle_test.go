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

	WaitForSignals(os.Interrupt, syscall.SIGTERM)

	require.True(t, ran, "the signal triggers Shutdown")
	require.Equal(t, []os.Signal{os.Interrupt, syscall.SIGTERM}, *gotSigs,
		"the given signals are subscribed")
	require.True(t, *stopped, "signal delivery is stopped on the way out")
}

func TestWaitForSignalsDefaultsToInterruptAndTerm(t *testing.T) {
	defer goleak.VerifyNone(t)
	swapStd(t)
	gotSigs, _ := swapSignals(t, os.Interrupt)
	WaitForSignals()
	require.Equal(t, []os.Signal{os.Interrupt, syscall.SIGTERM}, *gotSigs,
		"no arguments defaults to the common termination pair")
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
	WaitForSignals()

	require.Len(t, *records, 1, "a failing shutdown is logged before WaitForSignals returns")
	require.Equal(t, "lifecycle: shutdown error", (*records)[0].Message)
	require.Equal(t, slog.LevelError, (*records)[0].Level)
}
