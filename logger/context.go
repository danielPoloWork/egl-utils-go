package logger

import (
	"context"
	"log/slog"
	"time"
)

// Field is a single structured logging attribute. It is an alias for
// slog.Attr, so a value from slog (slog.String, slog.Group, …) is a Field and
// vice versa; the constructors below are conveniences so a caller need not
// import slog for the common cases.
type Field = slog.Attr

// String builds a string Field. It and its siblings (Int, Bool, Duration, Any)
// wrap the corresponding slog constructors.
func String(key, value string) Field { return slog.String(key, value) }

// Int builds an integer Field.
func Int(key string, value int) Field { return slog.Int(key, value) }

// Bool builds a boolean Field.
func Bool(key string, value bool) Field { return slog.Bool(key, value) }

// Duration builds a time.Duration Field.
func Duration(key string, value time.Duration) Field { return slog.Duration(key, value) }

// Any builds a Field from a value of any type.
func Any(key string, value any) Field { return slog.Any(key, value) }

// fieldsKey is the unexported context key under which the accumulated fields
// live; an unexported type cannot collide with another package's keys.
type fieldsKey struct{}

// WithFields returns a copy of ctx carrying fields in addition to any already
// attached by an earlier WithFields — fields accumulate down a call chain, so a
// request-scoped field set by an outer layer is still present for an inner one.
// Calling it with no fields returns ctx unchanged. The parent context's field
// set is never mutated.
func WithFields(ctx context.Context, fields ...Field) context.Context {
	if len(fields) == 0 {
		return ctx
	}
	existing, _ := ctx.Value(fieldsKey{}).([]Field)
	merged := make([]Field, 0, len(existing)+len(fields))
	merged = append(merged, existing...)
	merged = append(merged, fields...)
	return context.WithValue(ctx, fieldsKey{}, merged)
}

// FromContext returns a *slog.Logger derived from slog.Default with every field
// attached to ctx by WithFields applied, so its records carry the accumulated
// context. When ctx carries no fields it returns slog.Default unchanged. Wire
// the aggregation logger as the base with slog.SetDefault(logger.NewStructured(…)).
func FromContext(ctx context.Context) *slog.Logger {
	fields, _ := ctx.Value(fieldsKey{}).([]Field)
	if len(fields) == 0 {
		return slog.Default()
	}
	args := make([]any, len(fields))
	for i, f := range fields {
		args[i] = f
	}
	return slog.Default().With(args...)
}
