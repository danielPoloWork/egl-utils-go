package ratelimit_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/ratelimit"
)

// Allow is the fail-fast path: it answers immediately and never queues.
func ExampleLimiter_Allow() {
	// The bucket starts full, so a fresh limiter admits its whole burst at
	// once. Refill is computed from elapsed time, and at one token per second
	// nothing measurable refills during this example — which is why the third
	// answer is stable.
	limiter := ratelimit.NewLimiter(1, 2)

	fmt.Println(limiter.Allow(), limiter.Allow(), limiter.Allow())
	// Output: true true false
}

// Middleware wraps the fail-fast path as an ordinary net/http decorator.
func ExampleLimiter_Middleware() {
	limiter := ratelimit.NewLimiter(1, 1)

	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	admitted := httptest.NewRecorder()
	handler.ServeHTTP(admitted, httptest.NewRequest(http.MethodGet, "/", nil))
	fmt.Println(admitted.Code)

	// The second request is shed rather than parked: a burst cannot accumulate
	// waiting goroutines and held connections. The refusal carries Retry-After
	// in whole seconds and discloses nothing about the limiter's state.
	shed := httptest.NewRecorder()
	handler.ServeHTTP(shed, httptest.NewRequest(http.MethodGet, "/", nil))
	fmt.Println(shed.Code, shed.Header().Get("Retry-After"))
	// Output:
	// 200
	// 429 1
}
