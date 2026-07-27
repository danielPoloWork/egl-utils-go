package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

// fakeClock is a deterministic time source: refill is driven by advance,
// never by the wall clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Unix(0, 0)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTestLimiter pins the limiter to the fake clock.
func newTestLimiter(rate float64, burst int, c *fakeClock) *Limiter {
	l := NewLimiter(rate, burst)
	l.now = c.now
	l.last = c.now()
	return l
}

// snapshotTokens reads the bucket level under the limiter's own lock.
func snapshotTokens(l *Limiter) float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.tokens
}

func TestRefillIsProportionalAndCapped(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	// 4 tokens/s and 250ms steps keep every value exact in float64.
	l := newTestLimiter(4, 2, clock)

	require.True(t, l.Allow())
	require.True(t, l.Allow())
	require.False(t, l.Allow(), "the burst is spent and the clock is frozen")

	clock.advance(250 * time.Millisecond) // funds exactly one token
	require.True(t, l.Allow())
	require.False(t, l.Allow(), "the single funded token is spent")

	clock.advance(10 * time.Second) // far more than burst/rate: capped at burst
	require.True(t, l.Allow())
	require.True(t, l.Allow())
	require.False(t, l.Allow(), "refill must cap at the burst capacity")
}

func TestWaitReservesInArrivalOrder(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	l := newTestLimiter(4, 1, clock)

	var sleeps []time.Duration
	l.sleep = func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return ctx.Err()
	}

	require.True(t, l.Allow())
	ctx := context.Background()
	require.NoError(t, l.Wait(ctx)) // first waiter: 1 token short
	require.NoError(t, l.Wait(ctx)) // second waiter: queued behind the first
	require.NoError(t, l.Wait(ctx)) // third
	require.Equal(t, []time.Duration{
		250 * time.Millisecond,
		500 * time.Millisecond,
		750 * time.Millisecond,
	}, sleeps, "each waiter sleeps exactly until its own token is funded")
}

func TestCanceledWaitRepaysItsReservation(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	l := newTestLimiter(4, 1, clock)
	require.True(t, l.Allow()) // tokens: 0

	ctx, cancel := context.WithCancel(context.Background())
	l.sleep = func(ctx context.Context, _ time.Duration) error {
		cancel() // the caller gives up mid-wait
		return ctx.Err()
	}
	require.ErrorIs(t, l.Wait(ctx), context.Canceled)
	require.InDelta(t, 0, snapshotTokens(l), 1e-9,
		"the reservation must be repaid: the bucket is back where it was")

	// Exactly one 250ms refill funds exactly one admission — a lost or
	// double-repaid token would break one of these two assertions.
	clock.advance(250 * time.Millisecond)
	require.True(t, l.Allow())
	require.False(t, l.Allow())
}

func TestWaitConsumesExactlyItsFundedToken(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	l := newTestLimiter(4, 1, clock)
	// The seam models a real sleep: the clock advances by the requested
	// duration and the wait succeeds.
	l.sleep = func(_ context.Context, d time.Duration) error {
		clock.advance(d)
		return nil
	}

	require.True(t, l.Allow())
	require.NoError(t, l.Wait(context.Background()))
	// The token funded during the sleep belonged to the waiter: the bucket
	// is empty again until the next 250ms elapses.
	require.False(t, l.Allow())
	clock.advance(250 * time.Millisecond)
	require.True(t, l.Allow())
}

// TestAdmissionObeysTheBucketLawProperty drives rapid-generated op
// sequences (advance / Allow) through a fake-clock limiter and asserts the
// token-bucket law: total admissions never exceed burst + rate·elapsed.
func TestAdmissionObeysTheBucketLawProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	rapid.Check(t, func(rt *rapid.T) {
		rate := rapid.Float64Range(0.5, 1000).Draw(rt, "rate")
		burst := rapid.IntRange(1, 10).Draw(rt, "burst")

		clock := newFakeClock()
		l := newTestLimiter(rate, burst, clock)

		admitted := 0
		var elapsed time.Duration
		steps := rapid.IntRange(1, 100).Draw(rt, "steps")
		for range steps {
			if rapid.Bool().Draw(rt, "advance") {
				d := time.Duration(rapid.IntRange(1, 500).Draw(rt, "ms")) * time.Millisecond
				clock.advance(d)
				elapsed += d
				continue
			}
			if l.Allow() {
				admitted++
			}
		}

		bound := float64(burst) + rate*elapsed.Seconds() + 1e-3
		require.LessOrEqualf(rt, float64(admitted), bound,
			"admitted %d exceeds the bucket law bound %.3f (rate %.3f, burst %d, elapsed %v)",
			admitted, bound, rate, burst, elapsed)
	})
}
