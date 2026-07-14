package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/middleware"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// serveCors runs one request through Cors(cfg) wrapping a handler that records
// whether it was reached, and returns the response recorder and that flag.
func serveCors(t *testing.T, cfg middleware.CorsConfig, r *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	reached := false
	h := middleware.Cors(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec, reached
}

func corsReq(method, target, origin string, hdrs map[string]string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	for k, v := range hdrs {
		r.Header.Set(k, v)
	}
	return r
}

func TestCorsNilHandlerPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Panics(t, func() { middleware.Cors(middleware.CorsConfig{})(nil) })
}

func TestCorsCredentialsWithWildcardPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t,
		`middleware: CORS AllowCredentials with wildcard origin "*" is forbidden by the Fetch spec`,
		func() {
			middleware.Cors(middleware.CorsConfig{AllowedOrigins: []string{"*"}, AllowCredentials: true})
		})
}

func TestCorsNegativeMaxAgePanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "middleware: CORS negative MaxAge", func() {
		middleware.Cors(middleware.CorsConfig{MaxAge: -time.Second})
	})
}

func TestCorsNonCorsRequestPassthrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodGet, "/", "", nil))
	require.True(t, reached, "a request without Origin is forwarded")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), "no CORS headers for a non-CORS request")
}

func TestCorsActualAllowedSpecificOrigin(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodGet, "/", "https://app.example.com", nil))
	require.True(t, reached)
	require.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, rec.Header().Values("Vary"), "Origin", "an echoed origin must Vary on Origin")
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCorsActualWildcardNoCredentials(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"*"}},
		corsReq(http.MethodGet, "/", "https://anywhere.example", nil))
	require.True(t, reached)
	require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	require.NotContains(t, rec.Header().Values("Vary"), "Origin", `"*" does not vary on Origin`)
}

func TestCorsActualDisallowedOrigin(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodGet, "/", "https://evil.example", nil))
	require.True(t, reached, "the request still reaches the handler; the browser blocks the response")
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), "no ACAO for a disallowed origin")
}

func TestCorsActualCredentialsEchoesOrigin(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{
		AllowedOrigins:   []string{"https://app.example.com"},
		AllowCredentials: true,
	}, corsReq(http.MethodGet, "/", "https://app.example.com", nil))
	require.True(t, reached)
	require.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	require.Contains(t, rec.Header().Values("Vary"), "Origin")
}

func TestCorsActualExposedHeaders(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := serveCors(t, middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		ExposedHeaders: []string{"X-Total-Count", "X-Page"},
	}, corsReq(http.MethodGet, "/", "https://app.example.com", nil))
	require.Equal(t, "X-Total-Count, X-Page", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestCorsPreflightAllowed(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodGet, http.MethodPut},
		MaxAge:         10 * time.Minute,
	}, corsReq(http.MethodOptions, "/things", "https://app.example.com",
		map[string]string{"Access-Control-Request-Method": http.MethodPut}))

	require.False(t, reached, "a preflight is terminal — next must not be called")
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "GET, PUT", rec.Header().Get("Access-Control-Allow-Methods"))
	require.Equal(t, "600", rec.Header().Get("Access-Control-Max-Age"))
	vary := rec.Header().Values("Vary")
	require.Contains(t, vary, "Origin")
	require.Contains(t, vary, "Access-Control-Request-Method")
	require.Contains(t, vary, "Access-Control-Request-Headers")
}

func TestCorsPreflightReflectsRequestHeaders(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Empty AllowedHeaders reflects the browser's Access-Control-Request-Headers.
	rec, _ := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodOptions, "/", "https://app.example.com", map[string]string{
			"Access-Control-Request-Method":  http.MethodPost,
			"Access-Control-Request-Headers": "X-Token, Content-Type",
		}))
	require.Equal(t, "X-Token, Content-Type", rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestCorsPreflightExplicitHeaders(t *testing.T) {
	defer goleak.VerifyNone(t)
	// An explicit list is sent verbatim, ignoring the request's ACRH.
	rec, _ := serveCors(t, middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedHeaders: []string{"X-Token"},
	}, corsReq(http.MethodOptions, "/", "https://app.example.com", map[string]string{
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "X-Evil",
	}))
	require.Equal(t, "X-Token", rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestCorsPreflightDisallowedOrigin(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, reached := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodOptions, "/", "https://evil.example",
			map[string]string{"Access-Control-Request-Method": http.MethodPost}))
	require.False(t, reached, "preflight is still terminal for a disallowed origin")
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Methods"))
}

func TestCorsPreflightDefaultMethods(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodOptions, "/", "https://app.example.com",
			map[string]string{"Access-Control-Request-Method": http.MethodPost}))
	require.Equal(t, "GET, HEAD, POST", rec.Header().Get("Access-Control-Allow-Methods"),
		"empty AllowedMethods defaults to the CORS-safelisted methods")
}

func TestCorsPreflightMaxAgeOmittedWhenZero(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodOptions, "/", "https://app.example.com",
			map[string]string{"Access-Control-Request-Method": http.MethodPost}))
	_, ok := rec.Header()["Access-Control-Max-Age"]
	require.False(t, ok, "a zero MaxAge omits the header")
}

func TestCorsPreflightWildcardHeadersReflect(t *testing.T) {
	defer goleak.VerifyNone(t)
	// A "*" entry in AllowedHeaders reflects the request's ACRH, like empty.
	rec, _ := serveCors(t, middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedHeaders: []string{"*"},
	}, corsReq(http.MethodOptions, "/", "https://app.example.com", map[string]string{
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "X-Anything",
	}))
	require.Equal(t, "X-Anything", rec.Header().Get("Access-Control-Allow-Headers"))
}

func TestCorsPreflightReflectWithoutRequestHeaders(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Reflecting mode with no Access-Control-Request-Headers omits Allow-Headers.
	rec, _ := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodOptions, "/", "https://app.example.com",
			map[string]string{"Access-Control-Request-Method": http.MethodPost}))
	_, ok := rec.Header()["Access-Control-Allow-Headers"]
	require.False(t, ok, "no Allow-Headers when the preflight requested none")
}

func TestCorsPreflightCredentials(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, _ := serveCors(t, middleware.CorsConfig{
		AllowedOrigins:   []string{"https://app.example.com"},
		AllowCredentials: true,
	}, corsReq(http.MethodOptions, "/", "https://app.example.com",
		map[string]string{"Access-Control-Request-Method": http.MethodPost}))
	require.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	require.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsOptionsWithoutRequestMethodIsActual(t *testing.T) {
	defer goleak.VerifyNone(t)
	// An OPTIONS without Access-Control-Request-Method is a real OPTIONS request,
	// not a preflight: it is forwarded to the handler.
	_, reached := serveCors(t, middleware.CorsConfig{AllowedOrigins: []string{"https://app.example.com"}},
		corsReq(http.MethodOptions, "/", "https://app.example.com", nil))
	require.True(t, reached, "a non-preflight OPTIONS is forwarded to next")
}
