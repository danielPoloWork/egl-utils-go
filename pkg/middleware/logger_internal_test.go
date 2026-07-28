package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestLevelFor(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := []struct {
		status int
		want   slog.Level
	}{
		{http.StatusContinue, slog.LevelInfo},
		{http.StatusOK, slog.LevelInfo},
		{http.StatusMovedPermanently, slog.LevelInfo},
		{http.StatusBadRequest, slog.LevelWarn},
		{http.StatusNotFound, slog.LevelWarn},
		{499, slog.LevelWarn},
		{http.StatusInternalServerError, slog.LevelError},
		{http.StatusBadGateway, slog.LevelError},
	}
	for _, tc := range cases {
		require.Equalf(t, tc.want, levelFor(tc.status), "status %d", tc.status)
	}
}

func TestResponseRecorderDefaultsAndCounts(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("status defaults to 200 before any write", func(t *testing.T) {
		rec := &responseRecorder{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
		require.Equal(t, http.StatusOK, rec.status)
		require.False(t, rec.wroteHeader)
	})

	t.Run("WriteHeader records the first code only", func(t *testing.T) {
		under := httptest.NewRecorder()
		rec := &responseRecorder{ResponseWriter: under, status: http.StatusOK}
		rec.WriteHeader(http.StatusAccepted)
		rec.WriteHeader(http.StatusTeapot) // superfluous; ignored by recorder and net/http
		require.Equal(t, http.StatusAccepted, rec.status)
		require.Equal(t, http.StatusAccepted, under.Code)
	})

	t.Run("Write locks the implicit 200 and accumulates bytes", func(t *testing.T) {
		under := httptest.NewRecorder()
		rec := &responseRecorder{ResponseWriter: under, status: http.StatusOK}
		n1, err := rec.Write([]byte("abc"))
		require.NoError(t, err)
		n2, err := rec.Write([]byte("de"))
		require.NoError(t, err)
		require.Equal(t, 3, n1)
		require.Equal(t, 2, n2)
		require.Equal(t, int64(5), rec.written)
		require.True(t, rec.wroteHeader)

		// A WriteHeader after a Write must not change the locked-in status.
		rec.WriteHeader(http.StatusInternalServerError)
		require.Equal(t, http.StatusOK, rec.status)
		require.Equal(t, "abcde", under.Body.String())
	})
}

func TestResponseRecorderUnwrap(t *testing.T) {
	defer goleak.VerifyNone(t)
	under := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: under, status: http.StatusOK}
	require.Same(t, under, rec.Unwrap())
}
