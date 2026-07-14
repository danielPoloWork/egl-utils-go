package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// captureDefault installs a structured logger writing to buf as slog's default
// for the test, restoring the original afterwards. Not parallel-safe (the
// default logger is global).
func captureDefault(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	orig := slog.Default()
	slog.SetDefault(logger.NewStructured(logger.WithWriter(buf)))
	t.Cleanup(func() { slog.SetDefault(orig) })
	return buf
}

func TestFromContextAppliesFields(t *testing.T) {
	defer goleak.VerifyNone(t)
	buf := captureDefault(t)
	ctx := logger.WithFields(context.Background(),
		logger.String("service", "api"), logger.Int("shard", 3))
	logger.FromContext(ctx).Info("hello")

	recs := lines(t, buf)
	require.Len(t, recs, 1)
	require.Equal(t, "api", recs[0]["service"])
	require.Equal(t, float64(3), recs[0]["shard"]) // JSON numbers decode to float64
	require.Equal(t, "hello", recs[0]["msg"])
}

func TestWithFieldsAccumulate(t *testing.T) {
	defer goleak.VerifyNone(t)
	buf := captureDefault(t)
	outer := logger.WithFields(context.Background(), logger.String("a", "1"))
	inner := logger.WithFields(outer, logger.String("b", "2"))
	logger.FromContext(inner).Info("x")

	recs := lines(t, buf)
	require.Len(t, recs, 1)
	require.Equal(t, "1", recs[0]["a"])
	require.Equal(t, "2", recs[0]["b"])
}

func TestWithFieldsDoesNotMutateParent(t *testing.T) {
	defer goleak.VerifyNone(t)
	buf := captureDefault(t)
	outer := logger.WithFields(context.Background(), logger.String("a", "1"))
	_ = logger.WithFields(outer, logger.String("b", "2")) // must not leak into outer
	logger.FromContext(outer).Info("x")

	recs := lines(t, buf)
	require.Len(t, recs, 1)
	require.Equal(t, "1", recs[0]["a"])
	require.NotContains(t, recs[0], "b", "the inner field must not leak into the parent context")
}

func TestWithFieldsNoFieldsReturnsSameContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctx := context.Background()
	require.Equal(t, ctx, logger.WithFields(ctx))
}

func TestFromContextNoFieldsReturnsDefault(t *testing.T) {
	defer goleak.VerifyNone(t)
	buf := captureDefault(t)
	logger.FromContext(context.Background()).Info("bare")

	recs := lines(t, buf)
	require.Len(t, recs, 1)
	require.Equal(t, "bare", recs[0]["msg"])
	// No context fields beyond the standard slog keys.
	for k := range recs[0] {
		require.Contains(t, []string{"time", "level", "msg"}, k)
	}
}

func TestFieldConstructors(t *testing.T) {
	defer goleak.VerifyNone(t)
	buf := captureDefault(t)
	ctx := logger.WithFields(
		context.Background(),
		logger.String("s", "x"),
		logger.Int("i", 7),
		logger.Bool("b", true),
		logger.Duration("d", 2*time.Second),
		logger.Any("a", []string{"y", "z"}),
	)
	logger.FromContext(ctx).Info("m")

	recs := lines(t, buf)
	require.Len(t, recs, 1)
	require.Equal(t, "x", recs[0]["s"])
	require.Equal(t, float64(7), recs[0]["i"])
	require.Equal(t, true, recs[0]["b"])
	require.Equal(t, float64(2*time.Second), recs[0]["d"]) // slog JSON encodes Duration as ns
	require.Equal(t, []any{"y", "z"}, recs[0]["a"])
}
