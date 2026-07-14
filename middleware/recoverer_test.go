package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/middleware"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// withCapturedDefault swaps slog's process-wide default logger for a capturing
// one for the duration of the test (Recoverer logs to slog.Default), restoring
// the original afterwards. Tests using it must not run in parallel — the
// default logger is global.
func withCapturedDefault(t *testing.T) *[]slog.Record {
	t.Helper()
	logger, records := newCapturingLogger()
	orig := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(orig) })
	return records
}

func TestRecovererNilHandlerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Panics(t, func() { middleware.Recoverer(nil) })
}

func TestRecovererRecoversAndWrites500(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	require.NotPanics(t, func() {
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, http.StatusText(http.StatusInternalServerError))
	require.NotContains(t, body, "boom", "the panic value must never reach the client")
	require.NotContains(t, body, "goroutine", "the stack trace must never reach the client")
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Len(t, *records, 1, "the panic is logged server-side")
}

func TestRecovererLogsPanicServerSide(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/things", nil))

	require.Len(t, *records, 1)
	r := (*records)[0]
	require.Equal(t, "http handler panic", r.Message)
	require.Equal(t, slog.LevelError, r.Level)
	a := attrsOf(r)
	require.Equal(t, http.MethodPost, a["method"].String())
	require.Equal(t, "/things", a["path"].String())
	require.Contains(t, a["panic"].String(), "kaboom")
	require.Contains(t, a["stack"].String(), "goroutine", "a stack trace is captured server-side")
}

func TestRecovererAttachesRequestID(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	// RequestID runs first (seeds the context), Recoverer sees the panic.
	chain := middleware.RequestID(middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Len(t, *records, 1)
	id, ok := attrsOf((*records)[0])["request_id"]
	require.True(t, ok, "request_id is logged when the chain seeded one")
	require.Equal(t, rec.Header().Get(middleware.HeaderName), id.String())
}

func TestRecovererReraisesErrAbortHandler(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	require.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})
	require.Empty(t, *records, "ErrAbortHandler is net/http's silent-abort sentinel: re-panicked, not logged")
}

func TestRecovererPassesThroughWithoutPanic(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	h := middleware.Recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
	require.Empty(t, *records, "no panic, nothing logged")
}

func TestRecovererLeavesCommittedResponseUntouched(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	h := middleware.Recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("boom after commit")
	}))
	rec := httptest.NewRecorder()
	require.NotPanics(t, func() {
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	})
	require.Equal(t, http.StatusOK, rec.Code, "a committed status is not overwritten")
	require.Equal(t, "partial", rec.Body.String(), "the committed body is preserved")
	require.Len(t, *records, 1, "the panic is still logged")
}

func TestRecovererOmitsQueryStringFromLog(t *testing.T) {
	defer goleak.VerifyNone(t)
	records := withCapturedDefault(t)
	h := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/search?token=secret", nil))
	require.Len(t, *records, 1)
	path := attrsOf((*records)[0])["path"].String()
	require.Equal(t, "/search", path)
	require.NotContains(t, path, "secret", "the query string must never be logged")
}

func TestRecovererPreservesFlusherThroughUnwrap(t *testing.T) {
	defer goleak.VerifyNone(t)
	flushed := false
	h := middleware.Recoverer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// httptest.ResponseRecorder implements http.Flusher; the controller must
		// reach it through responseRecorder.Unwrap.
		require.NoError(t, http.NewResponseController(w).Flush())
		flushed = true
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.True(t, flushed)
	require.True(t, rec.Flushed, "Flush must reach the underlying ResponseWriter")
}
