// Command service is a runnable example: one small HTTP service composed from
// egl-utils-go, in its own module.
//
// Every package in the library carries runnable Example functions that show its
// own calls (ADR-0053). This shows what they cannot — the arrangement. Which
// middleware goes outside which, which endpoints stay out of the chain
// altogether, how a readiness probe reports something real, and the order the
// shutdown hooks have to be registered in so that resources close in the right
// sequence. Each of those is a decision about two packages at once, so it has no
// home in either package's documentation.
//
// It composes: env (configuration), logger (the base structured logger and the
// per-request context fields), middleware (Recoverer, RequestID, Logger, Cors),
// ratelimit (the 429 admission path), metrics (the scrape endpoint), health
// (liveness and readiness), workerpool (bounded background work), and lifecycle
// (ordered shutdown on a signal).
//
// Run it — no configuration required, every setting has a working default:
//
//	cd examples/service
//	go run .
//
//	curl -i -X POST localhost:8080/orders    # 202, with an X-Request-ID
//	curl -s localhost:8080/healthz           # liveness: is the process up
//	curl -s localhost:8080/readyz            # readiness: can it take work
//	curl -s localhost:8080/metrics           # what the middleware recorded
//	# then Ctrl-C, and watch the shutdown run its hooks in LIFO order
//
// This directory is a separate Go module on purpose, and the reason is not
// stylistic: a directory of .go files with no go.mod of its own would silently
// join the root module, and everything a showcase imports would land in the
// library's dependency graph while every `./...` check kept passing. See
// README.md and ADR-0054; tools/import_graph_lint.py enforces it.
package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"

	utils "github.com/danielPoloWork/egl-utils-go/v2"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/lifecycle"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/logger"
)

func main() {
	cfg := loadConfig()

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}

	// One base logger for the process — and it is also installed as slog's
	// default, which is not redundant. middleware.Recoverer logs a recovered
	// panic on slog.Default, and logger.FromContext builds on it too, so without
	// SetDefault the single most important line this service can emit would go
	// to stderr in a different format from every other line it writes.
	log := logger.NewStructured(
		logger.WithLevel(level),
		logger.WithAttrs(
			slog.String("service", serviceName),
			slog.String("egl_utils_version", utils.Version),
		),
	)
	slog.SetDefault(log)

	svc := newService(cfg, log)
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: svc.handler(),
		// A listener with no header deadline is a Slowloris target, which is why
		// gosec's G112 fires on a Server literal without ReadHeaderTimeout.
		// net/http's zero value is "no timeout" for all four of these, so a
		// public listener has to state them.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Hooks run in reverse registration order, so registration order is
	// dependency order and reads like a stack of defers: the pool is registered
	// first and therefore closed last.
	//
	// That direction is the whole point here. Stopping the server first means no
	// new request can enqueue work while the pool drains; closing the pool first
	// would leave the listener accepting orders it can only answer with a 503.
	lifecycle.Register(svc.pool.Close)
	lifecycle.Register(srv.Shutdown)

	go func() {
		log.Info("listening", slog.String("addr", cfg.Addr))
		// ErrServerClosed is the expected end: it is what Shutdown makes
		// ListenAndServe return, so treating it as a failure would report every
		// clean stop as one.
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			// Anything else — a port already in use, most often — must not leave
			// the process alive and silent. Trigger unblocks WaitForSignals
			// exactly as a signal would, so the failure takes the one shutdown
			// path this service has instead of inventing a second one.
			log.Error("listener stopped", slog.Any("error", err))
			lifecycle.Trigger()
		}
	}()

	// Blocks for the process's whole life, and owns no goroutine while it does.
	// The first argument bounds the entire shutdown sequence, measured from the
	// moment the signal arrives rather than from this call — so a service up for
	// a week still gets its full grace period. Passing 0 would impose no
	// deadline and leave the bound to the platform's kill escalation, which is
	// the better choice when a grace period is already configured there
	// (ADR-0051).
	lifecycle.WaitForSignals(cfg.ShutdownGrace, os.Interrupt, syscall.SIGTERM)

	// WaitForSignals returns only after every hook has run, so by here the
	// listener is closed and the pool is drained.
	log.Info("stopped")
}
