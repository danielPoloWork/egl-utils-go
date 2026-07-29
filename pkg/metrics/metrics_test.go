package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/metrics"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// scrape returns the recorder's exposition text through its own endpoint, which
// is how a consumer reads these metrics — so the assertions below exercise the
// public path rather than package internals.
func scrape(t *testing.T, rec *metrics.Recorder) string {
	t.Helper()
	w := httptest.NewRecorder()
	rec.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)
	return w.Body.String()
}

// sampleValue returns the value of one exposition line, or -1 if that series is
// absent. Absent and zero are deliberately distinguishable: a label value that
// must never appear (a raw method token) has to be provably missing, not merely
// zero.
func sampleValue(t *testing.T, body, name, method, code string) float64 {
	t.Helper()
	prefix := name + `{code="` + code + `",method="` + method + `"} `
	for line := range strings.SplitSeq(body, "\n") {
		if after, ok := strings.CutPrefix(line, prefix); ok {
			v, err := strconv.ParseFloat(after, 64)
			require.NoError(t, err)
			return v
		}
	}
	return -1
}

func serve(h http.Handler, method string) {
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/x", nil))
}

func newHandler(rec *metrics.Recorder, status int) http.Handler {
	return rec.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
}

func TestRecordsCountAndDuration(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := metrics.New()
	h := newHandler(rec, http.StatusOK)

	for range 3 {
		serve(h, http.MethodGet)
	}

	body := scrape(t, rec)
	require.Equal(t, float64(3), sampleValue(t, body, "http_requests_total", "GET", "200"))
	require.Equal(t, float64(3), sampleValue(t, body, "http_request_duration_seconds_count", "GET", "200"))
	require.Contains(t, body, `http_request_duration_seconds_bucket{code="200",method="GET",le="+Inf"} 3`)
}

func TestLabelsByMethodAndStatus(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := metrics.New()
	mw := rec.Middleware()
	get200 := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})) // implicit 200
	post404 := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))

	serve(get200, http.MethodGet)
	serve(post404, http.MethodPost)

	body := scrape(t, rec)
	require.Equal(t, float64(1), sampleValue(t, body, "http_requests_total", "GET", "200"))
	require.Equal(t, float64(1), sampleValue(t, body, "http_requests_total", "POST", "404"))
}

func TestUnknownMethodBecomesOther(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := metrics.New()
	serve(newHandler(rec, http.StatusOK), "WEIRDMETHOD")

	body := scrape(t, rec)
	require.Equal(t, float64(1), sampleValue(t, body, "http_requests_total", "other", "200"),
		"an unknown method is bucketed as \"other\" to bound cardinality")
	require.Equal(t, float64(-1), sampleValue(t, body, "http_requests_total", "WEIRDMETHOD", "200"),
		"the raw method token must never become a label")
	require.NotContains(t, body, "WEIRDMETHOD")
}

func TestResponseWritesThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := metrics.New()
	h := rec.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("brew"))
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusTeapot, w.Code)
	require.Equal(t, "brew", w.Body.String())
	require.Equal(t, float64(1), sampleValue(t, scrape(t, rec), "http_requests_total", "GET", "418"))
}

func TestPreservesFlusherThroughUnwrap(t *testing.T) {
	defer goleak.VerifyNone(t)
	flushed := false
	h := metrics.New().Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// httptest.ResponseRecorder implements http.Flusher; the controller must
		// reach it through statusWriter.Unwrap.
		require.NoError(t, http.NewResponseController(w).Flush())
		flushed = true
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.True(t, flushed)
	require.True(t, w.Flushed, "Flush must reach the underlying ResponseWriter")
}

func TestHandlerExposesTextFormat(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := metrics.New()
	serve(newHandler(rec, http.StatusOK), http.MethodGet)

	w := httptest.NewRecorder()
	rec.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/plain; version=0.0.4; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, strconv.Itoa(w.Body.Len()), w.Header().Get("Content-Length"))
	require.Contains(t, w.Body.String(), "# HELP http_requests_total")
	require.Contains(t, w.Body.String(), "# TYPE http_request_duration_seconds histogram")
}

// TestFreshRecorderServesAnEmptyScrape is the public face of the family-omission
// rule: before any traffic there is nothing to report, and reporting zeros would
// claim observations that never happened.
func TestFreshRecorderServesAnEmptyScrape(t *testing.T) {
	defer goleak.VerifyNone(t)
	w := httptest.NewRecorder()
	metrics.New().Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Body.String())
}

// TestRecordersAreIndependent replaces v1's double-register panic test. Two
// installs on one registry used to be a wiring error worth panicking over; a
// Recorder owns its own series, so the same wiring is simply two independent
// recorders — the failure mode is gone rather than guarded.
func TestRecordersAreIndependent(t *testing.T) {
	defer goleak.VerifyNone(t)
	first, second := metrics.New(), metrics.New()

	serve(newHandler(first, http.StatusOK), http.MethodGet)
	serve(newHandler(second, http.StatusOK), http.MethodPost)

	firstBody, secondBody := scrape(t, first), scrape(t, second)
	require.Equal(t, float64(1), sampleValue(t, firstBody, "http_requests_total", "GET", "200"))
	require.Equal(t, float64(-1), sampleValue(t, firstBody, "http_requests_total", "POST", "200"))
	require.Equal(t, float64(1), sampleValue(t, secondBody, "http_requests_total", "POST", "200"))
	require.Equal(t, float64(-1), sampleValue(t, secondBody, "http_requests_total", "GET", "200"))
}

// TestOneMiddlewareInstrumentsManyHandlers pins that the decorator is reusable:
// every handler it wraps records into the one Recorder that produced it.
func TestOneMiddlewareInstrumentsManyHandlers(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec := metrics.New()
	mw := rec.Middleware()
	for range 2 {
		serve(mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})), http.MethodGet)
	}
	require.Equal(t, float64(2), sampleValue(t, scrape(t, rec), "http_requests_total", "GET", "200"))
}

func TestNilHandlerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "metrics: nil handler", func() {
		metrics.New().Middleware()(nil)
	})
}
