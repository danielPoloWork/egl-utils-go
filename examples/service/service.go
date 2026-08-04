package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/env"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/health"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/logger"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/metrics"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/ratelimit"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"
)

// serviceName is stamped on every log line, which is what makes lines from this
// process findable in an aggregator holding a hundred others.
const serviceName = "example-service"

// config is everything the service reads from its environment. Every field has a
// working default, which is what makes `go run .` enough to try it.
//
// This uses env rather than config.Load deliberately: a demo whose first step is
// "write a YAML file" is a demo the reader has to set up before it runs, and
// config.Load's own runnable examples already cover the file path. Note the rate
// limit's type — env has no float getter, so the limit is expressed in whole
// requests per second, which is how an operator states it anyway.
type config struct {
	Addr          string
	Debug         bool
	Workers       int
	QueueSize     int
	RateLimit     int
	Burst         int
	AllowedOrigin string
	ShutdownGrace time.Duration
}

// loadConfig reads the environment. Every getter here returns its fallback for
// an unset, empty *or* malformed value, so a typo in one variable degrades that
// one setting to its default instead of failing the process at startup.
func loadConfig() config {
	return config{
		Addr:          env.GetDefault("SERVICE_ADDR", ":8080"),
		Debug:         env.GetBool("SERVICE_DEBUG", false),
		Workers:       env.GetInt("SERVICE_WORKERS", 4),
		QueueSize:     env.GetInt("SERVICE_QUEUE_SIZE", 64),
		RateLimit:     env.GetInt("SERVICE_RATE_LIMIT", 20),
		Burst:         env.GetInt("SERVICE_BURST", 40),
		AllowedOrigin: env.GetDefault("SERVICE_ALLOWED_ORIGIN", "https://app.example.com"),
		ShutdownGrace: env.GetDuration("SERVICE_SHUTDOWN_GRACE", 15*time.Second),
	}
}

// service holds what the process owns for its whole life. Each field is
// constructed once at wiring time and shared by every request — that is the
// contract the library's components are built for, and constructing a Recorder
// or a Limiter per request would silently reset the state they exist to keep.
type service struct {
	cfg     config
	log     *slog.Logger
	pool    *workerpool.Pool
	metrics *metrics.Recorder
	limiter *ratelimit.Limiter
}

func newService(cfg config, log *slog.Logger) *service {
	return &service{
		cfg: cfg,
		log: log,
		// Non-blocking submission is the only defensible choice on a request
		// path. The blocking default would park the request's goroutine — and
		// its connection — until the queue drains, so a saturated pool would
		// convert into an unbounded backlog of held connections. Failing fast
		// converts it into a 503 the caller can retry.
		pool: workerpool.New(cfg.Workers, cfg.QueueSize,
			workerpool.WithNonBlockingSubmit(),
			// Without a handler the pool lets a task's panic propagate, which
			// takes the process down. That is the right default for a library —
			// an unobserved bug stays loud — and the wrong one for a long-lived
			// service, where one bad order should not stop the other workers.
			// Installing a handler is how a service opts into surviving it, and
			// the handler's job is to make sure the bug is still not silent.
			workerpool.WithPanicHandler(func(recovered any) {
				log.Error("background task panicked", slog.Any("panic", recovered))
			}),
		),
		metrics: metrics.New(),
		limiter: ratelimit.NewLimiter(float64(cfg.RateLimit), cfg.Burst),
	}
}

// handler builds the whole HTTP surface: the application endpoints behind the
// full middleware chain, and the operational endpoints deliberately outside it.
//
// Splitting the two is the composition decision no package's documentation can
// make, because each package is correct in isolation and only their arrangement
// can be wrong.
func (s *service) handler() http.Handler {
	app := http.NewServeMux()
	app.HandleFunc("POST /orders", s.createOrder)

	root := http.NewServeMux()

	// --- operational endpoints -----------------------------------------------
	//
	// These are not behind the application chain, and every omission is a
	// decision rather than an oversight:
	//
	//   - not rate-limited: a limiter that answers 429 to a readiness probe
	//     makes an orchestrator kill a healthy instance, and a scrape must not
	//     compete with user traffic for the same token bucket;
	//   - not counted: a 15-second scrape adds ~5 800 requests a day to
	//     http_requests_total that the service never served for anyone;
	//   - not logged: a probe every 10 seconds and a scrape every 15 produce
	//     ~14 000 lines a day that correlate with no user request;
	//   - not CORS-negotiated: no browser calls them.
	//
	// Recoverer stays on all three, because a panicking probe must not take the
	// process down — and if it does panic, the 500 is the signal.
	//
	// Liveness and readiness are separate handlers, and the split matters more
	// than it looks: /healthz carries no checks at all, so it answers "this
	// process is running". Giving it the dependency probes would let one
	// dependency's blip restart every instance at once, turning a partial
	// outage into a restart storm. /readyz is where dependencies belong,
	// because its failure removes one instance from the load balancer.
	root.Handle("/healthz", middleware.Recoverer(health.Handler()))
	root.Handle("/readyz", middleware.Recoverer(health.Handler(s.readiness())))
	root.Handle("/metrics", middleware.Recoverer(s.metrics.Handler()))

	// --- application endpoints -----------------------------------------------
	//
	// Outermost first, and each position is load-bearing:
	//
	//	Recoverer   outside everything, so a panic anywhere inside — including
	//	            inside another middleware — still becomes a 500;
	//	RequestID   next, so the id exists before anything logs it;
	//	Logger      outside the recorder and the limiter, so a 429 is logged;
	//	metrics     outside the limiter, so a shed request is counted — the
	//	            rate of 429s is exactly what tells you the limit is wrong;
	//	limiter     outside the router, so shedding costs no routing work;
	//	Cors        closest to the handler, so the terminal 204 it writes for a
	//	            preflight is logged and counted like any other response.
	//
	// The chain wraps the application's mux rather than each route, so a 404 or
	// a 405 the mux produces is logged and counted too. Wrapping per route would
	// leave the mux's own responses invisible.
	root.Handle("/", middleware.Recoverer(
		middleware.RequestID(
			middleware.Logger(s.log)(
				s.metrics.Middleware()(
					s.limiter.Middleware()(
						middleware.Cors(middleware.CorsConfig{
							AllowedOrigins: []string{s.cfg.AllowedOrigin},
							AllowedMethods: []string{http.MethodGet, http.MethodPost},
							AllowedHeaders: []string{"Content-Type"},
							MaxAge:         10 * time.Minute,
						})(app),
					),
				),
			),
		),
	))

	return root
}

// readiness reports whether this instance can accept work right now, and it
// answers by exercising the real admission path instead of returning nil: it
// submits a no-op task through the same Submit the request handler uses. That
// makes the endpoint answer the question an orchestrator is actually asking, and
// the pool's own errors are already the three answers it needs —
//
//	ErrClosed     shutdown has begun; stop routing traffic here
//	ErrQueueFull  this instance is saturated; route elsewhere
//	nil           it can take work
//
// A probe that returns nil unconditionally documents the wiring and verifies
// nothing. A probe for an out-of-process dependency needs that dependency's
// driver, which the core module is not allowed to import — that is what the
// contrib/* modules are for (see ../../contrib).
func (s *service) readiness() health.Check {
	return health.Check{
		Name: "worker-pool",
		Probe: func(ctx context.Context) error {
			return s.pool.Submit(ctx, func(context.Context) {})
		},
	}
}

// createOrder accepts an order and hands the slow part to the worker pool.
func (s *service) createOrder(w http.ResponseWriter, r *http.Request) {
	// RequestIDFrom and logger.WithFields are where two packages that know
	// nothing about each other meet: the middleware put the id in the request's
	// context, the logger package carries it onto every line this request
	// produces. Neither package imports the other; the consumer joins them.
	id := middleware.RequestIDFrom(r.Context())
	log := logger.FromContext(logger.WithFields(r.Context(), logger.String("request_id", id)))

	// The task must not capture r.Context(). That context is canceled the moment
	// the handler returns, so a background task built on it would be canceled
	// exactly when it started to matter. The pool hands the task its own
	// context; anything else the task needs is captured by value, as `id` is
	// here. This is the trap a worker pool behind an HTTP handler always sets.
	err := s.pool.Submit(r.Context(), func(ctx context.Context) {
		s.log.LogAttrs(ctx, slog.LevelInfo, "order dispatched", slog.String("request_id", id))
	})
	switch {
	case errors.Is(err, workerpool.ErrQueueFull):
		// Saturated, not broken. Say so, and say when to come back.
		w.Header().Set("Retry-After", "1")
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	case errors.Is(err, workerpool.ErrClosed):
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	case err != nil:
		log.Error("submit failed", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Info("order accepted")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if _, err := io.WriteString(w, "{\"status\":\"accepted\"}\n"); err != nil {
		// The client hung up mid-response. Nothing to do but record it: the
		// status line is already on the wire, so this cannot become a 500.
		log.Warn("response write failed", slog.Any("error", err))
	}
}
