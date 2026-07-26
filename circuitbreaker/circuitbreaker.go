// Package circuitbreaker provides a concurrency-safe circuit breaker
// guarding calls to an unreliable dependency: a closed/open/half-open state
// machine that fails fast with ErrOpen while the dependency recovers, then
// re-admits a bounded number of probe calls before closing again.
//
// Closed is the normal state: every call runs, and consecutive failures are
// counted. Reaching the failure threshold trips the breaker open: calls are
// rejected with ErrOpen without running, for the configured open timeout.
// After the timeout the breaker turns half-open and admits up to the success
// threshold of concurrent probe calls; that many successes close it again,
// while any probe failure reopens it and restarts the full cool-down.
//
// The breaker owns no goroutines and no timers — time-based transitions are
// evaluated lazily on admission — so it cannot leak and is safe for
// concurrent use. State reports the current position for observability
// (metrics, health), reflecting the lazy transition without performing it. The
// zero value is not usable; construct a Breaker with New. Design decisions are
// recorded in ADR-0010 (State observability added per ADR-0030).
package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrOpen is returned by Do when the call is rejected without running: the
// breaker is open and still cooling down, or half-open with its probe budget
// exhausted.
var ErrOpen = errors.New("circuitbreaker: circuit is open")

// Defaults applied by New when the corresponding option is absent.
const (
	defaultFailureThreshold = 5
	defaultSuccessThreshold = 1
	defaultOpenTimeout      = 30 * time.Second
)

// State is a position in the closed/open/half-open machine, reported by
// (*Breaker).State. Its zero value is StateClosed — the state of a fresh
// breaker.
type State uint8

const (
	// StateClosed admits every call and counts consecutive failures.
	StateClosed State = iota
	// StateOpen rejects calls with ErrOpen while cooling down.
	StateOpen
	// StateHalfOpen admits a bounded number of probe calls.
	StateHalfOpen
)

// String returns the lowercase state name ("closed", "open", "half-open").
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Breaker is a closed/open/half-open circuit breaker. All methods are safe
// for concurrent use. The zero value is not usable; construct a Breaker with
// New.
type Breaker struct {
	failureThreshold int           // consecutive failures in closed that trip the breaker
	successThreshold int           // half-open successes that close it; also the probe budget
	openTimeout      time.Duration // cool-down before half-open probes are admitted

	now func() time.Time // injectable clock for deterministic tests

	mu    sync.Mutex
	state State
	// generation is bumped on every state transition. An outcome is recorded
	// only against the generation that admitted the call, so a slow call that
	// completes after the breaker has moved on cannot corrupt the counters of
	// the state it never ran under.
	generation uint64
	failures   int       // consecutive failures, counted in completion order (closed)
	successes  int       // recorded probe successes (half-open)
	inFlight   int       // admitted, not yet resolved probes (half-open)
	openedAt   time.Time // when the breaker last tripped open
}

// New builds a Breaker. Without options it trips after 5 consecutive
// failures, stays open for 30 seconds, and closes again after 1 successful
// half-open probe; tune those with WithFailureThreshold, WithOpenTimeout,
// and WithSuccessThreshold.
func New(opts ...Option) *Breaker {
	b := &Breaker{
		failureThreshold: defaultFailureThreshold,
		successThreshold: defaultSuccessThreshold,
		openTimeout:      defaultOpenTimeout,
		now:              time.Now,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Do runs fn under the breaker's admission policy. If the call is rejected —
// the breaker is open and cooling down, or half-open with its probe budget
// exhausted — Do returns ErrOpen without invoking fn. If ctx is already done
// Do returns ctx.Err() without invoking fn, and the call is not counted:
// caller cancellation says nothing about the dependency's health. Otherwise
// Do returns fn's error verbatim.
//
// The outcome accounting is nil/non-nil: a nil return counts as a success, a
// non-nil return as a failure (Do never inspects the error value — a caller
// that must exempt some errors wraps fn accordingly). A panicking fn counts
// as a failure and the panic propagates untouched. A nil fn panics.
func (b *Breaker) Do(ctx context.Context, fn func() error) error {
	if fn == nil {
		panic("circuitbreaker: nil function")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	gen, err := b.admit()
	if err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed { // fn panicked: count the failure, let the panic propagate
			b.record(gen, false)
		}
	}()
	callErr := fn()
	completed = true
	b.record(gen, callErr == nil)
	return callErr
}

// admit decides whether a call may run now and returns the generation it was
// admitted under. Time-based transitions happen here: the first admission
// attempt after the open cool-down elapses moves the breaker to half-open
// and becomes its first probe.
func (b *Breaker) admit() (uint64, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == StateOpen {
		if b.now().Sub(b.openedAt) < b.openTimeout {
			return 0, ErrOpen
		}
		b.transition(StateHalfOpen)
	}
	if b.state == StateHalfOpen {
		// Admit while unresolved probes plus recorded successes fit the
		// budget: never more trial traffic than the successes still needed.
		if b.successes+b.inFlight >= b.successThreshold {
			return 0, ErrOpen
		}
		b.inFlight++
	}
	return b.generation, nil
}

// record applies a call's outcome to the state that admitted it. An outcome
// from a superseded generation is discarded: the episode it belongs to has
// already ended.
func (b *Breaker) record(gen uint64, success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if gen != b.generation {
		return
	}
	switch b.state {
	case StateClosed:
		if success {
			b.failures = 0
			return
		}
		b.failures++
		if b.failures >= b.failureThreshold {
			b.transition(StateOpen)
		}
	case StateHalfOpen:
		b.inFlight--
		if !success {
			b.transition(StateOpen) // one failed probe reopens; the cool-down restarts
			return
		}
		b.successes++
		if b.successes >= b.successThreshold {
			b.transition(StateClosed)
		}
	case StateOpen:
		// Unreachable with a matching generation: every transition into
		// stateOpen bumps the generation, orphaning outstanding calls.
	}
}

// transition moves the machine to a new state, resets the per-state
// counters, and bumps the generation so outstanding calls admitted before
// the transition are recorded against nothing.
func (b *Breaker) transition(to State) {
	b.state = to
	b.generation++
	b.failures = 0
	b.successes = 0
	b.inFlight = 0
	if to == StateOpen {
		b.openedAt = b.now()
	}
}

// State returns the breaker's current state. It reflects the lazy, time-based
// transition without performing it: an open breaker whose cool-down has
// elapsed reports StateHalfOpen — the state the next call would be admitted
// under — even though no call has yet triggered the move. State is a pure
// observer: it never admits a probe, mutates the breaker, or advances the
// generation, so polling it (e.g. for metrics) is free of side effects.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateOpen && b.now().Sub(b.openedAt) >= b.openTimeout {
		return StateHalfOpen
	}
	return b.state
}
