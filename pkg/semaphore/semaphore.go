// Package semaphore provides Weighted, a weighted counting semaphore for
// admission control: a caller Acquires some weight before doing work and
// Releases it afterward, bounding the total concurrent weight to a fixed
// capacity. Heavier operations reserve more capacity than lighter ones, so a
// single knob caps aggregate load rather than a raw operation count.
//
// It is a thin, house-consistent adapter over golang.org/x/sync/semaphore
// (ADR-0009): the blocking and fairness are delegated there; this package
// fixes the construction and misuse contract in the module's idiom (loud
// panics on programming errors). All methods are safe for concurrent use.
// Design decisions are recorded in ADR-0009.
package semaphore

import (
	"context"

	xsync "golang.org/x/sync/semaphore"
)

// Weighted is a weighted counting semaphore bounding concurrent admissions to
// a fixed capacity. The zero value is not usable; construct one with
// NewWeighted.
type Weighted struct {
	sem *xsync.Weighted
}

// NewWeighted returns a Weighted admitting up to capacity units of concurrent
// weight. It panics if capacity <= 0: a non-positive capacity admits nothing
// and is a programming error, not a runtime condition.
func NewWeighted(capacity int64) *Weighted {
	if capacity <= 0 {
		panic("semaphore: capacity must be positive")
	}
	return &Weighted{sem: xsync.NewWeighted(capacity)}
}

// Acquire blocks until weight units are free and admits them, or returns
// ctx.Err() if ctx is canceled first. A weight exceeding the capacity can
// never be satisfied — Acquire then blocks until ctx is done and returns its
// error. It panics if weight <= 0.
func (w *Weighted) Acquire(ctx context.Context, weight int64) error {
	if weight <= 0 {
		panic("semaphore: weight must be positive")
	}
	return w.sem.Acquire(ctx, weight)
}

// Release returns weight units to the semaphore, unblocking waiters that now
// fit. It panics if weight <= 0, or (per the x/sync contract) if it would
// release more than is currently held — releasing what was never acquired is
// a programming error.
func (w *Weighted) Release(weight int64) {
	if weight <= 0 {
		panic("semaphore: weight must be positive")
	}
	w.sem.Release(weight)
}
