package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/middleware"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// capture is a handler that records the request ID it observed in context.
func capture(seen *string) http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		*seen = middleware.RequestIDFrom(r.Context())
	})
}

// serve drives one request with the given inbound header value (skipped when
// empty) through RequestID and returns the ID seen in context and the ID
// echoed in the response header.
func serve(t *testing.T, header string) (inContext, inResponse string) {
	t.Helper()
	var seen string
	h := middleware.RequestID(capture(&seen))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set(middleware.HeaderName, header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return seen, rec.Header().Get(middleware.HeaderName)
}

func TestRequestIDNilHandlerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Panics(t, func() { middleware.RequestID(nil) })
}

func TestRequestIDGeneratesWhenAbsent(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctxID, respID := serve(t, "")
	require.NotEmpty(t, ctxID, "an ID must be generated when the header is absent")
	require.Equal(t, ctxID, respID, "the response header must echo the context ID")
	requireValidGenerated(t, ctxID)
}

func TestRequestIDPreservesValidInbound(t *testing.T) {
	defer goleak.VerifyNone(t)
	const inbound = "550e8400-e29b-41d4-a716-446655440000"
	ctxID, respID := serve(t, inbound)
	require.Equal(t, inbound, ctxID, "a valid inbound ID must be adopted verbatim")
	require.Equal(t, inbound, respID)
}

func TestRequestIDRegeneratesOnInvalidInbound(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := map[string]string{
		"CRLF injection":  "abc\r\nSet-Cookie: x=y",
		"embedded CR":     "abc\rdef",
		"embedded LF":     "abc\ndef",
		"NUL byte":        "abc\x00def",
		"tab":             "abc\tdef",
		"space":           "has a space",
		"non-ASCII":       "idé",
		"too long":        string(make([]byte, 129)), // 129 NULs: overlong and control
		"exactly overlen": repeat('a', 129),
	}
	for name, inbound := range cases {
		t.Run(name, func(t *testing.T) {
			ctxID, respID := serve(t, inbound)
			require.NotEqual(t, inbound, ctxID, "an invalid inbound ID must be replaced")
			require.Equal(t, ctxID, respID)
			requireValidGenerated(t, ctxID)
		})
	}
}

func TestRequestIDAcceptsMaxLength(t *testing.T) {
	defer goleak.VerifyNone(t)
	inbound := repeat('a', 128)
	ctxID, _ := serve(t, inbound)
	require.Equal(t, inbound, ctxID, "an ID at exactly the length cap must be adopted")
}

func TestRequestIDFromEmptyContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Empty(t, middleware.RequestIDFrom(context.Background()))
}

func TestRequestIDGeneratesDistinctIDs(t *testing.T) {
	defer goleak.VerifyNone(t)
	const n = 100
	seen := make(map[string]struct{}, n)
	for range n {
		id, _ := serve(t, "")
		require.NotContains(t, seen, id, "generated IDs must be distinct")
		seen[id] = struct{}{}
	}
}

func TestRequestIDCallsNextExactlyOnceWithUpdatedContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := 0
	var ctxID string
	h := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls++
		ctxID = middleware.RequestIDFrom(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 1, calls, "next must run exactly once")
	require.NotEmpty(t, ctxID, "next must see the ID in its request context")
}

// requireValidGenerated asserts id looks like a crypto/rand.Text value: at
// least 26 base32 (RFC 4648) characters.
func requireValidGenerated(t *testing.T, id string) {
	t.Helper()
	require.GreaterOrEqual(t, len(id), 26, "generated ID is too short to carry 128 bits")
	for _, c := range id {
		isBase32 := (c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')
		require.Truef(t, isBase32, "generated ID has non-base32 char %q", c)
	}
}

func repeat(b byte, n int) string {
	s := make([]byte, n)
	for i := range s {
		s[i] = b
	}
	return string(s)
}
