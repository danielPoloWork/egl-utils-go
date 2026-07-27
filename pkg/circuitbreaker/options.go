package circuitbreaker

import "time"

// Option customises a Breaker at construction time (functional options;
// ADR-0005, ADR-0010).
type Option func(*Breaker)

// WithFailureThreshold sets how many consecutive failures in the closed
// state trip the breaker open (default 5). It panics if n <= 0 — an invalid
// threshold is a programming error, not a runtime condition.
func WithFailureThreshold(n int) Option {
	if n <= 0 {
		panic("circuitbreaker: failure threshold must be > 0")
	}
	return func(b *Breaker) { b.failureThreshold = n }
}

// WithOpenTimeout sets how long the breaker stays open before admitting
// half-open probes (default 30s). It panics if d <= 0.
func WithOpenTimeout(d time.Duration) Option {
	if d <= 0 {
		panic("circuitbreaker: open timeout must be > 0")
	}
	return func(b *Breaker) { b.openTimeout = d }
}

// WithSuccessThreshold sets how many successful half-open probes close the
// breaker (default 1). The same value caps the probes admitted concurrently
// while half-open: never more trial traffic than the successes still needed.
// It panics if n <= 0.
func WithSuccessThreshold(n int) Option {
	if n <= 0 {
		panic("circuitbreaker: success threshold must be > 0")
	}
	return func(b *Breaker) { b.successThreshold = n }
}
