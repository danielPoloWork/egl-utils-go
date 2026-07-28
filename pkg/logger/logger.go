// Package logger builds structured slog loggers tuned for log aggregation and
// carries per-request logger fields through a context.Context.
//
// NewStructured returns a *slog.Logger backed by slog's JSON handler — one JSON
// object per line, the format ElasticSearch and Grafana Loki ingest directly —
// with functional options for the level, destination, source annotation, and a
// set of base attributes (e.g. service and version) stamped on every record.
// The returned logger plugs straight into middleware.Logger (roadmap 4.2).
package logger

import (
	"io"
	"log/slog"
	"os"
)

type options struct {
	w         io.Writer
	level     slog.Leveler
	addSource bool
	attrs     []slog.Attr
}

// Option configures NewStructured.
type Option func(*options)

// WithWriter directs output to w instead of the default os.Stdout. A nil w is
// ignored, keeping the default.
func WithWriter(w io.Writer) Option {
	return func(o *options) {
		if w != nil {
			o.w = w
		}
	}
}

// WithLevel sets the minimum level to emit (default Info). Passing a
// *slog.LevelVar makes the threshold adjustable at runtime. A nil level is
// ignored.
func WithLevel(level slog.Leveler) Option {
	return func(o *options) {
		if level != nil {
			o.level = level
		}
	}
}

// WithSource annotates every record with the source file and line ("source":
// {...}). It carries a runtime cost and is off by default.
func WithSource() Option {
	return func(o *options) { o.addSource = true }
}

// WithAttrs stamps attrs onto every record — the aggregation-friendly place for
// stable identifiers like service name, version, or environment. Repeated calls
// accumulate.
func WithAttrs(attrs ...slog.Attr) Option {
	return func(o *options) { o.attrs = append(o.attrs, attrs...) }
}

// NewStructured returns a JSON slog.Logger tuned for aggregation. By default it
// writes Info-and-above to os.Stdout with slog's standard time/level/msg keys —
// the lingua franca ElasticSearch and Loki understand — and no source location.
func NewStructured(opts ...Option) *slog.Logger {
	o := options{w: os.Stdout, level: slog.LevelInfo}
	for _, opt := range opts {
		opt(&o)
	}
	var h slog.Handler = slog.NewJSONHandler(o.w, &slog.HandlerOptions{
		Level:     o.level,
		AddSource: o.addSource,
	})
	if len(o.attrs) > 0 {
		h = h.WithAttrs(o.attrs)
	}
	return slog.New(h)
}
