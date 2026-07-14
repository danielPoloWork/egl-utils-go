package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// lines parses the buffer as newline-delimited JSON records.
func lines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if ln == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(ln), &m))
		out = append(out, m)
	}
	return out
}

func TestNewStructuredDefaults(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	lg := logger.NewStructured(logger.WithWriter(&buf))
	lg.Info("hello", "k", "v")

	recs := lines(t, &buf)
	require.Len(t, recs, 1)
	require.Equal(t, "hello", recs[0]["msg"])
	require.Equal(t, "INFO", recs[0]["level"])
	require.Equal(t, "v", recs[0]["k"])
	require.Contains(t, recs[0], "time")
	require.NotContains(t, recs[0], "source", "source is off by default")
}

func TestNewStructuredLevelFilters(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	lg := logger.NewStructured(logger.WithWriter(&buf), logger.WithLevel(slog.LevelWarn))
	lg.Info("dropped")
	lg.Warn("kept")

	recs := lines(t, &buf)
	require.Len(t, recs, 1)
	require.Equal(t, "kept", recs[0]["msg"])
}

func TestNewStructuredDynamicLevel(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelError)
	lg := logger.NewStructured(logger.WithWriter(&buf), logger.WithLevel(lv))

	lg.Warn("below threshold")
	require.Empty(t, lines(t, &buf), "Warn is below Error")

	lv.Set(slog.LevelWarn)
	lg.Warn("now above")
	recs := lines(t, &buf)
	require.Len(t, recs, 1)
	require.Equal(t, "now above", recs[0]["msg"])
}

func TestNewStructuredWithSource(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	lg := logger.NewStructured(logger.WithWriter(&buf), logger.WithSource())
	lg.Info("x")
	require.Contains(t, lines(t, &buf)[0], "source")
}

func TestNewStructuredWithAttrs(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	lg := logger.NewStructured(
		logger.WithWriter(&buf),
		logger.WithAttrs(slog.String("service", "api"), slog.String("env", "prod")),
	)
	lg.Info("one")
	lg.Info("two")
	recs := lines(t, &buf)
	require.Len(t, recs, 2)
	for _, r := range recs {
		require.Equal(t, "api", r["service"])
		require.Equal(t, "prod", r["env"])
	}
}

func TestNewStructuredNilOptionsIgnored(t *testing.T) {
	defer goleak.VerifyNone(t)
	var buf bytes.Buffer
	// nil writer and nil level are ignored, keeping the buffer and default Info.
	lg := logger.NewStructured(logger.WithWriter(nil), logger.WithWriter(&buf), logger.WithLevel(nil))
	lg.Info("kept")
	recs := lines(t, &buf)
	require.Len(t, recs, 1)
	require.Equal(t, "INFO", recs[0]["level"])
}
