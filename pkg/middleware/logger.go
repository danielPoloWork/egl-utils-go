package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger returns middleware that logs one structured line per request to l
// after the downstream handler returns, recording the method, the request
// path, the response status, the wall-clock duration, and the number of
// response body bytes written. When the request carries a RequestID (this
// package, run earlier in the chain), its value is attached as request_id so
// the line correlates with the rest of the request's trail.
//
// The line's level is derived from the status: 5xx logs at Error, 4xx at
// Warn, everything else at Info. The request query string is deliberately
// not logged — only the path — because query parameters routinely carry
// secrets (tokens, keys); see ADR-0014. Logging happens from a deferred
// call, so a request whose handler panics is still logged before the panic
// propagates; compose Logger outside Recoverer (Logger(Recoverer(h))) so the
// recovered 500 is the status it observes.
//
// Logger panics if l is nil, and the returned decorator panics if next is
// nil — wiring errors, caught at setup (ADR-0005 idiom).
func Logger(l *slog.Logger) func(http.Handler) http.Handler {
	if l == nil {
		panic("middleware: nil logger")
	}
	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("middleware: nil handler")
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			defer func() {
				elapsed := time.Since(start)
				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rec.status),
					slog.Duration("duration", elapsed),
					slog.Int64("bytes", rec.written),
				}
				if id := RequestIDFrom(r.Context()); id != "" {
					attrs = append(attrs, slog.String("request_id", id))
				}
				l.LogAttrs(r.Context(), levelFor(rec.status), "http request", attrs...)
			}()
			next.ServeHTTP(rec, r)
		})
	}
}

// levelFor maps a response status to the log level of its line.
func levelFor(status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// responseRecorder wraps an http.ResponseWriter to observe the status code
// and the number of body bytes written, without altering what reaches the
// client. It defaults the status to 200 — the value net/http sends when a
// handler writes a body or returns without calling WriteHeader.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

// WriteHeader records the first status code and forwards it; net/http ignores
// later calls, so the recorder does too.
func (r *responseRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Write locks in the implicit 200 (if WriteHeader was not called) and
// accumulates the byte count.
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// Unwrap exposes the wrapped writer so http.ResponseController can reach the
// underlying Flusher, Hijacker, and the rest — the Go 1.20+ way to preserve
// optional ResponseWriter capabilities through a wrapper.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
