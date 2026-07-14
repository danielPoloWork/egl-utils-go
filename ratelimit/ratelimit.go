// Package ratelimit provides a token-bucket rate limiter: a bucket of burst
// tokens refills continuously at rate tokens per second, each admitted call
// costs one token, and callers choose between failing fast (Allow) and
// queueing for the next token (Wait).
//
// The bucket starts full, so a fresh limiter admits a burst immediately.
// Refill is computed lazily from elapsed time on each call — the limiter
// owns no goroutines and no background timers (the only timer lives inside
// a blocked Wait, and is stopped when Wait returns), so it cannot leak and
// is safe for concurrent use. Waiters reserve their token on arrival and
// sleep exactly until it is funded, giving arrival-order fairness with no
// wake-and-recheck herding; a canceled Wait repays its reservation. The
// zero value is not usable; construct a Limiter with NewLimiter. Design
// decisions are recorded in ADR-0012.
package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"
)

// Limiter is a token-bucket rate limiter. All methods are safe for
// concurrent use. The zero value is not usable; construct a Limiter with
// NewLimiter.
type Limiter struct {
	rate  float64 // tokens added per second
	burst float64 // bucket capacity

	now   func() time.Time                                 // test seam; time.Now in production
	sleep func(ctx context.Context, d time.Duration) error // test seam; timer-based wait in production

	mu     sync.Mutex
	tokens float64   // current tokens; negative is debt reserved by waiters
	last   time.Time // instant tokens was last brought current
}

// NewLimiter builds a full bucket holding burst tokens that refills at rate
// tokens per second. It panics if rate is not a positive, finite number or
// if burst < 1 (a zero-capacity bucket could never admit anything and every
// Wait would block forever) — invalid limits are programming errors, not
// runtime conditions.
func NewLimiter(rate float64, burst int) *Limiter {
	if rate <= 0 || math.IsInf(rate, 1) || math.IsNaN(rate) {
		panic("ratelimit: rate must be a positive, finite number of tokens per second")
	}
	if burst < 1 {
		panic("ratelimit: burst must be >= 1")
	}
	return &Limiter{
		rate:   rate,
		burst:  float64(burst),
		now:    time.Now,
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// Allow reports whether a token is available right now, consuming it when
// so. It never blocks: a false means the caller should shed the work.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.advance()
	if l.tokens >= 1 {
		l.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available or ctx ends, consuming the token
// on success. The token is reserved on arrival, so concurrent waiters are
// served in the order they arrive and each sleeps exactly until its own
// token is funded. If ctx is already done, or ends while waiting, Wait
// returns ctx.Err() and the reservation is repaid — a canceled Wait never
// costs a token.
func (l *Limiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	l.advance()
	l.tokens-- // reserve this caller's token, possibly as debt
	shortfall := -l.tokens
	l.mu.Unlock()
	if shortfall <= 0 {
		return nil
	}
	d := time.Duration(math.Ceil(shortfall / l.rate * float64(time.Second)))
	if err := l.wait(ctx, d); err != nil {
		l.mu.Lock()
		l.advance()
		l.tokens = min(l.burst, l.tokens+1) // repay the reservation
		l.mu.Unlock()
		return err
	}
	return nil
}

// advance brings tokens current with the clock; the caller holds mu.
func (l *Limiter) advance() {
	now := l.now()
	elapsed := now.Sub(l.last)
	if elapsed <= 0 {
		return
	}
	l.tokens = min(l.burst, l.tokens+elapsed.Seconds()*l.rate)
	l.last = now
}

// wait sleeps for d or until ctx ends, whichever comes first.
func (l *Limiter) wait(ctx context.Context, d time.Duration) error {
	if l.sleep != nil {
		return l.sleep(ctx, d)
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
