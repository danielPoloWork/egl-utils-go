package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Benchmarks for the hand-written recording and exposition paths (ADR-0050).
// The comparison against the client_golang implementation they replaced is in
// docs/benchmarks/2026-07-29-metrics-without-the-sdk.md; it was measured with
// both implementations compiled into one binary, which is the only sound way to
// compare on this hardware (ADR-0037), and cannot be reproduced here because the
// dependency is gone.

// sink keeps rendered exposition reachable so the compiler cannot delete the work
// being measured — a discarded result benchmarks as zero allocations.
var sink []byte

func BenchmarkObserve(b *testing.B) {
	r := New()
	b.ReportAllocs()
	for range b.N {
		r.observe(http.MethodGet, http.StatusOK, 0.023)
	}
}

// BenchmarkObserveParallel is the contended shape: every goroutine records the
// same label pair, so all of them hit one series' atomics. This is what a real
// server looks like, and it is where the lock-free design earns its keep — a
// per-series mutex would serialise here.
func BenchmarkObserveParallel(b *testing.B) {
	r := New()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.observe(http.MethodGet, http.StatusOK, 0.023)
		}
	})
}

// BenchmarkObserveDistinctSeries spreads across label pairs, so the map lookup
// varies and series creation is amortised rather than free.
func BenchmarkObserveDistinctSeries(b *testing.B) {
	codes := []int{200, 201, 301, 400, 404, 418, 500, 503}
	r := New()
	b.ReportAllocs()
	for i := range b.N {
		r.observe(http.MethodGet, codes[i%len(codes)], 0.023)
	}
}

// BenchmarkExposition measures a scrape over nine series — three methods by three
// status codes, so 108 bucket lines plus sums, counts and the counter family.
func BenchmarkExposition(b *testing.B) {
	r := New()
	for _, m := range []string{http.MethodGet, http.MethodPost, "other"} {
		for _, c := range []int{200, 404, 500} {
			r.observe(m, c, 0.023)
		}
	}
	buf := make([]byte, 0, 16384)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		sink = r.appendExposition(buf[:0])
	}
}

// BenchmarkMiddleware measures the decorator end to end against a discarding
// writer. httptest.NewRecorder is deliberately not used per iteration: its
// allocations would be attributed to the middleware and would swamp what is
// being measured.
func BenchmarkMiddleware(b *testing.B) {
	h := New().Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := discardWriter{header: make(http.Header)}

	b.ReportAllocs()
	for range b.N {
		h.ServeHTTP(w, req)
	}
}

type discardWriter struct{ header http.Header }

func (d discardWriter) Header() http.Header       { return d.header }
func (discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (discardWriter) WriteHeader(int)             {}
