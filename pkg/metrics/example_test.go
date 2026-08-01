package metrics_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/metrics"
)

// A Recorder instruments handlers and serves its own metrics endpoint. It owns
// its registry, so a process can have more than one and they stay independent.
func ExampleRecorder() {
	recorder := metrics.New()

	// Wrap the application handler once, at wiring time.
	app := recorder.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for range 2 {
		app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders", nil))
	}

	// Handler serves the Prometheus text exposition format — written directly,
	// with no SDK in the dependency graph. Only the counter is printed here: the
	// duration histogram's buckets depend on how long the requests really took.
	scrape := httptest.NewRecorder()
	recorder.Handler().ServeHTTP(scrape, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	// Note the labels: method and code only. There is deliberately no path
	// label, because a client controls the path and an unbounded label is a
	// cardinality bomb; the method is normalized to a known set for the same
	// reason.
	for _, line := range strings.Split(scrape.Body.String(), "\n") {
		if strings.HasPrefix(line, "http_requests_total{") {
			fmt.Println(line)
		}
	}
	// Output: http_requests_total{code="200",method="GET"} 2
}
