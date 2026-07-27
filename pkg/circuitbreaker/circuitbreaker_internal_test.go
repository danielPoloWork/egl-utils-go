package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

// fakeClock is a deterministic time source: the breaker's lazy time-based
// transitions are driven by advance, never by the wall clock.
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

// snapshot reads the breaker's internals under its own lock.
func snapshot(b *Breaker) (st State, failures, successes, inFlight int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state, b.failures, b.successes, b.inFlight
}

func TestNewAppliesDocumentedDefaults(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := New()
	require.Equal(t, 5, b.failureThreshold)
	require.Equal(t, 1, b.successThreshold)
	require.Equal(t, 30*time.Second, b.openTimeout)
}

// TestStateReflectsLazyTransitionWithoutMutating pins the observability
// contract: State() reports the effective state (an open breaker past its
// cool-down reads half-open) but never performs the transition — the stored
// state and generation are untouched, so polling has no side effect.
func TestStateReflectsLazyTransition(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	b := New(WithFailureThreshold(1), WithOpenTimeout(100*time.Millisecond))
	b.now = clock.now
	bg := context.Background()

	require.Equal(t, StateClosed, b.State(), "a fresh breaker is closed")

	require.Error(t, b.Do(bg, func() error { return errors.New("boom") }))
	require.Equal(t, StateOpen, b.State(), "tripped breaker is open")

	clock.advance(99 * time.Millisecond)
	require.Equal(t, StateOpen, b.State(), "still open one tick short of the timeout")

	clock.advance(time.Millisecond) // cool-down elapsed
	genBefore := b.generation
	require.Equal(t, StateHalfOpen, b.State(), "past the cool-down State reports half-open")

	// The pure-observer property: State() did not perform the transition.
	rawState, _, _, inFlight := snapshot(b)
	require.Equal(t, StateOpen, rawState, "State must not mutate the stored state")
	require.Equal(t, genBefore, b.generation, "State must not advance the generation")
	require.Equal(t, 0, inFlight, "State must not admit a probe")

	// A real call now performs the transition and closes the breaker.
	require.NoError(t, b.Do(bg, func() error { return nil }))
	require.Equal(t, StateClosed, b.State())
}

func TestOpenAdmitsProbeExactlyAtTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	b := New(WithFailureThreshold(1), WithOpenTimeout(100*time.Millisecond))
	b.now = clock.now
	bg := context.Background()

	errBoom := errors.New("boom")
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)

	clock.advance(99 * time.Millisecond)
	ran := false
	require.ErrorIs(t, b.Do(bg, func() error { ran = true; return nil }), ErrOpen)
	require.False(t, ran, "one clock tick short of the timeout must still reject")

	clock.advance(time.Millisecond) // exactly the timeout: the next call is the probe
	require.NoError(t, b.Do(bg, func() error { return nil }))
	st, _, _, _ := snapshot(b)
	require.Equal(t, StateClosed, st)
}

func TestFailedProbeRestartsTheFullCoolDown(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	b := New(WithFailureThreshold(1), WithOpenTimeout(100*time.Millisecond))
	b.now = clock.now
	bg := context.Background()

	errBoom := errors.New("boom")
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)

	clock.advance(100 * time.Millisecond)
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom) // the probe fails

	// Reopened at the probe's failure: the cool-down starts over in full.
	clock.advance(99 * time.Millisecond)
	require.ErrorIs(t, b.Do(bg, func() error { return nil }), ErrOpen)

	clock.advance(time.Millisecond)
	require.NoError(t, b.Do(bg, func() error { return nil }))
	st, _, _, _ := snapshot(b)
	require.Equal(t, StateClosed, st)
}

// TestStaleClosedOutcomeIsDiscarded pins the generation guard: a call
// admitted in one closed episode that completes only after the breaker has
// tripped and fully recovered must not count against the new closed state.
func TestStaleClosedOutcomeIsDiscarded(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	b := New(WithFailureThreshold(1), WithOpenTimeout(100*time.Millisecond))
	b.now = clock.now
	bg := context.Background()
	errBoom := errors.New("boom")

	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- b.Do(bg, func() error {
			close(entered)
			<-release
			return errBoom // completes long after this closed episode ended
		})
	}()
	<-entered

	// Trip and fully recover while the slow call is still in flight.
	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	clock.advance(100 * time.Millisecond)
	require.NoError(t, b.Do(bg, func() error { return nil }))

	close(release)
	require.ErrorIs(t, <-done, errBoom) // the slow caller still sees its own error

	// With threshold 1, the stale failure would have tripped the breaker had
	// it been counted against the new generation.
	st, failures, _, _ := snapshot(b)
	require.Equal(t, StateClosed, st)
	require.Equal(t, 0, failures)
	require.NoError(t, b.Do(bg, func() error { return nil }))
}

// TestOrphanedProbeOutcomeIsDiscarded pins the half-open side of the
// generation guard: when one probe's failure reopens the breaker, a sibling
// probe still in flight belongs to a dead episode and its outcome must not
// leak into the new one.
func TestOrphanedProbeOutcomeIsDiscarded(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	b := New(
		WithFailureThreshold(1),
		WithSuccessThreshold(2),
		WithOpenTimeout(100*time.Millisecond),
	)
	b.now = clock.now
	bg := context.Background()
	errBoom := errors.New("boom")

	require.ErrorIs(t, b.Do(bg, func() error { return errBoom }), errBoom)
	clock.advance(100 * time.Millisecond)

	// Admit both probes and hold them mid-flight.
	type probe struct {
		entered chan struct{}
		release chan struct{}
		done    chan error
		result  error
	}
	probes := [2]*probe{
		{entered: make(chan struct{}), release: make(chan struct{}), done: make(chan error, 1), result: errBoom},
		{entered: make(chan struct{}), release: make(chan struct{}), done: make(chan error, 1), result: nil},
	}
	for _, p := range probes {
		go func() {
			p.done <- b.Do(bg, func() error {
				close(p.entered)
				<-p.release
				return p.result
			})
		}()
		<-p.entered
	}

	// The failing probe reopens the breaker; its sibling is orphaned.
	close(probes[0].release)
	require.ErrorIs(t, <-probes[0].done, errBoom)
	st, _, _, inFlight := snapshot(b)
	require.Equal(t, StateOpen, st)
	require.Equal(t, 0, inFlight)

	// The orphaned probe's success reaches its caller but is discarded by
	// the breaker: still open, no successes carried into the next episode.
	close(probes[1].release)
	require.NoError(t, <-probes[1].done)
	st, _, successes, inFlight := snapshot(b)
	require.Equal(t, StateOpen, st)
	require.Equal(t, 0, successes)
	require.Equal(t, 0, inFlight)

	// A fresh half-open episode still needs its full two successes.
	clock.advance(100 * time.Millisecond)
	require.NoError(t, b.Do(bg, func() error { return nil }))
	st, _, _, _ = snapshot(b)
	require.Equal(t, StateHalfOpen, st)
	require.NoError(t, b.Do(bg, func() error { return nil }))
	st, _, _, _ = snapshot(b)
	require.Equal(t, StateClosed, st)
}

// breakerModel is a sequential reference model of the breaker's documented
// contract; the rapid property drives it alongside the real implementation.
type breakerModel struct {
	failureThreshold int
	successThreshold int
	openTimeout      time.Duration

	st          State
	failures    int
	successes   int
	openElapsed time.Duration
}

func (m *breakerModel) advance(d time.Duration) {
	m.openElapsed += d
}

// call feeds one call outcome through the model and reports whether the
// breaker should have executed it.
func (m *breakerModel) call(success bool) bool {
	if m.st == StateOpen {
		if m.openElapsed < m.openTimeout {
			return false
		}
		m.st = StateHalfOpen
		m.successes = 0
	}
	if m.st == StateHalfOpen {
		if !success {
			m.st = StateOpen
			m.openElapsed = 0
			return true
		}
		m.successes++
		if m.successes >= m.successThreshold {
			m.st = StateClosed
			m.failures = 0
		}
		return true
	}
	if success {
		m.failures = 0
		return true
	}
	m.failures++
	if m.failures >= m.failureThreshold {
		m.st = StateOpen
		m.openElapsed = 0
	}
	return true
}

// TestBreakerMatchesSequentialModel drives rapid-generated sequences of
// calls and clock advances through the breaker and the reference model and
// asserts they never diverge — in per-call admission or in state.
func TestBreakerMatchesSequentialModel(t *testing.T) {
	defer goleak.VerifyNone(t)
	rapid.Check(t, func(rt *rapid.T) {
		failN := rapid.IntRange(1, 4).Draw(rt, "failureThreshold")
		succN := rapid.IntRange(1, 3).Draw(rt, "successThreshold")
		timeout := time.Duration(rapid.IntRange(1, 150).Draw(rt, "openTimeoutMs")) * time.Millisecond

		clock := newFakeClock()
		b := New(
			WithFailureThreshold(failN),
			WithSuccessThreshold(succN),
			WithOpenTimeout(timeout),
		)
		b.now = clock.now
		m := &breakerModel{
			failureThreshold: failN,
			successThreshold: succN,
			openTimeout:      timeout,
		}

		errBoom := errors.New("boom")
		bg := context.Background()
		steps := rapid.IntRange(1, 60).Draw(rt, "steps")
		for range steps {
			switch rapid.IntRange(0, 2).Draw(rt, "op") {
			case 0: // advance the clock
				d := time.Duration(rapid.IntRange(1, 100).Draw(rt, "advanceMs")) * time.Millisecond
				clock.advance(d)
				m.advance(d)
			case 1: // successful call
				err := b.Do(bg, func() error { return nil })
				if m.call(true) {
					require.NoError(rt, err)
				} else {
					require.ErrorIs(rt, err, ErrOpen)
				}
			case 2: // failing call
				err := b.Do(bg, func() error { return errBoom })
				if m.call(false) {
					require.ErrorIs(rt, err, errBoom)
				} else {
					require.ErrorIs(rt, err, ErrOpen)
				}
			}
			st, _, _, _ := snapshot(b)
			require.Equal(rt, m.st, st, "state diverged from the model")
		}
	})
}
