package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CorsConfig configures the Cors middleware. The zero value denies every
// cross-origin request (no allowed origins), which is the safe default; a
// consumer opts in by naming the origins, methods, and headers it trusts.
type CorsConfig struct {
	// AllowedOrigins is the set of origins permitted to make cross-origin
	// requests, each an exact origin serialization ("https://app.example.com").
	// A single "*" entry allows every origin; empty allows none. Matching is
	// exact and case-sensitive — configure origins as the browser sends them
	// (lowercase scheme and host).
	AllowedOrigins []string

	// AllowedMethods is the set of methods advertised in a preflight's
	// Access-Control-Allow-Methods. Empty defaults to the CORS-safelisted
	// methods GET, HEAD, POST.
	AllowedMethods []string

	// AllowedHeaders is the set of request headers advertised in a preflight's
	// Access-Control-Allow-Headers. Empty or a single "*" entry reflects the
	// browser's Access-Control-Request-Headers verbatim; otherwise the explicit
	// list is sent.
	AllowedHeaders []string

	// ExposedHeaders lists response headers the browser may expose to script
	// via Access-Control-Expose-Headers. Empty omits the header.
	ExposedHeaders []string

	// AllowCredentials sets Access-Control-Allow-Credentials: true, permitting
	// cookies and HTTP auth on cross-origin requests. It is forbidden by the
	// Fetch spec to combine credentials with a wildcard origin; Cors panics at
	// construction if AllowCredentials is set while AllowedOrigins contains "*".
	AllowCredentials bool

	// MaxAge is how long a browser may cache a preflight result. It is emitted
	// as whole seconds in Access-Control-Max-Age; zero omits the header (the
	// browser applies its own default). A negative value is a configuration
	// error and panics at construction.
	MaxAge time.Duration
}

// CORS response/request header names and the default method set.
const (
	headerOrigin             = "Origin"
	headerACRequestMethod    = "Access-Control-Request-Method"
	headerACRequestHeaders   = "Access-Control-Request-Headers"
	headerACAllowOrigin      = "Access-Control-Allow-Origin"
	headerACAllowMethods     = "Access-Control-Allow-Methods"
	headerACAllowHeaders     = "Access-Control-Allow-Headers"
	headerACAllowCredentials = "Access-Control-Allow-Credentials"
	headerACExposeHeaders    = "Access-Control-Expose-Headers"
	headerACMaxAge           = "Access-Control-Max-Age"
	headerVary               = "Vary"
)

var defaultCorsMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost}

// cors holds the precomputed, immutable form of a CorsConfig so the per-request
// path allocates nothing beyond what the response headers require.
type cors struct {
	allowedOrigins   map[string]struct{}
	allowAllOrigins  bool
	allowCredentials bool
	allowedMethods   string // joined, ready for the header
	allowedHeaders   string // joined; used only when reflectHeaders is false
	reflectHeaders   bool
	exposedHeaders   string // joined; "" omits the header
	maxAge           string // whole seconds; "" omits the header
}

// Cors returns middleware that answers CORS preflight requests and annotates
// cross-origin responses per cfg. A request without an Origin header is passed
// through untouched (it is not cross-origin). A preflight (an OPTIONS request
// carrying Access-Control-Request-Method) is answered directly with 204 and the
// negotiated Access-Control-* headers, and is not forwarded to next. Any other
// request is forwarded to next, with Access-Control-Allow-Origin (and the
// credential/expose headers) added when the origin is allowed and omitted when
// it is not — the browser, not this middleware, enforces the policy.
//
// CORS is a security boundary, so two misconfigurations are rejected loudly at
// construction (ADR-0017): AllowCredentials with a "*" origin (forbidden by the
// Fetch spec), and a negative MaxAge. The returned decorator panics if next is
// nil, as elsewhere in this package (ADR-0013).
func Cors(cfg CorsConfig) func(http.Handler) http.Handler {
	c := newCors(cfg)
	return func(next http.Handler) http.Handler {
		if next == nil {
			panic("middleware: nil handler")
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get(headerOrigin)
			if origin == "" {
				next.ServeHTTP(w, r) // not a cross-origin request
				return
			}
			allowed := c.allowAllOrigins || c.isAllowed(origin)
			if r.Method == http.MethodOptions && r.Header.Get(headerACRequestMethod) != "" {
				c.preflight(w, r, origin, allowed)
				return // preflight is terminal — do not forward to next
			}
			c.actual(w, origin, allowed)
			next.ServeHTTP(w, r)
		})
	}
}

// newCors validates cfg and precomputes the per-request form. It panics on the
// two configuration errors CORS cannot express safely.
func newCors(cfg CorsConfig) *cors {
	allowAll := false
	origins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		if o == "*" {
			allowAll = true
			continue
		}
		origins[o] = struct{}{}
	}
	if cfg.AllowCredentials && allowAll {
		panic(`middleware: CORS AllowCredentials with wildcard origin "*" is forbidden by the Fetch spec`)
	}
	if cfg.MaxAge < 0 {
		panic("middleware: CORS negative MaxAge")
	}

	methods := cfg.AllowedMethods
	if len(methods) == 0 {
		methods = defaultCorsMethods
	}
	reflect := len(cfg.AllowedHeaders) == 0
	for _, h := range cfg.AllowedHeaders {
		if h == "*" {
			reflect = true
			break
		}
	}
	maxAge := ""
	if cfg.MaxAge > 0 {
		maxAge = strconv.FormatInt(int64(cfg.MaxAge/time.Second), 10)
	}

	return &cors{
		allowedOrigins:   origins,
		allowAllOrigins:  allowAll,
		allowCredentials: cfg.AllowCredentials,
		allowedMethods:   strings.Join(methods, ", "),
		allowedHeaders:   strings.Join(cfg.AllowedHeaders, ", "),
		reflectHeaders:   reflect,
		exposedHeaders:   strings.Join(cfg.ExposedHeaders, ", "),
		maxAge:           maxAge,
	}
}

// isAllowed reports whether origin is in the configured exact-match set.
func (c *cors) isAllowed(origin string) bool {
	_, ok := c.allowedOrigins[origin]
	return ok
}

// writeAllowOrigin sets Access-Control-Allow-Origin and reports whether the
// response now varies on Origin. It emits "*" only for the wildcard-without-
// credentials case (the credentials+wildcard combination is rejected at
// construction) and then does not vary; otherwise it echoes the specific origin
// and returns true so the caller adds Vary: Origin exactly once.
func (c *cors) writeAllowOrigin(w http.ResponseWriter, origin string) (variesOnOrigin bool) {
	if c.allowAllOrigins && !c.allowCredentials {
		w.Header().Set(headerACAllowOrigin, "*")
		return false
	}
	w.Header().Set(headerACAllowOrigin, origin)
	return true
}

// actual annotates a non-preflight cross-origin response. When the origin is
// not allowed no CORS headers are written and the browser blocks the response.
func (c *cors) actual(w http.ResponseWriter, origin string, allowed bool) {
	if !allowed {
		return
	}
	if c.writeAllowOrigin(w, origin) {
		w.Header().Add(headerVary, headerOrigin)
	}
	if c.allowCredentials {
		w.Header().Set(headerACAllowCredentials, "true")
	}
	if c.exposedHeaders != "" {
		w.Header().Set(headerACExposeHeaders, c.exposedHeaders)
	}
}

// preflight answers an OPTIONS preflight with 204 and the negotiated headers.
// The Vary set always names the request headers a preflight branches on, so a
// cache never serves one origin's preflight to another.
func (c *cors) preflight(w http.ResponseWriter, r *http.Request, origin string, allowed bool) {
	h := w.Header()
	h.Add(headerVary, headerOrigin)
	h.Add(headerVary, headerACRequestMethod)
	h.Add(headerVary, headerACRequestHeaders)
	if allowed {
		c.writeAllowOrigin(w, origin)
		h.Set(headerACAllowMethods, c.allowedMethods)
		if c.reflectHeaders {
			if req := r.Header.Get(headerACRequestHeaders); req != "" {
				h.Set(headerACAllowHeaders, req)
			}
		} else if c.allowedHeaders != "" {
			h.Set(headerACAllowHeaders, c.allowedHeaders)
		}
		if c.allowCredentials {
			h.Set(headerACAllowCredentials, "true")
		}
		if c.maxAge != "" {
			h.Set(headerACMaxAge, c.maxAge)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
