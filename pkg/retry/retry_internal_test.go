package retry

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

// withSeams installs the deterministic test seams on p: every requested
// sleep is appended to *sleeps instead of elapsing, and the jitter draw is
// fed from us (round-robin; 0.5 when us is empty).
func withSeams(p Policy, sleeps *[]time.Duration, us []float64) Policy {
	i := 0
	p.sleep = func(ctx context.Context, d time.Duration) error {
		*sleeps = append(*sleeps, d)
		return ctx.Err()
	}
	p.rand = func() float64 {
		if len(us) == 0 {
			return 0.5
		}
		u := us[i%len(us)]
		i++
		return u
	}
	return p
}

func TestDelaysDoubleAndCapWithoutJitter(t *testing.T) {
	defer goleak.VerifyNone(t)
	var sleeps []time.Duration
	p := withSeams(Policy{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    350 * time.Millisecond,
	}, &sleeps, nil)

	err := Backoff(context.Background(), p, func(context.Context) error { return errors.New("boom") })
	require.Error(t, err)
	require.Equal(t, []time.Duration{
		100 * time.Millisecond, // base
		200 * time.Millisecond, // doubled
		350 * time.Millisecond, // 400 capped
		350 * time.Millisecond, // stays at the cap
	}, sleeps)
}

func TestJitterSpreadsWithinBoundsAndRespectsCap(t *testing.T) {
	defer goleak.VerifyNone(t)
	var sleeps []time.Duration
	p := withSeams(Policy{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    110 * time.Millisecond,
		Jitter:      0.5,
	}, &sleeps, []float64{0, 0.9999999})

	_ = Backoff(context.Background(), p, func(context.Context) error { return errors.New("boom") })
	require.Len(t, sleeps, 2)
	// u=0 lands the low edge exactly: 100ms * (1 - 0.5).
	require.Equal(t, 50*time.Millisecond, sleeps[0])
	// Second pre-jitter delay is next(100ms)=200ms capped to 110ms; u≈1
	// pushes it up ~1.5x, and the cap must survive the spread.
	require.Equal(t, 110*time.Millisecond, sleeps[1])
}

func TestNextClampsOnOverflowAndCap(t *testing.T) {
	defer goleak.VerifyNone(t)
	maxDur := time.Duration(math.MaxInt64)
	p := Policy{MaxAttempts: 2, MaxDelay: maxDur}
	require.Equal(t, maxDur, p.next(maxDur/2+1), "doubling past int64 must clamp")
	require.Equal(t, maxDur, p.next(maxDur), "doubling the max duration must clamp")

	capped := Policy{MaxAttempts: 2, MaxDelay: 100}
	require.Equal(t, time.Duration(10), capped.next(5))
	require.Equal(t, time.Duration(100), capped.next(60), "120 exceeds the cap")
}

func TestZeroBaseDelayRetriesImmediately(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Real wait path: a zero delay never touches a timer.
	p := Policy{MaxAttempts: 3}
	calls := 0
	err := Backoff(context.Background(), p, func(context.Context) error {
		calls++
		return errors.New("boom")
	})
	require.Error(t, err)
	require.Equal(t, 3, calls)
}

// TestBackoffDelayBoundInvariantsProperty drives rapid-generated policies
// through Backoff with an always-failing fn and asserts the documented
// bounds: exactly MaxAttempts calls, one sleep per gap, and every sleep
// within [(1-J)·exp, min((1+J)·exp, MaxDelay)] for the capped exponential
// envelope exp (±1ns for float truncation).
func TestBackoffDelayBoundInvariantsProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	rapid.Check(t, func(rt *rapid.T) {
		attempts := rapid.IntRange(1, 8).Draw(rt, "maxAttempts")
		base := time.Duration(rapid.Int64Range(0, int64(time.Second)).Draw(rt, "baseDelay"))
		maxD := time.Duration(rapid.Int64Range(int64(base), 2*int64(time.Second)).Draw(rt, "maxDelay"))
		jitter := rapid.Float64Range(0, 1).Draw(rt, "jitter")

		us := make([]float64, 0, attempts-1)
		for range attempts - 1 {
			us = append(us, rapid.Float64Range(0, 0.9999999).Draw(rt, "u"))
		}

		var sleeps []time.Duration
		p := withSeams(Policy{
			MaxAttempts: attempts,
			BaseDelay:   base,
			MaxDelay:    maxD,
			Jitter:      jitter,
		}, &sleeps, us)

		errBoom := errors.New("boom")
		calls := 0
		err := Backoff(context.Background(), p, func(context.Context) error {
			calls++
			return errBoom
		})

		require.Equal(rt, errBoom, err, "exhaustion must return the last error verbatim")
		require.Equal(rt, attempts, calls, "fn must run exactly MaxAttempts times")
		require.Len(rt, sleeps, attempts-1, "one sleep between consecutive attempts")

		exp := base
		for i, got := range sleeps {
			low := time.Duration((1 - jitter) * float64(exp))
			high := time.Duration((1 + jitter) * float64(exp))
			if high > maxD {
				high = maxD
			}
			require.GreaterOrEqualf(rt, got, low-1, "sleep %d below the jitter floor (exp %v)", i, exp)
			require.LessOrEqualf(rt, got, high+1, "sleep %d above the jitter ceiling (exp %v)", i, exp)
			require.LessOrEqualf(rt, got, maxD, "sleep %d exceeds MaxDelay", i)
			exp = p.next(exp)
		}
	})
}
