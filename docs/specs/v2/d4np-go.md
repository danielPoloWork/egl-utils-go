# Software Specification: d4np-go (Go Concurrency & Backend Utilities Library)

| | |
|---|---|
| **Version** | 2.0 (addresses spec-review issue #8) |
| **Date** | 2026-07-14 |
| **Status** | Reviewed draft |
| **ADRs** | [ADR-001: Logger on slog](adr/d4np_go_adr_001_logger_slog.md) · [ADR-002: Wrap x/sync & x/time](adr/d4np_go_adr_002_wrap_x_packages.md) · [ADR-003: Dependency policy](adr/d4np_go_adr_003_dependency_policy.md) |

## 1. Description & Design Philosophy
`d4np-go` is a Go module providing advanced concurrency tooling, high-performance HTTP middleware, and production-API utilities.

Every principle below now names its **verification mechanism** — v1 asserted guarantees the document never made checkable:

| Principle | Meaning | Verified by |
|---|---|---|
| **Idiomatic Go** | channels, `context` cancellation, `error` values; no framework-isms | `golangci-lint` + `go vet` gates (§7) |
| **Zero goroutine leaks** | every internal goroutine stops via context/`Close()`; every type that starts one exposes a lifecycle | `go.uber.org/goleak` in every package's `TestMain` (§7) |
| **Race-free under concurrency** | all exported APIs safe per their §4 contract | full suite under `-race` in CI (§7) |
| **Allocation discipline** | `sync.Pool` reuse, zero-alloc hot paths | benchmarks with `B.ReportAllocs` and per-target gates (§5) |
| **Controlled import graph** | core depends on stdlib + `golang.org/x` only | dependency policy in [ADR-003](adr/d4np_go_adr_003_dependency_policy.md), enforced by a CI `go mod graph` check |

---

## 2. Functional Specification (25 items)

### Concurrency & Async Flows
1. **`workerpool.Pool`** — configurable goroutine pool with bounded task queue; `Submit(ctx, task)` blocks or fails fast per option; `Close()` drains then stops (goleak-verified).
2. **`pubsub.Broker`** — in-memory channel-based pub/sub with filtered subscriptions; slow-subscriber policy explicit (bounded per-subscriber buffer, drop-oldest or disconnect).
3. **`fanin.Merge`** — merges N input channels into one; terminates when all inputs close **or** the passed context is canceled (no orphan forwarders).
4. **`fanout.Split`** — distributes one source channel to N destinations in parallel; same cancellation contract as item 3.
5. **`semaphore.Weighted`** — thin wrapper of **`golang.org/x/sync/semaphore`** *(v1 cited a nonexistent stdlib `sync/semaphore`; there is no such package — source corrected, wrap-vs-reimplement decided in [ADR-002](adr/d4np_go_adr_002_wrap_x_packages.md))*.

### Resilience & Cloud Patterns
6. **`circuitbreaker.Breaker`** — circuit breaker for outbound calls; states/thresholds/half-open probe count configurable and observable.
7. **`retry.Backoff`** — retries with exponential backoff and full jitter; context-canceled aborts immediately.
8. **`ratelimit.Limiter`** — token bucket **wrapping `golang.org/x/time/rate`** *(v1 implied a reimplementation on raw timers; ADR-002 decides wrap — the stdlib-adjacent implementation is battle-tested and the wrapper only adds middleware ergonomics)*; burst accuracy gated by NFR-04.

### HTTP Middleware & REST Utilities
9. **`middleware.RequestID`** — extracts/generates a request ID into the context.
10. **`middleware.Logger`** — request logging (latency, bytes) via the §2.15 slog logger.
11. **`middleware.Recoverer`** — converts handler panics into clean 500s without killing the server.
12. **`middleware.Cors`** — robust, configurable CORS handling.

### Configuration & Environment
13. **`config.Loader`** — JSON/YAML/env configuration with struct validation (via item 19); parser is a fuzz target (§7).
14. **`env.GetDefault`** — env reads with safe fallbacks.

### Structured Logging
15. **`logger.Structured`** — **built on stdlib `log/slog`** (Go ≥ 1.21) with opinionated JSON defaults for Elastic/Loki ingestion *(v1 specified a custom JSON logger and never mentioned `slog`; build-vs-wrap decided in [ADR-001](adr/d4np_go_adr_001_logger_slog.md))*.
16. **`logger.Context`** — attach/extract logger fields through `context.Context` (`slog` attrs under the hood).

### Caching & Data Helpers
17. **`cache.InMemory`** — TTL map cache with a periodic **janitor goroutine** *(v1 said "cleanup thread" — Go has goroutines)* and an explicit lifecycle: `New(...) *Cache` starts the janitor, **`Close()` stops it**; the §1 zero-leak guarantee is verified for this type specifically by a goleak test that creates and closes 1 000 caches.
18. **`db.Transaction`** — runs a function inside a SQL transaction with automatic rollback on error or panic (re-panics after rollback).

### Validation & Security
19. **`validator.Struct`** — tag-based struct validation (`validate:"required,email"`).
20. **`hash.HashPassword` / `hash.CheckPassword`** — password **hashing** via bcrypt *(v1 said "encryption"; bcrypt is an adaptive one-way hash — corrected)*. Contract: configurable cost, **default 12** (min accepted 10); the **72-byte input limit** is surfaced, not hidden — `golang.org/x/crypto/bcrypt` returns `ErrPasswordTooLong` and this API propagates it (no silent truncation); doc note recommends `argon2id` (via `x/crypto/argon2`) for new systems with a stated migration path (algorithm tag in stored hash → verify-and-rehash on login).

### Diagnostics & Lifecycle
21. **`lifecycle.GracefulShutdown`** — coordinated shutdown of HTTP servers, DB pools, and queues on SIGINT/SIGTERM, with a **bounded deadline** (§6 example) and `Trigger()` for programmatic shutdown.
22. **`health.Handler`** — health endpoint; **DB/Redis probes live in separate submodules** (`contrib/…`) so the core never imports driver dependencies (ADR-003).
23. **`metrics.Prometheus`** — request latency/counter middleware in Prometheus exposition format.
24. **`syncpool.BufferPool`** — `sync.Pool`-managed `bytes.Buffer` reuse (NFR-05).
25. **`errx.Wrap`** — error utilities. Two corrections from v1: (a) the package is named **`errx`**, not `errors` — a package shadowing stdlib `errors` forces an alias at every call site; (b) the claim "without losing the original call stack" is restated accurately: **`%w` wrapping preserves the error *chain*, not stack traces**. Stack capture is explicit and opt-in: `errx.WithStack(err)` records `runtime.Callers` at the wrap point, exposed via `errx.StackTrace(err)`.

---

## 3. Architecture (C4 Component View — package layering & import policy)
```
 ┌─ module github.com/danielpolowork/d4np-go (core: stdlib + golang.org/x only) ─┐
 │                                                                               │
 │  L3  HTTP: middleware.* ─ health.Handler ─ metrics.Prometheus                 │
 │        │ may import ▼                                                         │
 │  L2  Services: workerpool ─ pubsub ─ circuitbreaker ─ retry ─ ratelimit ─     │
 │        cache ─ db ─ config ─ validator ─ hash ─ lifecycle                     │
 │        │ may import ▼                                                         │
 │  L1  Foundation: logger (slog) ─ errx ─ env ─ syncpool ─ fanin/fanout ─       │
 │        semaphore                                                              │
 │                                                                               │
 │  Import rules (CI-enforced via go mod graph + depguard):                      │
 │   • arrows point downward only; L1 imports nothing above stdlib/x             │
 │   • no package imports database drivers, redis clients, or prometheus SDK —   │
 │     metrics emits exposition format directly; probes live in contrib          │
 └───────────────────────────────────────────────────────────────────────────────┘
 ┌─ nested submodules (separate go.mod each — ADR-003) ──────────────────────────┐
 │  contrib/redishealth (imports go-redis) · contrib/pgxhealth (imports pgx)     │
 └───────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Per-Package API Contracts (core packages)

| Package | Key signatures | Concurrency safety | Context/cancellation | Error semantics |
|---|---|---|---|---|
| `workerpool` | `New(size, queue int, ...Option) *Pool` · `(p) Submit(ctx, func(ctx))` · `(p) Close() error` | `Pool` safe for concurrent `Submit` | `Submit` honors ctx while queue full; task funcs receive a ctx canceled on `Close` deadline | `ErrQueueFull` (fail-fast mode), `ErrClosed` |
| `pubsub` | `NewBroker[T](...Option)` · `Subscribe(ctx, filter) <-chan T` · `Publish(ctx, T) error` | fully concurrent | subscription auto-removed on ctx cancel | `ErrSlowSubscriber` per policy |
| `circuitbreaker` | `New(Settings) *Breaker` · `(b) Do(ctx, fn) error` | concurrent | open-state fails fast without calling fn | `ErrOpen` distinguishable via `errors.Is` |
| `retry` | `Do(ctx, policy, fn) error` | stateless | aborts between attempts on ctx cancel | returns last error wrapped with attempt count |
| `ratelimit` | `New(rate, burst) *Limiter` · `(l) Wait(ctx)` / `Allow()` / `Middleware()` | concurrent | `Wait` honors ctx deadline | `ErrLimited` on `Allow` middleware path |
| `cache` | `New[K comparable, V any](ttl, ...Option) *Cache[K,V]` · `Get/Set/Delete` · `Close()` | concurrent (sharded RWMutex) | janitor stops on `Close` | `Get` returns `(V, bool)` — absence is not an error |
| `lifecycle` | `Register(hook func(ctx) error)` · `Trigger()` · `WaitForSignals(timeout, ...os.Signal)` | `Register` safe until wait starts | hooks receive ctx with the bounded timeout | hook errors aggregated via `errors.Join` |
| `errx` | `Wrap(err, msg)` · `WithStack(err)` · `StackTrace(err) []Frame` | stateless | — | chain-compatible with `errors.Is/As` |

---

## 5. Non-Functional Requirements & Benchmark Methodology
**Methodology:** `go test -bench` with `B.ReportAllocs`, ≥ 10 runs compared via `benchstat` (p < 0.05), reference machine Ryzen 7 5800X, latest stable Go, pinned in `bench/README`. Nightly CI tracks results; > 10% regression fails.

| ID | Target |
|---|---|
| NFR-01 | Middleware chain (RequestID + Recoverer + Cors) adds ≤ 1 µs and **0 allocs/op** per request on the non-logging path; `middleware.Logger` adds ≤ 3 allocs/op |
| NFR-02 | `workerpool`: ≥ 1 M no-op tasks/s scheduled at 8 workers; `Submit` p99 ≤ 2 µs uncontended |
| NFR-03 | `pubsub`: ≥ 500 k msgs/s fan-out to 10 subscribers (64 B payload) |
| NFR-04 | `ratelimit`: token-bucket admission within ±1% of configured rate over a 10 s bursty pattern (burst = 2× rate) |
| NFR-05 | `syncpool.BufferPool`: **0 allocs/op** steady-state in the render-loop benchmark |
| NFR-06 | `cache.InMemory`: `Get` p99 ≤ 200 ns at 1 M entries, 90/10 read/write, 8 goroutines |

---

## 6. API Example (graceful shutdown — v1's example did not compile)
v1 imported `time` without using it (a hard compile error in Go), discarded the `ListenAndServe` error (a bind failure would block forever on signal wait), and showed no shutdown deadline. Corrected, `go build`-clean:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/danielpolowork/d4np-go/lifecycle"
)

func main() {
	server := &http.Server{Addr: ":8080"}

	lifecycle.Register(func(ctx context.Context) error {
		return server.Shutdown(ctx) // ctx carries the bounded deadline below
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed to start", "err", err)
			lifecycle.Trigger() // begin shutdown instead of blocking on signals forever
		}
	}()

	// Blocks until SIGINT/SIGTERM (or Trigger), then runs hooks with a 10 s deadline.
	lifecycle.WaitForSignals(10*time.Second, os.Interrupt, syscall.SIGTERM)
}
```

---

## 7. Verification, CI & Release
* **Static gates (per PR):** `golangci-lint` (incl. `depguard` enforcing the §3 import rules), `go vet`.
* **Tests:** full suite under **`-race`**; **goleak** in every package `TestMain` (the §1 zero-leak guarantee's teeth); coverage gate ≥ 85%.
* **Fuzzing:** native Go fuzz targets `FuzzConfigLoader` (JSON/YAML inputs) and `FuzzValidatorTags`; corpora committed; 10-min PR budget.
* **Security:** `govulncheck` per PR; bcrypt cost-factor benchmark documented so deployers can size it (NFR appendix).
* **Benchmarks:** §5 methodology, nightly with regression gate.
* **Supported versions:** the two most recent stable Go releases (matching the Go release policy); CI matrix runs both.
* **Versioning:** SemVer; a v2+ would follow Go's module major-version import-path rule (`/v2`). Nested `contrib/*` submodules version independently (ADR-003).

---

## 8. Decision Log
* [ADR-001 — Logger: build on stdlib `log/slog`, not a custom JSON logger](adr/d4np_go_adr_001_logger_slog.md)
* [ADR-002 — Wrap `golang.org/x/sync` and `golang.org/x/time/rate`, don't reimplement](adr/d4np_go_adr_002_wrap_x_packages.md)
* [ADR-003 — Zero-dependency core; driver-dependent probes as nested contrib submodules](adr/d4np_go_adr_003_dependency_policy.md)
