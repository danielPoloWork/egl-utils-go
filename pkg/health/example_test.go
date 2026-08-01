package health_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/health"
)

// Handler probes every dependency concurrently and answers 200 or 503.
func ExampleHandler() {
	handler := health.Handler(
		health.Check{
			Name: "database",
			// A real probe hands ctx to its driver — db.PingContext(ctx) — so a
			// client that hangs up stops the work its request started. Returning
			// ctx.Err() is the same contract in miniature: nil while the request
			// is alive.
			Probe: func(ctx context.Context) error { return ctx.Err() },
		},
		health.Check{
			Name: "cache",
			Probe: func(context.Context) error {
				return errors.New("dial tcp 10.0.0.7:6379: connection refused")
			},
		},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	// Any failing probe makes the endpoint unavailable. The body names which
	// check failed and never why: the probe's error would hand an
	// unauthenticated caller hostnames, ports and DSNs, so it belongs in the
	// server's logs. Probes run concurrently on the request's context, so the
	// latency is the slowest probe rather than their sum.
	fmt.Println(rec.Code)
	fmt.Print(rec.Body.String())
	// Output:
	// 503
	// {"status":"unavailable","checks":{"cache":"fail","database":"ok"}}
}

// With every probe passing — or no probes at all — the endpoint is healthy.
func ExampleHandler_healthy() {
	handler := health.Handler(
		health.Check{Name: "database", Probe: func(context.Context) error { return nil }},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	fmt.Println(rec.Code)
	fmt.Print(rec.Body.String())
	// Output:
	// 200
	// {"status":"ok","checks":{"database":"ok"}}
}
