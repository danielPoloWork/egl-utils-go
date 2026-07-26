package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/ratelimit"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// okHandler is the protected handler: it records that it ran and answers 200.
func okHandler(calls *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*calls++
		w.WriteHeader(http.StatusOK)
	})
}

// serve runs one GET / through h and returns the recorded response.
func serve(h http.Handler) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec
}

func TestMiddlewareAdmitsWithinBurst(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := 0
	// A rate low enough that no token can refill during the test: every
	// admission comes out of the burst, so the count is exact.
	h := ratelimit.NewLimiter(0.001, 3).Middleware()(okHandler(&calls))

	for i := range 3 {
		rec := serve(h)
		require.Equal(t, http.StatusOK, rec.Code, "request %d is within the burst", i)
	}
	require.Equal(t, 3, calls, "every admitted request reaches the wrapped handler")
}

func TestMiddlewareRefusesWhenBucketIsEmpty(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := 0
	h := ratelimit.NewLimiter(0.001, 1).Middleware()(okHandler(&calls))

	require.Equal(t, http.StatusOK, serve(h).Code, "the burst token admits the first request")

	rec := serve(h)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, 1, calls, "a refused request never reaches the wrapped handler")
	require.Equal(t, "Too Many Requests\n", rec.Body.String(),
		"the body is the generic status text — nothing about the limiter is disclosed")
}

func TestMiddlewareSetsRetryAfter(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := []struct {
		name string
		rate float64
		want string
	}{
		{"one per second", 1, "1"},
		{"faster than a second is still rounded up to one", 100, "1"},
		{"one every two seconds", 0.5, "2"},
		{"fractional waits round up", 0.3, "4"}, // 1/0.3 = 3.33... -> 4
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := ratelimit.NewLimiter(tc.rate, 1).Middleware()(okHandler(new(int)))
			require.Equal(t, http.StatusOK, serve(h).Code)

			rec := serve(h)
			require.Equal(t, http.StatusTooManyRequests, rec.Code)
			require.Equal(t, tc.want, rec.Header().Get("Retry-After"),
				"Retry-After is ceil(1/rate) whole seconds")
		})
	}
}

func TestMiddlewareOmitsRetryAfterWhenAdmitting(t *testing.T) {
	defer goleak.VerifyNone(t)
	h := ratelimit.NewLimiter(1, 1).Middleware()(okHandler(new(int)))
	rec := serve(h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Retry-After"), "an admitted request carries no retry hint")
}

func TestMiddlewareDecoratorsShareOneBucket(t *testing.T) {
	defer goleak.VerifyNone(t)
	l := ratelimit.NewLimiter(0.001, 1)
	calls := 0
	first := l.Middleware()(okHandler(&calls))
	second := l.Middleware()(okHandler(&calls))

	require.Equal(t, http.StatusOK, serve(first).Code)
	require.Equal(t, http.StatusTooManyRequests, serve(second).Code,
		"a second decorator over the same limiter draws on the same bucket, not a fresh one")
	require.Equal(t, 1, calls)
}

func TestMiddlewarePassesRequestAndWriterThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	var gotPath string
	h := ratelimit.NewLimiter(1, 1).Middleware()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("X-From-Handler", "yes")
			w.WriteHeader(http.StatusTeapot)
		},
	))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/deep/path", nil))

	require.Equal(t, "/deep/path", gotPath, "the request reaches the handler unaltered")
	require.Equal(t, http.StatusTeapot, rec.Code, "the handler owns the response on the admit path")
	require.Equal(t, "yes", rec.Header().Get("X-From-Handler"))
}

func TestErrLimitedIsAStableSentinel(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.EqualError(t, ratelimit.ErrLimited, "ratelimit: rate limit exceeded")
}

// TestMiddlewareAdmitPathDoesNotAllocate guards the NFR-01 posture: the
// decorator must add nothing to the steady-state request path.
func TestMiddlewareAdmitPathDoesNotAllocate(t *testing.T) {
	defer goleak.VerifyNone(t)
	// A large burst and a high rate keep every iteration on the admit path.
	h := ratelimit.NewLimiter(1e9, 1_000_000).Middleware()(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	allocs := testing.AllocsPerRun(100, func() { h.ServeHTTP(rec, req) })
	require.Zero(t, allocs, "the admit path allocates nothing per request")
}

func BenchmarkMiddlewareAdmit(b *testing.B) {
	h := ratelimit.NewLimiter(1e9, 1_000_000).Middleware()(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(rec, req)
	}
}

// nullWriter is a minimal http.ResponseWriter that discards everything, so a
// benchmark measures the middleware rather than a recorder's bookkeeping.
type nullWriter struct{ h http.Header }

func (n nullWriter) Header() http.Header       { return n.h }
func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }
func (nullWriter) WriteHeader(int)             {}

func BenchmarkMiddlewareRefuse(b *testing.B) {
	l := ratelimit.NewLimiter(0.001, 1)
	require.True(b, l.Allow()) // drain the single burst token
	h := l.Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := nullWriter{h: make(http.Header)}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, req)
	}
}
