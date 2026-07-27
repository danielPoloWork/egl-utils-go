package middleware_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware"
)

// NFR-01 (spec v2 §5): the middleware chain RequestID + Recoverer + Cors adds
// ≤ 1 µs and 0 allocs/op per request on the non-logging path; middleware.Logger
// adds ≤ 3 allocs/op.
//
// The chain is built once per benchmark, as a consumer builds it once at wiring
// time; only the per-request cost is measured. The response writer is a
// discarding one rather than httptest.NewRecorder, whose own bookkeeping would
// otherwise be attributed to the middleware (a mistake caught while writing the
// ratelimit benchmarks in 10.4).

// nullWriter discards everything, so the benchmark measures the chain and not a
// recorder.
type nullWriter struct{ h http.Header }

func (n nullWriter) Header() http.Header       { return n.h }
func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }
func (nullWriter) WriteHeader(int)             {}

func noopHandler() http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
}

// corsForBench is a realistic single-origin policy: the shape a service actually
// deploys, not the zero value (which would short-circuit before doing work).
func corsForBench() middleware.CorsConfig {
	return middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
		AllowedHeaders: []string{"Content-Type"},
	}
}

// benchRequest returns the request the chain is measured against: a plain GET
// carrying a valid inbound request ID and an allowed Origin, so every middleware
// takes its normal path rather than an early return.
func benchRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/orders/42", nil)
	r.Header.Set("X-Request-ID", "0123456789abcdef0123456789abcdef")
	r.Header.Set("Origin", "https://app.example.com")
	return r
}

// BenchmarkNFR01Chain is the NFR-01 headline: the full non-logging chain.
func BenchmarkNFR01Chain(b *testing.B) {
	h := middleware.RequestID(middleware.Recoverer(
		middleware.Cors(corsForBench())(noopHandler()),
	))
	w := nullWriter{h: make(http.Header)}
	r := benchRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

// The per-middleware breakdowns exist so a regression in the chain can be
// attributed without bisecting it by hand.

func BenchmarkNFR01RequestIDOnly(b *testing.B) {
	h := middleware.RequestID(noopHandler())
	w := nullWriter{h: make(http.Header)}
	r := benchRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

// BenchmarkNFR01RequestIDGenerated is the path where no usable inbound ID is
// present, so one is minted with crypto/rand. It is reported separately because
// generation is inherently allocating and would otherwise mask the adopt path.
func BenchmarkNFR01RequestIDGenerated(b *testing.B) {
	h := middleware.RequestID(noopHandler())
	w := nullWriter{h: make(http.Header)}
	r := httptest.NewRequest(http.MethodGet, "/orders/42", nil) // no X-Request-ID

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkNFR01RecovererOnly(b *testing.B) {
	h := middleware.Recoverer(noopHandler())
	w := nullWriter{h: make(http.Header)}
	r := benchRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkNFR01CorsOnly(b *testing.B) {
	h := middleware.Cors(corsForBench())(noopHandler())
	w := nullWriter{h: make(http.Header)}
	r := benchRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

// BenchmarkNFR01CorsPreflight measures the terminal OPTIONS path, which never
// reaches the wrapped handler.
func BenchmarkNFR01CorsPreflight(b *testing.B) {
	h := middleware.Cors(corsForBench())(noopHandler())
	w := nullWriter{h: make(http.Header)}
	r := httptest.NewRequest(http.MethodOptions, "/orders/42", nil)
	r.Header.Set("Origin", "https://app.example.com")
	r.Header.Set("Access-Control-Request-Method", http.MethodPost)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

// BenchmarkNFR01Logger is the second half of NFR-01: Logger's own per-request
// cost, measured against a discarding slog handler so the sink is not the
// bottleneck being reported.
func BenchmarkNFR01Logger(b *testing.B) {
	l := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := middleware.Logger(l)(noopHandler())
	w := nullWriter{h: make(http.Header)}
	r := benchRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

// BenchmarkNFR01ChainWithLogger is the whole realistic stack — the number a
// consumer actually pays.
func BenchmarkNFR01ChainWithLogger(b *testing.B) {
	l := slog.New(slog.NewJSONHandler(io.Discard, nil))
	h := middleware.RequestID(middleware.Logger(l)(middleware.Recoverer(
		middleware.Cors(corsForBench())(noopHandler()),
	)))
	w := nullWriter{h: make(http.Header)}
	r := benchRequest()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}
