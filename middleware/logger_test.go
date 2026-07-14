package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/middleware"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// capturingHandler is a slog.Handler that records every emitted record for
// assertion. It enables all levels so status-derived leveling is observable.
type capturingHandler struct {
	mu      *sync.Mutex
	records *[]slog.Record
}

func newCapturingLogger() (*slog.Logger, *[]slog.Record) {
	records := &[]slog.Record{}
	h := capturingHandler{mu: &sync.Mutex{}, records: records}
	return slog.New(h), records
}

func (h capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, r.Clone())
	return nil
}

func (h capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h capturingHandler) WithGroup(string) slog.Handler      { return h }

// attrsOf flattens a record's attributes into a name→value map.
func attrsOf(r slog.Record) map[string]slog.Value {
	m := make(map[string]slog.Value, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value
		return true
	})
	return m
}

// runLogger drives one request through Logger(handler) and returns the sole
// captured record and the response recorder.
func runLogger(t *testing.T, handler http.HandlerFunc, req *http.Request) (slog.Record, *httptest.ResponseRecorder) {
	t.Helper()
	logger, records := newCapturingLogger()
	h := middleware.Logger(logger)(handler)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Len(t, *records, 1, "exactly one line per request")
	return (*records)[0], rec
}

func TestLoggerNilLoggerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Panics(t, func() { middleware.Logger(nil) })
}

func TestLoggerNilHandlerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger, _ := newCapturingLogger()
	require.Panics(t, func() { middleware.Logger(logger)(nil) })
}

func TestLoggerRecordsCoreFields(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := runLogger(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}, httptest.NewRequest(http.MethodPost, "/things", nil))

	a := attrsOf(rec)
	require.Equal(t, "http request", rec.Message)
	require.Equal(t, slog.LevelInfo, rec.Level)
	require.Equal(t, http.MethodPost, a["method"].String())
	require.Equal(t, "/things", a["path"].String())
	require.Equal(t, int64(http.StatusCreated), a["status"].Int64())
	require.Equal(t, int64(5), a["bytes"].Int64())
	require.GreaterOrEqual(t, a["duration"].Duration(), int64(0))
}

func TestLoggerDefaultsStatusTo200(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("body without WriteHeader", func(t *testing.T) {
		rec, _ := runLogger(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("body"))
		}, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, int64(http.StatusOK), attrsOf(rec)["status"].Int64())
		require.Equal(t, int64(4), attrsOf(rec)["bytes"].Int64())
	})
	t.Run("no write at all", func(t *testing.T) {
		rec, _ := runLogger(t, func(http.ResponseWriter, *http.Request) {},
			httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, int64(http.StatusOK), attrsOf(rec)["status"].Int64())
		require.Equal(t, int64(0), attrsOf(rec)["bytes"].Int64())
	})
}

func TestLoggerLevelFollowsStatus(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := []struct {
		status int
		want   slog.Level
	}{
		{http.StatusOK, slog.LevelInfo},
		{http.StatusNotFound, slog.LevelWarn},
		{http.StatusInternalServerError, slog.LevelError},
	}
	for _, tc := range cases {
		rec, _ := runLogger(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equalf(t, tc.want, rec.Level, "status %d", tc.status)
		require.Equal(t, int64(tc.status), attrsOf(rec)["status"].Int64())
	}
}

func TestLoggerOmitsQueryString(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := runLogger(t, func(http.ResponseWriter, *http.Request) {},
		httptest.NewRequest(http.MethodGet, "/search?token=secret&q=x", nil))
	path := attrsOf(rec)["path"].String()
	require.Equal(t, "/search", path)
	require.NotContains(t, path, "secret", "the query string must never be logged")
}

func TestLoggerAttachesRequestIDWhenPresent(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger, records := newCapturingLogger()
	var chain http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	chain = middleware.Logger(logger)(chain)
	chain = middleware.RequestID(chain) // RequestID runs first, seeding the context

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Len(t, *records, 1)
	a := attrsOf((*records)[0])
	got, ok := a["request_id"]
	require.True(t, ok, "request_id must be logged when the context carries one")
	require.Equal(t, rec.Header().Get(middleware.HeaderName), got.String())
}

func TestLoggerOmitsRequestIDWhenAbsent(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := runLogger(t, func(http.ResponseWriter, *http.Request) {},
		httptest.NewRequest(http.MethodGet, "/", nil))
	_, ok := attrsOf(rec)["request_id"]
	require.False(t, ok, "no request_id attr when the context has none")
}

func TestLoggerWritesResponseThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	_, resp := runLogger(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("brew"))
	}, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusTeapot, resp.Code)
	require.Equal(t, "brew", resp.Body.String())
}

func TestLoggerPreservesFlusherThroughUnwrap(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger, _ := newCapturingLogger()
	flushed := false
	h := middleware.Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// httptest.ResponseRecorder implements http.Flusher; the controller
		// must reach it through responseRecorder.Unwrap.
		require.NoError(t, http.NewResponseController(w).Flush())
		flushed = true
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.True(t, flushed)
	require.True(t, rec.Flushed, "Flush must reach the underlying ResponseWriter")
}

func TestLoggerLogsThenPropagatesPanic(t *testing.T) {
	defer goleak.VerifyNone(t)
	logger, records := newCapturingLogger()
	h := middleware.Logger(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	require.PanicsWithValue(t, "boom", func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})
	require.Len(t, *records, 1, "a panicking request must still be logged (deferred)")
}
