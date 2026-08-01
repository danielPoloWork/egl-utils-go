package middleware_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware"
)

// The four middleware compose as a standard decorator chain. Order matters, and
// this is the order to copy: Recoverer outermost so a panic anywhere inside it
// still becomes a 500, then RequestID so the id exists before anything logs,
// then Logger, then Cors closest to the handler.
func Example() {
	cors := middleware.Cors(middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
	})

	var mux http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.Recoverer(
		middleware.RequestID(
			middleware.Logger(slog.New(slog.NewJSONHandler(io.Discard, nil)))(
				cors(mux),
			),
		),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/orders", nil))

	fmt.Println(rec.Code, rec.Header().Get(middleware.HeaderName) != "")
	// Output: 200 true
}

// RequestID reuses the client's correlation id when there is one and generates
// one when there is not.
func ExampleRequestID() {
	handler := middleware.RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	// A client-supplied id is kept, so a single id spans every hop of a request.
	// It is sanitized first: an id is used for correlation and never as identity.
	supplied := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.HeaderName, "req-42")
	handler.ServeHTTP(supplied, req)
	fmt.Println(supplied.Header().Get(middleware.HeaderName))

	// Without one the middleware generates it with a CSPRNG. The value is
	// random by design, so only its presence can be printed.
	generated := httptest.NewRecorder()
	handler.ServeHTTP(generated, httptest.NewRequest(http.MethodGet, "/", nil))
	fmt.Println(generated.Header().Get(middleware.HeaderName) != "")
	// Output:
	// req-42
	// true
}

// RequestIDFrom is how everything downstream of the middleware reads the id.
func ExampleRequestIDFrom() {
	handler := middleware.RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// A log line, an outbound call, an error report: all take the id from
		// the context rather than re-reading the header.
		fmt.Println(middleware.RequestIDFrom(r.Context()))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(middleware.HeaderName, "req-42")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	// Output: req-42
}

// Logger emits one structured line per request, after the handler returns.
func ExampleLogger() {
	var buf strings.Builder
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := middleware.Logger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/orders?token=secret", nil),
	)

	// The line also carries duration and bytes, which move every run, and a
	// request_id when RequestID is in the chain — so only the stable fields are
	// printed. Note "path": the query string is deliberately never logged,
	// because secrets travel in it.
	var line map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &line); err != nil {
		fmt.Println("decode:", err)
		return
	}
	fmt.Println(line["level"], line["method"], line["path"], line["status"])
	// Output: INFO POST /orders 201
}

// Recoverer turns a panic into a 500 and tells the client nothing else.
func ExampleRecoverer() {
	// The panic value and its stack are logged on slog.Default. This example
	// sends that to io.Discard so its own output stays readable — in a service
	// it is what you actually want to keep.
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	defer slog.SetDefault(previous)

	handler := middleware.Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("connect to db-primary.internal failed: password=hunter2")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// A generic 500, and nothing from the panic message reaches the response —
	// that message is exactly the kind of thing a stack trace would leak.
	fmt.Println(rec.Code, strings.Contains(rec.Body.String(), "hunter2"))
	// Output: 500 false
}

// Cors answers preflights itself and lets the browser enforce the rest.
func ExampleCors() {
	handler := middleware.Cors(middleware.CorsConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
		MaxAge:         10 * time.Minute,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// A preflight is terminal: the middleware answers 204 and the handler never
	// sees it. Max-Age goes out in whole seconds.
	preflight := httptest.NewRecorder()
	options := httptest.NewRequest(http.MethodOptions, "/orders", nil)
	options.Header.Set("Origin", "https://app.example.com")
	options.Header.Set("Access-Control-Request-Method", http.MethodPost)
	handler.ServeHTTP(preflight, options)
	fmt.Println(preflight.Code,
		preflight.Header().Get("Access-Control-Allow-Origin"),
		preflight.Header().Get("Access-Control-Max-Age"))

	// An origin that is not allowed simply gets no Access-Control-Allow-Origin.
	// The request still runs — CORS is enforced by the browser, so it is not an
	// authorization mechanism.
	rejected := httptest.NewRecorder()
	get := httptest.NewRequest(http.MethodGet, "/orders", nil)
	get.Header.Set("Origin", "https://evil.example.com")
	handler.ServeHTTP(rejected, get)
	fmt.Println(rejected.Code, rejected.Header().Get("Access-Control-Allow-Origin") == "")
	// Output:
	// 204 https://app.example.com 600
	// 200 true
}
