package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestMiddlewareReadmitsAfterRefill drives the refill deterministically on the
// fake clock: the wall clock never advances, so a re-admission can only come
// from the token the elapsed fake time funded.
func TestMiddlewareReadmitsAfterRefill(t *testing.T) {
	defer goleak.VerifyNone(t)
	clock := newFakeClock()
	l := newTestLimiter(1, 1, clock) // one token per second, burst of one
	calls := 0
	h := l.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))

	do := func() int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		return rec.Code
	}

	require.Equal(t, http.StatusOK, do(), "the burst token admits the first request")
	require.Equal(t, http.StatusTooManyRequests, do(), "the bucket is empty at the same instant")

	clock.advance(999 * time.Millisecond)
	require.Equal(t, http.StatusTooManyRequests, do(),
		"one millisecond short of a full token is still a refusal")

	clock.advance(time.Millisecond)
	require.Equal(t, http.StatusOK, do(), "the refilled token admits again")
	require.Equal(t, 2, calls, "exactly the two admitted requests reached the handler")
}
