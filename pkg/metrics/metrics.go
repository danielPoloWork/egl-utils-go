// Package metrics instruments HTTP handlers and exposes what it records in
// Prometheus text exposition format.
//
// A Recorder owns the counters; its Middleware decorates a handler and its
// Handler serves the scrape endpoint. Both come from the same Recorder, so the
// endpoint always exposes exactly what the middleware records:
//
//	rec := metrics.New()
//	mux.Handle("/metrics", rec.Handler())
//	h := rec.Middleware()(appHandler)
//
// Two families are recorded: a http_requests_total counter and a
// http_request_duration_seconds histogram over the standard latency buckets,
// each labelled by request method and response status code.
//
// Label cardinality is bounded on purpose: the request path is never a label (it
// is unbounded and would explode Prometheus memory), and the method label is
// normalized to the known HTTP methods plus "other", so a client sending
// arbitrary method tokens cannot inflate cardinality. Since the method label has
// ten values and net/http admits only status codes 100–999, a Recorder holds at
// most 9 000 series whatever traffic it is shown.
//
// No Prometheus client library is involved: the exposition text is written
// directly, so this package adds no dependency to a consumer. What that costs is
// worth knowing — the endpoint serves these two families and nothing else, with
// no go_* runtime or process_* collectors and no Accept negotiation. A consumer
// wanting those imports a Prometheus client library itself and mounts its
// handler at a second path, which puts that dependency where it is actually
// wanted. Design decisions are recorded in ADR-0027 and ADR-0050.
package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// knownMethods bounds the "method" label: anything not here becomes "other", so
// an attacker cannot drive unbounded cardinality with junk method tokens. It
// maps each method to the package's own constant rather than reporting the
// caller's string back, so a label value never holds a reference into a request
// buffer.
var knownMethods = map[string]string{
	http.MethodGet:     http.MethodGet,
	http.MethodHead:    http.MethodHead,
	http.MethodPost:    http.MethodPost,
	http.MethodPut:     http.MethodPut,
	http.MethodPatch:   http.MethodPatch,
	http.MethodDelete:  http.MethodDelete,
	http.MethodConnect: http.MethodConnect,
	http.MethodOptions: http.MethodOptions,
	http.MethodTrace:   http.MethodTrace,
}

func normalizeMethod(m string) string {
	if canonical, ok := knownMethods[m]; ok {
		return canonical
	}
	return "other"
}

// Recorder records HTTP request metrics and exposes them in Prometheus text
// exposition format. All methods are safe for concurrent use. The zero value is
// not usable; construct a Recorder with New.
type Recorder struct {
	// mu guards the series map only — finding or creating a series. Recording
	// into one holds no lock, because every counter inside it is an atomic.
	mu     sync.RWMutex
	series map[seriesKey]*series
}

// New builds a Recorder. Each Recorder is independent: two of them in one
// process record and expose separately, so a double install is not something to
// guard against.
func New() *Recorder {
	return &Recorder{series: make(map[seriesKey]*series)}
}

// Middleware returns middleware that records, for every request, the request
// counter and the latency histogram labelled by (method, code). It panics if it
// is given a nil handler — a wiring error, caught at setup (ADR-0005 idiom).
//
// The returned decorator may be applied to any number of handlers; they all
// record into this Recorder.
func (r *Recorder) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("metrics: nil handler")
		}
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, req)
			r.observe(normalizeMethod(req.Method), rec.status, time.Since(start).Seconds())
		})
	}
}

// Handler returns the exposition endpoint for this Recorder. It answers every
// request with the current metrics in text format; a Recorder that has recorded
// nothing serves an empty body, which is a valid scrape.
func (r *Recorder) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Render into a buffer first: the exposition is built from a snapshot
		// under no lock, and the write to a possibly slow client then happens
		// with nothing held.
		body := r.appendExposition(make([]byte, 0, 1024))
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	})
}

// observe records one request. The fast path — a series that already exists —
// takes only a read lock and allocates nothing.
func (r *Recorder) observe(method string, code int, seconds float64) {
	key := seriesKey{method: method, code: code}

	r.mu.RLock()
	s := r.series[key]
	r.mu.RUnlock()

	if s == nil {
		s = r.newSeries(key)
	}
	s.observe(seconds)
}

// newSeries creates the series for key, or returns the one another goroutine
// created first. The status code is rendered to a string exactly here, once per
// series, rather than on every request.
func (r *Recorder) newSeries(key seriesKey) *series {
	r.mu.Lock()
	defer r.mu.Unlock()

	if s, ok := r.series[key]; ok {
		return s // lost the race; the winner's series is the live one
	}
	s := &series{method: key.method, code: strconv.Itoa(key.code)}
	r.series[key] = s
	return s
}

// statusWriter captures the response status code (defaulting to 200) without
// altering what reaches the client, and exposes Unwrap so
// http.ResponseController still reaches the underlying writer.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
