package ratelimit

import (
	"errors"
	"math"
	"net/http"
	"strconv"
)

// ErrLimited reports that a limiter denied admission — the bucket was empty
// and the caller should shed the work. It is the canonical sentinel for the
// fail-fast Allow path: an http.Handler cannot return an error, so Middleware
// signals this condition to the client as 429 Too Many Requests, and code that
// gates its own work on Allow returns ErrLimited so its callers can recognise
// the refusal with errors.Is:
//
//	if !limiter.Allow() {
//		return ratelimit.ErrLimited
//	}
//
// A limited request is a normal operating condition, not a fault: it means the
// configured budget is doing its job.
var ErrLimited = errors.New("ratelimit: rate limit exceeded")

// Middleware returns net/http middleware that admits each request through the
// limiter, passing it to next when a token is available and answering 429 Too
// Many Requests when the bucket is empty. It is an ordinary decorator —
// func(http.Handler) http.Handler — so it composes with the middleware package
// and any third party that speaks the same shape.
//
// Admission uses Allow, never Wait: a limited request is refused immediately
// rather than parked until a token is funded. Blocking would hold the server's
// goroutine and its connection for the duration, converting an over-budget
// burst into a queue that grows without bound — shedding load is the point of
// the limiter, so the middleware sheds.
//
// The response carries a Retry-After of ceil(1/rate) seconds — the worst-case
// wait for a single token, a conservative hint rounded up to the whole seconds
// RFC 9110 requires — and a body of "Too Many Requests". Nothing about the
// limiter's configuration or current level is disclosed. Denials need no
// logging here: they surface as ordinary 429s to whatever observes the chain,
// so pairing this with middleware.Logger records them without this package
// touching a logger, and without giving a client a way to flood the logs.
//
// The limiter is shared by every request the middleware admits, so it bounds
// total throughput rather than any one client's share: one heavy caller can
// consume the whole budget. Per-client limiting is the consumer's decision —
// keep a limiter per key (and evict idle keys) and gate with Allow directly,
// since a fair per-key policy needs a key-extraction and eviction policy this
// package does not presume.
//
// Middleware may be called more than once on the same Limiter; every returned
// decorator draws on that one bucket. It owns no goroutines.
func (l *Limiter) Middleware() func(http.Handler) http.Handler {
	// rate is immutable after construction, so the hint is computed once per
	// decorator rather than per request. rate > 0 is guaranteed by NewLimiter,
	// so the quotient is finite and the ceiling is at least 1.
	retryAfter := strconv.Itoa(int(math.Ceil(1 / l.rate)))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow() {
				w.Header().Set(headerRetryAfter, retryAfter)
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// headerRetryAfter is the RFC 9110 §10.2.3 response header naming how long a
// client should wait before retrying.
const headerRetryAfter = "Retry-After"
