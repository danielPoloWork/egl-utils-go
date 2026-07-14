package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

// HeaderName is the HTTP header RequestID reads an inbound request ID from
// and writes the resolved ID back to: the de-facto standard X-Request-ID.
const HeaderName = "X-Request-ID"

// maxIDLen bounds an accepted inbound request ID. It is generous enough for
// every common format (UUID, ULID, hex, base64url trace IDs) while capping
// the memory and log volume an attacker-supplied header can force.
const maxIDLen = 128

// requestIDKey is the unexported context key for the request ID; an
// unexported type cannot collide with keys set by other packages.
type requestIDKey struct{}

// RequestID is middleware that gives every request a correlation ID. It
// adopts a client-supplied X-Request-ID when the header is present and valid,
// and generates a fresh one otherwise; the resolved ID is stored in the
// request context (read it with RequestIDFrom) and echoed in the response's
// X-Request-ID header so callers and proxies can correlate. It panics if
// next is nil — a nil handler is a wiring error, caught here rather than on
// the first request.
//
// The ID is a correlation token only: it is derived from untrusted input and
// must never be used for authentication or authorization.
func RequestID(next http.Handler) http.Handler {
	if next == nil {
		panic("middleware: nil handler")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderName)
		if !isValidID(id) {
			id = generateID()
		}
		w.Header().Set(HeaderName, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the request ID stored in ctx by RequestID, or the
// empty string if there is none.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// isValidID reports whether an inbound ID may be adopted verbatim. It admits
// a non-empty string of at most maxIDLen bytes, each a visible ASCII
// character (0x21–0x7e): this excludes control characters — CR and LF in
// particular, which would enable log- and header-injection when the ID is
// echoed or logged downstream — as well as spaces, while accepting every
// common ID alphabet. An ID that fails is replaced by a generated one rather
// than rejected with an error, so a malformed header never fails the request.
func isValidID(id string) bool {
	if len(id) == 0 || len(id) > maxIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < 0x21 || id[i] > 0x7e {
			return false
		}
	}
	return true
}

// generateID returns a fresh, unpredictable request ID. crypto/rand.Text
// yields at least 128 bits of randomness over the RFC 4648 base32 alphabet —
// unguessable, collision-resistant, and (being crypto/rand) free of any
// weak-RNG concern; the value leaks no server state.
func generateID() string {
	return rand.Text()
}
