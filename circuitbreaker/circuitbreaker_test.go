package circuitbreaker_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/circuitbreaker"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var errBoom = errors.New("boom")

func TestDoNilFunctionPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New()
	require.Panics(t, func() { _ = b.Do(context.Background(), nil) })
}

func TestOptionsPanicOnNonPositiveValues(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Panics(t, func() { circuitbreaker.WithFailureThreshold(0) })
	require.Panics(t, func() { circuitbreaker.WithFailureThreshold(-1) })
	require.Panics(t, func() { circuitbreaker.WithSuccessThreshold(0) })
	require.Panics(t, func() { circuitbreaker.WithSuccessThreshold(-1) })
	require.Panics(t, func() { circuitbreaker.WithOpenTimeout(0) })
	require.Panics(t, func() { circuitbreaker.WithOpenTimeout(-time.Second) })
}

func TestStateString(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Equal(t, "closed", circuitbreaker.StateClosed.String())
	require.Equal(t, "open", circuitbreaker.StateOpen.String())
	require.Equal(t, "half-open", circuitbreaker.StateHalfOpen.String())
	require.Equal(t, "unknown", circuitbreaker.State(99).String())
	require.Equal(t, circuitbreaker.StateClosed, circuitbreaker.State(0), "the zero value is closed")
}

func TestStateObservable(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(circuitbreaker.WithFailureThreshold(1))
	require.Equal(t, circuitbreaker.StateClosed, b.State())

	require.ErrorIs(t, b.Do(context.Background(), func() error { return errBoom }), errBoom)
	require.Equal(t, circuitbreaker.StateOpen, b.State(), "one failure at threshold 1 trips it open")
}

func TestClosedPassesResultsThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New()
	bg := context.Background()

	require.NoError(t, b.Do(bg, func() error { return nil }))
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	// One failure under the default threshold of five: still closed.
	require.NoError(t, b.Do(bg, func() error { return nil }))
}

func TestConsecutiveFailuresTripTheBreaker(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(circuitbreaker.WithFailureThreshold(3))
	bg := context.Background()

	for range 3 {
		require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	}

	ran := false
	err := b.Do(bg, func() error { ran = true; return nil })
	require.ErrorIs(t, err, circuitbreaker.ErrOpen)
	require.False(t, ran, "a rejected call must not run")
}

func TestSuccessResetsConsecutiveFailures(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(circuitbreaker.WithFailureThreshold(2))
	bg := context.Background()

	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	require.NoError(t, b.Do(bg, func() error { return nil })) // resets the count
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	// Only one consecutive failure so far: the next call still runs — and,
	// failing, becomes the second of the two that trip the breaker.
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	require.ErrorIs(t, b.Do(bg, func() error { return nil }), circuitbreaker.ErrOpen)
}

func TestCanceledContextIsNotRunOrCounted(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(circuitbreaker.WithFailureThreshold(2))
	bg := context.Background()

	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom) // failure 1 of 2

	ctx, cancel := context.WithCancel(bg)
	cancel()
	ran := false
	require.ErrorIs(t, b.Do(ctx, func() error { ran = true; return nil }), context.Canceled)
	require.False(t, ran, "a call with a done context must not run")

	// The canceled call recorded neither a success (which would have reset
	// the count) nor a failure (which would have tripped already): this
	// failure is the second consecutive one and trips the breaker.
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	require.ErrorIs(t, b.Do(bg, func() error { return nil }), circuitbreaker.ErrOpen)
}

func TestPanicCountsAsFailureAndPropagates(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(circuitbreaker.WithFailureThreshold(1))
	bg := context.Background()

	require.PanicsWithValue(t, "kaboom", func() {
		_ = b.Do(bg, func() error { panic("kaboom") })
	})

	ran := false
	require.ErrorIs(t, b.Do(bg, func() error { ran = true; return nil }), circuitbreaker.ErrOpen)
	require.False(t, ran, "the panic must have tripped the threshold-1 breaker")
}

func TestBreakerRecoversThroughHalfOpen(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(1),
		circuitbreaker.WithOpenTimeout(150*time.Millisecond),
	)
	bg := context.Background()

	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)

	ran := false
	require.ErrorIs(t, b.Do(bg, func() error { ran = true; return nil }), circuitbreaker.ErrOpen)
	require.False(t, ran, "the breaker must reject while cooling down")

	time.Sleep(250 * time.Millisecond)
	require.NoError(t, b.Do(bg, func() error { return nil })) // the half-open probe
	require.NoError(t, b.Do(bg, func() error { return nil })) // closed again
}

func TestHalfOpenProbeBudgetBoundsAdmission(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(1),
		circuitbreaker.WithSuccessThreshold(2),
		circuitbreaker.WithOpenTimeout(150*time.Millisecond),
	)
	bg := context.Background()

	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	time.Sleep(250 * time.Millisecond)

	// Occupy both probe slots with calls blocked mid-flight.
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	probeErrs := make(chan error, 2)
	var probes sync.WaitGroup
	probes.Add(2)
	for range 2 {
		go func() {
			defer probes.Done()
			probeErrs <- b.Do(bg, func() error {
				started <- struct{}{}
				<-release
				return nil
			})
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("probe was not admitted in time")
		}
	}

	// Budget exhausted: a third call is rejected without running.
	ran := false
	require.ErrorIs(t, b.Do(bg, func() error { ran = true; return nil }), circuitbreaker.ErrOpen)
	require.False(t, ran)

	close(release)
	probes.Wait()
	close(probeErrs)
	for err := range probeErrs {
		require.NoError(t, err)
	}

	// Two probe successes: closed again.
	require.NoError(t, b.Do(bg, func() error { return nil }))
}

func TestConcurrentUseIsRaceClean(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(3),
		circuitbreaker.WithSuccessThreshold(2),
		circuitbreaker.WithOpenTimeout(time.Millisecond),
	)
	bg := context.Background()

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func() {
			defer wg.Done()
			for i := range 200 {
				err := b.Do(bg, func() error {
					if (w+i)%3 == 0 {
						return errBoom
					}
					return nil
				})
				if err != nil && !errors.Is(err, errBoom) && !errors.Is(err, circuitbreaker.ErrOpen) {
					t.Errorf("unexpected error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
