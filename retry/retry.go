// Package retry provides function execution with retry, exponential backoff,
// and random jitter: Backoff runs a call until it succeeds, its attempt
// budget is spent, or its context ends, sleeping between attempts with
// exponentially growing, jittered, hard-capped delays.
//
// The delay before retry i (counting from 1) starts at Policy.BaseDelay and
// doubles each retry, capped at Policy.MaxDelay; Policy.Jitter then spreads
// it uniformly over ±Jitter of its value (still never above MaxDelay), so
// simultaneous failers do not retry in lockstep. Backoff is a pure function
// with no shared state — it owns no goroutines and is safe for concurrent
// use. Design decisions are recorded in ADR-0011.
package retry

import (
	"context"
	"math/rand/v2"
	"time"
)

// Policy configures Backoff. The zero value is not usable: MaxAttempts must
// be at least 1.
type Policy struct {
	// MaxAttempts is the total number of times fn may run, counting the
	// first call: 1 means no retry. Must be > 0.
	MaxAttempts int

	// BaseDelay is the pre-jitter delay before the first retry; each further
	// retry doubles it. Zero means immediate retries. Must be >= 0.
	BaseDelay time.Duration

	// MaxDelay is the hard cap: no sleep, jittered or not, ever exceeds it.
	// Must be >= BaseDelay.
	MaxDelay time.Duration

	// Jitter is the fraction of each delay used as the jitter half-range: a
	// delay d becomes uniform in [d*(1-Jitter), d*(1+Jitter)], then is
	// re-capped at MaxDelay. Zero disables jitter. Must be in [0, 1].
	Jitter float64

	// Test seams (unexported, in-package tests only): nil selects the real
	// clock and math/rand.
	sleep func(ctx context.Context, d time.Duration) error
	rand  func() float64
}

// Backoff runs fn under policy until it returns nil, the attempt budget is
// spent, or ctx ends. It returns nil on the first success; the last error fn
// returned, verbatim, when MaxAttempts calls all failed (wrap fn to classify
// errors — Backoff never inspects them); or ctx.Err() when ctx ends before
// the first call or during a between-attempt sleep. fn receives ctx
// unchanged. A nil fn or an invalid policy panics — both are programming
// errors, not runtime conditions.
func Backoff(ctx context.Context, policy Policy, fn func(context.Context) error) error {
	if fn == nil {
		panic("retry: nil function")
	}
	policy.mustBeValid()
	if err := ctx.Err(); err != nil {
		return err
	}
	delay := policy.BaseDelay
	for attempt := 1; ; attempt++ {
		lastErr := fn(ctx)
		if lastErr == nil {
			return nil
		}
		if attempt == policy.MaxAttempts {
			return lastErr
		}
		if err := policy.wait(ctx, policy.jittered(delay)); err != nil {
			return err
		}
		delay = policy.next(delay)
	}
}

// mustBeValid panics on a policy no caller can have meant.
func (p Policy) mustBeValid() {
	switch {
	case p.MaxAttempts <= 0:
		panic("retry: Policy.MaxAttempts must be > 0")
	case p.BaseDelay < 0:
		panic("retry: Policy.BaseDelay must be >= 0")
	case p.MaxDelay < p.BaseDelay:
		panic("retry: Policy.MaxDelay must be >= Policy.BaseDelay")
	case p.Jitter < 0 || p.Jitter > 1:
		panic("retry: Policy.Jitter must be in [0, 1]")
	}
}

// jittered spreads d uniformly over ±Jitter of its value, re-capped at
// MaxDelay so the cap survives the spread.
func (p Policy) jittered(d time.Duration) time.Duration {
	if p.Jitter == 0 || d == 0 {
		return d
	}
	var u float64
	if p.rand != nil {
		u = p.rand()
	} else {
		u = rand.Float64() //nolint:gosec // G404: jitter spreads retry storms, not secrets; crypto/rand would be waste
	}
	j := time.Duration((1 + p.Jitter*(2*u-1)) * float64(d))
	switch {
	case j > p.MaxDelay:
		return p.MaxDelay
	case j < 0: // float rounding paranoia; (1-Jitter) >= 0 keeps this theoretical
		return 0
	default:
		return j
	}
}

// next doubles d, clamping at MaxDelay and on int64 overflow.
func (p Policy) next(d time.Duration) time.Duration {
	doubled := d * 2
	if doubled < d || doubled > p.MaxDelay {
		return p.MaxDelay
	}
	return doubled
}

// wait sleeps for d or until ctx ends, whichever comes first.
func (p Policy) wait(ctx context.Context, d time.Duration) error {
	if p.sleep != nil {
		return p.sleep(ctx, d)
	}
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
