package ratelimit

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// NFR-04 (spec v2 §5): token-bucket admission stays within ±1% of the configured
// rate over a 10 s bursty pattern with burst = 2 × rate.
//
// This is the one NFR in the suite that is **hardware-independent**, so it is a
// test with a hard assertion rather than a benchmark to be eyeballed. The
// limiter's refill is computed from an injected clock (ADR-0012), so advancing
// fake time by 10 s exercises exactly the arithmetic the NFR is about, with no
// dependence on how fast the machine runs. That is why it can be a gate: the
// throughput and tail NFRs cannot be asserted on shared CI hardware, but this one
// is exact everywhere.
//
// It lives in the internal test package because it drives the unexported clock
// seam, the same way the existing refill tests do.

// admissionAccounting explains the expected total, which is the part of this NFR
// easy to get wrong. A limiter at rate R with burst B starts **full**, so over a
// window of T seconds the supply is:
//
//	B (the initial bucket)  +  R × T (refilled during the window)
//
// The NFR constrains the *rate*, so the initial burst must be removed before
// comparing: measured rate = (admitted − B) / T. Leaving B in would report a
// 10 s / rate-100 run as 12% fast and the NFR as failed, which is an accounting
// error rather than a defect — worth stating in the code, because a future reader
// will meet the same trap.
func TestNFR04AdmissionRateAccuracy(t *testing.T) {
	defer goleak.VerifyNone(t)

	const (
		window    = 10 * time.Second
		tick      = time.Millisecond // arrival granularity of the simulated pattern
		tolerance = 0.01             // ±1%
	)

	for _, rate := range []float64{10, 100, 1000, 12.5} {
		t.Run(fmt.Sprintf("rate=%v", rate), func(t *testing.T) {
			burst := int(2 * rate) // NFR-04: burst = 2 × rate
			clock := newFakeClock()
			l := newTestLimiter(rate, burst, clock)

			// A bursty pattern rather than a steady one, as the NFR specifies:
			// every 100 ms of fake time, a clump of arrivals three times the
			// burst size hammers the limiter; the rest of the time nothing
			// arrives. Demand therefore always exceeds supply, so admissions are
			// bounded by the bucket and not by the offered load — which is what
			// makes the measured rate a property of the limiter.
			//
			// Two harness details that a uniform −1.00% "deviation" at every rate
			// exposed while this test was being written. Both were bugs in the
			// measurement, not the limiter, and a systematic error identical
			// across rates is exactly the signature of that:
			//
			//  1. The bucket starts **full**, so an idle first window wastes its
			//     refill against the cap — min(burst, tokens+elapsed×rate) discards
			//     it, which is correct token-bucket behaviour. The initial burst is
			//     therefore drained at t=0, before the clock moves, so what follows
			//     measures the refill rate rather than the capacity.
			//  2. The clump fires on (i+1), putting arrivals at 100 ms, 200 ms, …
			//     and crucially at exactly 10 s. Firing on i left the last clump at
			//     9.901 s with the final 99 ms unharvested.
			admitted := 0
			drain := func() {
				for range burst * 3 {
					if l.Allow() {
						admitted++
					}
				}
			}
			drain() // t=0: empty the initial bucket

			ticks := int(window / tick)
			for i := range ticks {
				clock.advance(tick)
				if (i+1)%100 == 0 {
					drain()
				}
			}

			// Remove the initial burst: it is capacity the bucket started with,
			// not throughput the rate produced.
			refilled := float64(admitted - burst)
			measured := refilled / window.Seconds()
			delta := (measured - rate) / rate

			t.Logf("rate=%v burst=%d admitted=%d (burst %d + refilled %.0f) "+
				"measured=%.4f/s delta=%+.4f%%",
				rate, burst, admitted, burst, refilled, measured, delta*100)

			require.LessOrEqual(t, absFloat(delta), tolerance,
				"admission rate %.4f/s deviates %+.3f%% from the configured %v/s, "+
					"outside the ±1%% NFR-04 tolerance", measured, delta*100, rate)
		})
	}
}

// TestNFR04NeverExceedsBudget is the safety half of the same property: over any
// window the limiter must never admit *more* than the bucket can fund. A limiter
// that runs fast is a broken rate limit, not a fast one, so this is asserted
// exactly rather than within a tolerance.
func TestNFR04NeverExceedsBudget(t *testing.T) {
	defer goleak.VerifyNone(t)

	const rate = 100.0
	burst := int(2 * rate)
	clock := newFakeClock()
	l := newTestLimiter(rate, burst, clock)

	admitted := 0
	for range 10_000 { // 10 s in 1 ms ticks, saturating demand every tick
		clock.advance(time.Millisecond)
		for range 5 {
			if l.Allow() {
				admitted++
			}
		}
	}

	// Supply ceiling: the full initial bucket plus one token per 1/rate seconds.
	ceiling := burst + int(rate*10)
	require.LessOrEqual(t, admitted, ceiling,
		"admitted %d exceeds the %d tokens the bucket could fund in the window", admitted, ceiling)
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
