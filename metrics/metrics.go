// Package metrics provides Prometheus instrumentation for HTTP handlers.
//
// Prometheus returns middleware that records a request counter and a latency
// histogram, each labelled by request method and response status code, into a
// caller-supplied registry. Handler returns the exposition endpoint for the
// default registry. A typical wiring registers on the default registry and
// exposes it:
//
//	mux.Handle("/metrics", metrics.Handler())
//	h := metrics.Prometheus(prometheus.DefaultRegisterer)(appHandler)
//
// Label cardinality is bounded on purpose: the request path is never a label
// (it is unbounded and would explode Prometheus memory), and the method label
// is normalized to the known HTTP methods plus "other", so a client sending
// arbitrary method tokens cannot inflate cardinality.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// knownMethods bounds the "method" label: anything not here becomes "other",
// so an attacker cannot drive unbounded cardinality with junk method tokens.
var knownMethods = map[string]struct{}{
	http.MethodGet: {}, http.MethodHead: {}, http.MethodPost: {},
	http.MethodPut: {}, http.MethodPatch: {}, http.MethodDelete: {},
	http.MethodConnect: {}, http.MethodOptions: {}, http.MethodTrace: {},
}

func normalizeMethod(m string) string {
	if _, ok := knownMethods[m]; ok {
		return m
	}
	return "other"
}

// Prometheus returns middleware that records, for every request, a
// http_requests_total counter and a http_request_duration_seconds histogram
// labelled by (method, code), registering both on reg. It panics if reg is
// nil, if the returned decorator is given a nil handler, or if reg already has
// these collectors registered (a double-install wiring error) — all caught at
// setup (ADR-0005 idiom).
func Prometheus(reg prometheus.Registerer) func(http.Handler) http.Handler {
	if reg == nil {
		panic("metrics: nil registerer")
	}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests, by method and response status code.",
	}, []string{"method", "code"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and response status code.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "code"})
	reg.MustRegister(requests, duration)

	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("metrics: nil handler")
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)

			method := normalizeMethod(r.Method)
			code := strconv.Itoa(rec.status)
			requests.WithLabelValues(method, code).Inc()
			duration.WithLabelValues(method, code).Observe(time.Since(start).Seconds())
		})
	}
}

// Handler returns the Prometheus exposition endpoint for the default registry
// (equivalent to promhttp.Handler()). Pair it with
// Prometheus(prometheus.DefaultRegisterer); for a custom registry, expose it
// with promhttp.HandlerFor directly.
func Handler() http.Handler { return promhttp.Handler() }

// statusRecorder captures the response status code (defaulting to 200) without
// altering what reaches the client, and exposes Unwrap so
// http.ResponseController still reaches the underlying writer.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
