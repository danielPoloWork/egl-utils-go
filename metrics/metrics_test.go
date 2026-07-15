package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// counterValue returns the value of a (method, code)-labelled sample of the
// named counter in reg, or -1 if that series is absent.
func counterValue(t *testing.T, reg *prometheus.Registry, name, method, code string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m, method, code) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return -1
}

// histogramSampleCount returns the observation count of a (method, code)-labelled
// series of the named histogram, or -1 if absent.
func histogramSampleCount(t *testing.T, reg *prometheus.Registry, name, method, code string) uint64 {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m, method, code) {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}

func labelsMatch(m *dto.Metric, method, code string) bool {
	var gotMethod, gotCode string
	for _, l := range m.GetLabel() {
		switch l.GetName() {
		case "method":
			gotMethod = l.GetValue()
		case "code":
			gotCode = l.GetValue()
		}
	}
	return gotMethod == method && gotCode == code
}

func newHandler(reg *prometheus.Registry, status int) http.Handler {
	return metrics.Prometheus(reg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
}

func TestRecordsCountAndDuration(t *testing.T) {
	defer goleak.VerifyNone(t)
	reg := prometheus.NewRegistry()
	h := newHandler(reg, http.StatusOK)

	for range 3 {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	}
	require.Equal(t, float64(3), counterValue(t, reg, "http_requests_total", "GET", "200"))
	require.Equal(t, uint64(3), histogramSampleCount(t, reg, "http_request_duration_seconds", "GET", "200"))
}

func TestLabelsByMethodAndStatus(t *testing.T) {
	defer goleak.VerifyNone(t)
	reg := prometheus.NewRegistry()
	mw := metrics.Prometheus(reg)
	get200 := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})) // implicit 200
	post404 := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))

	get200.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	post404.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))

	require.Equal(t, float64(1), counterValue(t, reg, "http_requests_total", "GET", "200"))
	require.Equal(t, float64(1), counterValue(t, reg, "http_requests_total", "POST", "404"))
}

func TestUnknownMethodBecomesOther(t *testing.T) {
	defer goleak.VerifyNone(t)
	reg := prometheus.NewRegistry()
	h := newHandler(reg, http.StatusOK)
	req := httptest.NewRequest("WEIRDMETHOD", "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	require.Equal(t, float64(1), counterValue(t, reg, "http_requests_total", "other", "200"),
		"an unknown method is bucketed as \"other\" to bound cardinality")
	require.Equal(t, float64(-1), counterValue(t, reg, "http_requests_total", "WEIRDMETHOD", "200"),
		"the raw method token must never become a label")
}

func TestResponseWritesThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	reg := prometheus.NewRegistry()
	h := metrics.Prometheus(reg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("brew"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, "brew", rec.Body.String())
	require.Equal(t, float64(1), counterValue(t, reg, "http_requests_total", "GET", "418"))
}

func TestPreservesFlusherThroughUnwrap(t *testing.T) {
	defer goleak.VerifyNone(t)
	reg := prometheus.NewRegistry()
	flushed := false
	h := metrics.Prometheus(reg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// httptest.ResponseRecorder implements http.Flusher; the controller must
		// reach it through statusRecorder.Unwrap.
		require.NoError(t, http.NewResponseController(w).Flush())
		flushed = true
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.True(t, flushed)
	require.True(t, rec.Flushed, "Flush must reach the underlying ResponseWriter")
}

func TestHandlerExposesMetrics(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "# HELP", "the exposition endpoint returns Prometheus text")
}

func TestNilRegistererPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "metrics: nil registerer", func() { metrics.Prometheus(nil) })
}

func TestNilHandlerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "metrics: nil handler", func() {
		metrics.Prometheus(prometheus.NewRegistry())(nil)
	})
}

func TestDoubleRegisterPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	reg := prometheus.NewRegistry()
	metrics.Prometheus(reg)
	require.Panics(t, func() { metrics.Prometheus(reg) }, "re-registering on the same registry is a wiring error")
}
