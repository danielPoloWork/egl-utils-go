package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/logger"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware"
)

// These tests exist because a directory of Go files that only *compiles* proves
// nothing — the same trap ADR-0053 closes for Example functions with its
// verified `// Output:` rule. Every claim the comments in service.go make about
// the composition is asserted here, so a rearrangement that breaks one of them
// fails CI instead of quietly shipping wrong advice.
//
// The library's own test dependencies (testify, goleak, rapid) are deliberately
// absent: this module requires the core and nothing else, which is the property
// the README advertises, so its tests are stdlib-only.

// syncBuffer is a log sink safe to write from workers and read from the test.
// The JSON handler already serializes its own writes, but a worker goroutine
// writing while the test reads has no synchronization edge of its own, and the
// race detector is right to say so.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// testConfig is the wiring under test with generous limits; each test narrows
// the fields it cares about.
func testConfig() config {
	return config{
		Addr:          ":0",
		Workers:       2,
		QueueSize:     16,
		RateLimit:     1000,
		Burst:         1000,
		AllowedOrigin: "https://app.example.com",
		ShutdownGrace: time.Second,
	}
}

// newTestService builds a service whose log lines all land in the returned
// buffer — including the ones the library writes on slog.Default, which is where
// Recoverer and logger.FromContext send theirs. Restoring the default afterwards
// keeps the test binary's own output clean.
func newTestService(t *testing.T, cfg config) (*service, *syncBuffer, http.Handler) {
	t.Helper()

	sink := &syncBuffer{}
	log := logger.NewStructured(logger.WithWriter(sink), logger.WithLevel(slog.LevelDebug))

	previous := slog.Default()
	slog.SetDefault(log)
	t.Cleanup(func() { slog.SetDefault(previous) })

	svc := newService(cfg, log)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := svc.pool.Close(ctx); err != nil {
			t.Errorf("closing the pool: %v", err)
		}
	})

	return svc, sink, svc.handler()
}

func do(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// The happy path through the whole chain: the order is accepted, and the
// correlation id RequestID generated is on the response.
func TestApplicationChainAcceptsAnOrder(t *testing.T) {
	_, _, h := newTestService(t, testConfig())

	rec := do(h, http.MethodPost, "/orders")

	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if got := rec.Header().Get(middleware.HeaderName); got == "" {
		t.Errorf("%s header is empty; RequestID is not in the chain", middleware.HeaderName)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"accepted"`) {
		t.Errorf("body = %q, want it to report acceptance", got)
	}
}

// The mux's own responses go through the chain, which is why the chain wraps the
// mux rather than each route: an unknown path is still logged and counted.
func TestUnknownApplicationPathIsStillLogged(t *testing.T) {
	_, sink, h := newTestService(t, testConfig())

	if rec := do(h, http.MethodGet, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := sink.String(); !strings.Contains(got, `"path":"/nope"`) {
		t.Errorf("the 404 was not logged; log was %q", got)
	}
}

// The liveness/readiness split, which is the point of having two endpoints:
// once the pool is gone the instance can take no work, but the process is still
// running and must not be restarted for it.
func TestLivenessIgnoresDependenciesThatReadinessReports(t *testing.T) {
	svc, _, h := newTestService(t, testConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.pool.Close(ctx); err != nil {
		t.Fatalf("closing the pool: %v", err)
	}

	if rec := do(h, http.MethodGet, "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want %d: liveness must not depend on a dependency",
			rec.Code, http.StatusOK)
	}

	rec := do(h, http.MethodGet, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"worker-pool":"fail"`) {
		t.Errorf("body = %q, want it to name the failing check", body)
	}
	// The body names which check failed and never why: an unauthenticated
	// readiness endpoint must not describe the internals it probed.
	if strings.Contains(body, "workerpool") || strings.Contains(body, "closed") {
		t.Errorf("body = %q leaks the probe's error text", body)
	}
}

// A saturated pool is reported as not-ready rather than as broken, and it is
// reported by the probe exercising the real admission path.
func TestReadinessFailsWhileTheQueueIsFull(t *testing.T) {
	cfg := testConfig()
	cfg.Workers, cfg.QueueSize = 1, 1
	svc, _, h := newTestService(t, cfg)

	// No sleeping: the first task announces that a worker has dequeued it, so by
	// the time `started` fires the single worker is busy and the one-slot queue is
	// empty. The second submission therefore fills the queue, and the third — the
	// probe's — must find it full.
	//
	// `started` MUST be buffered (BUG-0002). The send is non-blocking so the second
	// invocation of blocker cannot park on it, and on an unbuffered channel a
	// non-blocking send with no receiver yet ready takes `default` and drops the
	// signal — after which this test waits on `<-started` forever. The buffer makes
	// the first send land whether or not the receiver has arrived, which is the
	// ordering guarantee the comment above claims.
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	blocker := func(context.Context) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
	}
	for i := range 2 {
		if err := svc.pool.Submit(context.Background(), blocker); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if i == 0 {
			<-started
		}
	}
	defer close(release)

	rec := do(h, http.MethodGet, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/readyz = %d, want %d while the queue is full",
			rec.Code, http.StatusServiceUnavailable)
	}
}

// A request that cannot be enqueued is shed with a 503 and a Retry-After rather
// than parking the request goroutine — the reason the pool is non-blocking.
func TestOrderIsShedWhenTheQueueIsFull(t *testing.T) {
	cfg := testConfig()
	cfg.Workers, cfg.QueueSize = 1, 1
	svc, _, h := newTestService(t, cfg)

	// Buffered for the reason given in TestReadinessFailsWhileTheQueueIsFull
	// (BUG-0002): an unbuffered channel loses this signal whenever the worker
	// reaches the send before the test reaches the receive.
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	blocker := func(context.Context) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
	}
	for i := range 2 {
		if err := svc.pool.Submit(context.Background(), blocker); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if i == 0 {
			<-started
		}
	}
	defer close(release)

	rec := do(h, http.MethodPost, "/orders")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q", got, "1")
	}
}

// Once shutdown has begun the endpoint says so instead of pretending to accept.
func TestOrderIsRefusedAfterShutdown(t *testing.T) {
	svc, _, h := newTestService(t, testConfig())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.pool.Close(ctx); err != nil {
		t.Fatalf("closing the pool: %v", err)
	}

	rec := do(h, http.MethodPost, "/orders")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got := rec.Body.String(); !strings.Contains(got, "shutting down") {
		t.Errorf("body = %q, want it to say the service is shutting down", got)
	}
}

// The work the handler hands off outlives the request, which is why the task must
// not be built on the request's context. Close drains the queue and joins the
// workers, so once it returns every dispatched task has run.
func TestBackgroundWorkCompletesAfterTheResponse(t *testing.T) {
	svc, sink, h := newTestService(t, testConfig())

	rec := do(h, http.MethodPost, "/orders")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	id := rec.Header().Get(middleware.HeaderName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.pool.Close(ctx); err != nil {
		t.Fatalf("closing the pool: %v", err)
	}

	got := sink.String()
	if !strings.Contains(got, "order dispatched") {
		t.Errorf("the background task did not run; log was %q", got)
	}
	// The id travelled from the middleware's context into a goroutine that no
	// longer has that context — captured by value, which is the only way it can.
	if !strings.Contains(got, `"request_id":"`+id+`"`) {
		t.Errorf("the background line does not carry request_id %q; log was %q", id, got)
	}
}

// The panic handler earns its place: with a single worker, the task that panics
// and the task submitted after it are handled by the same goroutine, so the
// second one running is proof the first did not take the worker down. Without a
// handler the pool would let that panic propagate and end the process.
func TestPanickingTaskDoesNotStopTheWorker(t *testing.T) {
	cfg := testConfig()
	cfg.Workers = 1
	svc, sink, h := newTestService(t, cfg)

	if err := svc.pool.Submit(context.Background(), func(context.Context) {
		panic("row mapper: nil pointer")
	}); err != nil {
		t.Fatalf("submitting the panicking task: %v", err)
	}
	if rec := do(h, http.MethodPost, "/orders"); rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.pool.Close(ctx); err != nil {
		t.Fatalf("closing the pool: %v", err)
	}

	got := sink.String()
	if !strings.Contains(got, "background task panicked") {
		t.Errorf("the panic was not reported; log was %q", got)
	}
	if !strings.Contains(got, "order dispatched") {
		t.Errorf("the worker did not survive the panic; log was %q", got)
	}
}

// The composition claim that is easiest to get wrong: the limiter is on the
// application chain only, so shedding user traffic never sheds a readiness probe
// or a scrape. A probe answered with 429 gets a healthy instance killed.
func TestOperationalEndpointsAreNotRateLimited(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimit, cfg.Burst = 1, 1
	_, _, h := newTestService(t, cfg)

	if rec := do(h, http.MethodPost, "/orders"); rec.Code != http.StatusAccepted {
		t.Fatalf("first order = %d, want %d", rec.Code, http.StatusAccepted)
	}
	// The bucket started full with one token and refills at one per second, so
	// the next request within the same millisecond is refused. No sleeping is
	// involved in either direction.
	if rec := do(h, http.MethodPost, "/orders"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second order = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		for i := range 5 {
			if rec := do(h, http.MethodGet, path); rec.Code != http.StatusOK {
				t.Errorf("%s request %d = %d, want %d — the limiter reaches an "+
					"operational endpoint", path, i, rec.Code, http.StatusOK)
			}
		}
	}
}

// A shed request is counted, because the rate of 429s is what tells an operator
// the limit is set wrong — and a scrape is not, because it is traffic the service
// never served for anyone.
func TestRecorderCountsSheddingAndIgnoresScrapes(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimit, cfg.Burst = 1, 1
	_, _, h := newTestService(t, cfg)

	do(h, http.MethodPost, "/orders") // 202
	do(h, http.MethodPost, "/orders") // 429
	do(h, http.MethodGet, "/metrics") // not counted

	body := do(h, http.MethodGet, "/metrics").Body.String()
	for _, want := range []string{
		`http_requests_total{code="202",method="POST"} 1`,
		`http_requests_total{code="429",method="POST"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `method="GET"`) {
		t.Errorf("the scrape counted itself; /metrics is inside the recorder:\n%s", body)
	}
}

// Cors sits closest to the handler, so the preflight it terminates is still a
// response the chain above it observes.
func TestPreflightIsAnsweredAndCounted(t *testing.T) {
	_, _, h := newTestService(t, testConfig())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/orders", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the configured origin", got)
	}
	if body := do(h, http.MethodGet, "/metrics").Body.String(); !strings.Contains(
		body, `http_requests_total{code="204",method="OPTIONS"} 1`,
	) {
		t.Errorf("the preflight was not counted:\n%s", body)
	}
}

// loadConfig is the one place the process reads the outside world, and env's
// contract is what makes it safe: a malformed value is "not configured" rather
// than a startup failure.
func TestLoadConfigFallsBackPerField(t *testing.T) {
	t.Setenv("SERVICE_ADDR", ":9090")
	t.Setenv("SERVICE_WORKERS", "16")
	t.Setenv("SERVICE_SHUTDOWN_GRACE", "not-a-duration")

	cfg := loadConfig()

	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":9090")
	}
	if cfg.Workers != 16 {
		t.Errorf("Workers = %d, want 16", cfg.Workers)
	}
	if cfg.ShutdownGrace != 15*time.Second {
		t.Errorf("ShutdownGrace = %v, want the 15s default: a malformed value is "+
			"not configuration", cfg.ShutdownGrace)
	}
	if cfg.QueueSize != 64 {
		t.Errorf("QueueSize = %d, want the 64 default", cfg.QueueSize)
	}
}
