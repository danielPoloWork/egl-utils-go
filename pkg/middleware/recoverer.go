package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer is middleware that turns a panic from a downstream handler into a
// clean 500 response instead of a dropped connection — and, for a panic that
// would otherwise unwind past net/http, a protected server. When next panics,
// Recoverer recovers, logs the panic value and a stack trace server-side at
// Error level via slog.Default (with method, path, and request_id when the
// chain seeded one), and — if the response has not been committed yet — writes
// a generic "Internal Server Error" with status 500.
//
// The panic value and the stack trace are logged server-side only; they are
// never written to the response. Leaking a stack trace or panic message to the
// client is an information-disclosure vector (ADR-0016), so the client sees the
// generic status text and nothing more.
//
// http.ErrAbortHandler is deliberately not recovered: net/http uses it as a
// sentinel to abort a handler silently, so Recoverer re-panics it unchanged and
// preserves that contract.
//
// If the handler already wrote a status or body before panicking, the response
// is already committed and its status cannot be corrected; Recoverer still logs
// the panic but leaves the partial response untouched. Compose Recoverer as the
// innermost middleware around the handler (e.g. Logger(Recoverer(h))), so the
// outer middleware observe the recovered 500 (ADR-0014).
//
// Recoverer panics if next is nil — a wiring error, caught at setup (ADR-0013).
func Recoverer(next http.Handler) http.Handler {
	if next == nil {
		panic("middleware: nil handler")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				// net/http's silent-abort sentinel: propagate unchanged.
				panic(v)
			}
			logPanic(r, v)
			if !rec.wroteHeader {
				http.Error(rec, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

// logPanic records a recovered panic on the default slog logger at Error level,
// carrying the request method and path (never the query string — the same
// information-disclosure rule as Logger, ADR-0014), the panic value, a stack
// trace, and request_id when the chain carries one. The stack is captured and
// logged server-side only; it never reaches the client.
func logPanic(r *http.Request, v any) {
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Any("panic", v),
		slog.String("stack", string(debug.Stack())),
	}
	if id := RequestIDFrom(r.Context()); id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	slog.Default().LogAttrs(r.Context(), slog.LevelError, "http handler panic", attrs...)
}
