# Software Specification: Go Concurrency & Backend Utilities Library (Go 1.25+)

> Rendered from the intake interview (Phase 5). Frozen contract: diverging implementation
> updates this spec in the same PR or adds an ADR superseding the relevant section.
>
> **Amendments** (this document is frozen; every edit after the Phase 5 render is logged here):
>
> - *2026-07-27* — **language floor 1.24 → 1.25** in the title and in §3's portability line.
>   `go.mod` has declared `go 1.25.0` since the M9/M10 dependency work, so 1.24 no longer builds
>   the module and the stated floor was false. Toolchain fact only: no functional, API, or
>   behavioural contract is altered, and the v1.0.0 API-stability commitment is untouched.
>   Amended under this document's own divergence rule rather than by a superseding ADR, because
>   there is no decision to record — only a number that had gone stale. Ledgered in ROADMAP 11.1.
> - *2026-07-27* — **§3 and §6 coverage floor 80% → 85%, enforced per package.** (§6 states it
>   twice — the gate list and the toolchain line — and both were missed on the first pass; found by
>   the full read-through.) Raised by spec v2 §7 and
>   implemented in 10.9 / [ADR-0036](../adr/0036-coverage-gate.md); "per package" is the operative
>   half, since with most packages at 100% a module-wide 85% average could never fail. The stated
>   80% understated a gate that had been stricter for a milestone. ROADMAP 11.2.
> - *2026-07-27* — **§3 and §6: the goleak hedge removed.** Both said leak assertions used "an
>   in-repo stack-based guard" until ROADMAP 2.6 landed the test-only dependencies. 2.6 landed in
>   M2; `go.uber.org/goleak` has been the assertion mechanism ever since, so the hedge described a
>   state that had ended. ROADMAP 11.2.
> - *2026-07-27* — **§3 compatibility clause SUPERSEDED, not amended**, by
>   [ADR-0042](../adr/0042-post-1.0-compatibility-contract.md). This one is a governance change
>   rather than a stale fact — the pre-1.0 clause made a breaking change *mergeable with a note*,
>   while the v1.0.0 commitment makes it not mergeable into v1.x at all — so it takes the ADR
>   branch of the divergence rule and the original text is struck in place, not rewritten.
>   ROADMAP 11.2.

## 1. Objective & Business Context

Provide a production-ready Go utilities module — advanced concurrency primitives,
resilience patterns, high-performance HTTP middleware, and API-development helpers —
that removes boilerplate and correctness risk (goroutine leaks, GC pressure, unsafe
shutdown) from Go backend services. Design philosophy (imported from the brief):
idiomatic Go throughout (channels, context.Context, the error interface); zero goroutine
leaks — every internal goroutine stops deterministically via context or close(done);
allocation-conscious hot paths via pointer discipline and sync.Pool object reuse.

## 2. Functional Requirements

- workerpool.Pool — configurable goroutine pool with a bounded task queue and explicit Submit/Stop lifecycle
- pubsub.Broker — in-memory publish-subscribe broker over Go channels with filtered subscriptions
- fanin.Merge — merge multiple input channels into a single output channel without goroutine leaks
- fanout.Split — distribute messages from one source channel to multiple destination channels in parallel
- semaphore.Weighted — weighted task admission control (wrapper over golang.org/x/sync/semaphore)
- circuitbreaker.Breaker — circuit breaker guarding outbound HTTP calls (closed/open/half-open states)
- retry.Backoff — function execution with retry, exponential backoff, and random jitter
- ratelimit.Limiter — token-bucket rate limiter built on Go timers
- middleware.RequestID — extract or generate a unique request ID per HTTP call, stored in the request context
- middleware.Logger — HTTP request logging with response-time and bytes-written statistics
- middleware.Recoverer — recover panics in HTTP handlers, emit a clean 500, never crash the server
- middleware.Cors — robust, configurable CORS header handling
- config.Loader — load configuration from JSON/YAML files or environment variables, with string validation
- env.GetDefault — fast environment-variable reads with safe fallback values
- logger.Structured — JSON logger ready for ElasticSearch / Grafana Loki ingestion
- logger.Context — attach key logger fields to and read them from a context.Context
- cache.InMemory — map-backed local cache with per-entry TTL and a periodic cleanup goroutine
- db.Transaction — run SQL statements inside a transaction with automatic rollback on error or panic
- validator.Struct — tag-driven struct validation (e.g. validate:"required,email")
- hash.HashPassword / hash.CheckPassword — bcrypt password hashing and verification
- lifecycle.GracefulShutdown — coordinated shutdown of HTTP servers, databases, and queues on SIGINT/SIGTERM
- health.Handler — preconfigured health-check endpoint reporting the state of active connections (DB, Redis)
- metrics.Prometheus — middleware exposing standard latency and request-count metrics in Prometheus format
- syncpool.BufferPool — bytes.Buffer pool via sync.Pool to cut allocations for strings and temporary buffers
- errors.Wrap — attach context to errors while preserving the original call stack for tracing


## 3. Non-Functional Requirements

<!-- Scalability / load budgets belong here as NUMBERS, not adjectives (the design "scalability"
     fold): a value per hard NFR axis — throughput / concurrency, p99 latency, memory ceiling,
     target FPS, cold-start budget — each phrased so CI could prove a violation. -->
- Idiomatic Go: gofumpt-clean and golangci-lint (govet, staticcheck, errcheck, revive, gosec) green on every PR
- Zero goroutine leaks: every goroutine-spawning component stops via context or close(done); per-component leak assertions in tests (go.uber.org/goleak)
- Race-free: go test -race green in CI on every PR — the canonical concurrency gate
- Allocation-conscious hot paths: -benchmem benchmarks for pooled and middleware paths; syncpool.BufferPool asserts zero steady-state allocations via testing.AllocsPerRun
- Supply chain: govulncheck green; runtime deps limited to stdlib + golang.org/x/* + vetted few (prometheus/client_golang, a YAML parser); test-only deps: testify, goleak, rapid
- Portability: Tier-1 Linux/Windows/macOS; CI on Go 1.25 & 1.26; go.mod language floor 1.25
- Coverage: at least 85 percent line coverage enforced in CI **per package** (not as a module-wide average)
- Compatibility: ~~SemVer, pre-1.0 milestone-driven; breaking changes to the public interface require a MAJOR-intent note in the PR~~ — **SUPERSEDED by [ADR-0042](../adr/0042-post-1.0-compatibility-contract.md)**: the module is post-1.0, and under the v1.0.0 commitment a breaking change is not mergeable into v1.x with a note — it is deferred to the `/v2` ledger (ADR-0030 §2). The struck text is the pre-1.0 regime, retained as the historical contract.


## 4. Logical Architecture & Core Algorithm

<!-- For a non-obvious core algorithm, include a short LANGUAGE-FREE pseudocode sketch (control
     flow + invariants) alongside the prose + diagram (the design "pseudocode" fold); skip it when
     the approach is standard. If the design owns persistent state, capture the data model here —
     entities, relations, normal form, migration policy — within ADR-0004's secondary-SQL frame. -->
A flat collection of small, orthogonal packages — one concern per package — under a
single Go module (github.com/danielPoloWork/egl-utils-go). There is no cross-package
framework: packages compose only through stdlib contracts (context.Context,
net/http.Handler, error), so each is adoptable in isolation.

  concurrency:   workerpool | pubsub | fanin | fanout | semaphore
  resilience:    circuitbreaker | retry | ratelimit
  http:          middleware (RequestID, Logger, Recoverer, Cors) | health | metrics
  config/env:    config | env
  logging:       logger (Structured, Context)
  storage:       cache | db
  validation:    validator | hash
  lifecycle:     lifecycle (GracefulShutdown)
  core:          syncpool | errors

Concurrency components own their goroutines and stop deterministically (context /
close(done)); construction uses functional options for forward compatibility. HTTP
concerns follow the standard func(http.Handler) http.Handler decorator chain.
Packages live at the module root — one directory per feature package — per ADR-0003
(idiomatic Go root layout, superseding the cross-language tree for this repository);
go.mod sits at the repository root with module path github.com/danielPoloWork/egl-utils-go.

## 5. Public Interface

<!-- The API contract (the design "api" fold): each operation with its payload shapes, the error
     model (the failure taxonomy, not just the happy path), and the versioning / SemVer surface.
     A service/web project may keep the written-out contract under docs/api/ (capabilities.api_spec). -->
Consumers import via `import "github.com/danielPoloWork/egl-utils-go/workerpool"`. The public surface:

- workerpool: New(workers, queueSize int, opts ...Option) *Pool; (*Pool).Submit(ctx, Task) error; (*Pool).Stop(ctx) error; ErrQueueFull
- pubsub: NewBroker[T](opts ...Option) *Broker[T]; (*Broker[T]).Publish(topic string, msg T); (*Broker[T]).Subscribe(topic string, filter func(T) bool) (<-chan T, func()); (*Broker[T]).Close() — additive shutdown surface (ADR-0006)
- fanin: Merge[T](ctx, ins ...<-chan T) <-chan T
- fanout: Split[T](ctx, in <-chan T, outs ...chan<- T)
- semaphore: NewWeighted(capacity int64) *Weighted; Acquire(ctx, weight) error; Release(weight)
- circuitbreaker: New(opts ...Option) *Breaker; (*Breaker).Do(ctx, func() error) error; ErrOpen
- retry: Backoff(ctx, policy Policy, fn func(ctx) error) error — Policy{MaxAttempts, BaseDelay, MaxDelay, Jitter}
- ratelimit: NewLimiter(rate float64, burst int) *Limiter; (*Limiter).Allow() bool; (*Limiter).Wait(ctx) error
- middleware: RequestID(next http.Handler) http.Handler; RequestIDFrom(ctx) string
- middleware: Logger(l *slog.Logger) func(http.Handler) http.Handler — logs method, path, status, duration, bytes
- middleware: Recoverer(next http.Handler) http.Handler — 500 on panic, stack to the structured logger
- middleware: Cors(cfg CorsConfig) func(http.Handler) http.Handler
- config: Load[T any](path string, opts ...Option) (T, error) — JSON/YAML/env with validation
- env: GetDefault(key, fallback string) string; GetInt/GetBool/GetDuration variants
- logger: NewStructured(opts ...Option) *slog.Logger — JSON handler tuned for log aggregation
- logger: WithFields(ctx, ...Field) context.Context; FromContext(ctx) *slog.Logger
- cache: NewInMemory[K comparable, V any](ttl time.Duration, opts ...Option) *Cache[K, V]; Get/Set/Delete; Close(); ErrNotFound
- db: Transaction(ctx, db *sql.DB, fn func(*sql.Tx) error) error — commit on nil, rollback on error or panic
- validator: Struct(v any) error — tag grammar: required, email, min, max, oneof
- hash: HashPassword(pw string) (string, error); CheckPassword(pw, hash string) error
- lifecycle: Register(fn func(ctx) error); WaitForSignals(sig ...os.Signal); Shutdown(ctx) error
- health: Handler(checks ...Check) http.Handler — Check{Name, Probe func(ctx) error}
- metrics: Prometheus(reg prometheus.Registerer) func(http.Handler) http.Handler; Handler() http.Handler
- syncpool: NewBufferPool() *BufferPool; (*BufferPool).Get() *bytes.Buffer; (*BufferPool).Put(*bytes.Buffer)
- errors: Wrap(err error, msg string) error; Wrapf(err, format, args...) error — errors.Is/As/Unwrap compatible
- Error model: exported sentinel errors per package (ErrQueueFull, ErrOpen, ErrNotFound, ...); context cancellation surfaces ctx.Err(); all wrapping is errors.Is/As transparent
- Versioning surface: SemVer over all exported identifiers above; MAJOR = any breaking change to these signatures or their documented behavioral contracts


## 6. Verification & Test Strategy

Every functional requirement maps to package-level table-driven unit tests (go test);
the Spec Coverage Map in ROADMAP.md keeps one row per spec section (spec-map lint gate).
Concurrency components additionally carry: a leak assertion (no leaked goroutines
after Stop/Close/cancel, asserted with go.uber.org/goleak), mandatory go test -race in CI, and deterministic
clocks for timing-sensitive logic (retry, ratelimit, cache TTL). Property-based tests (rapid) cover
pubsub delivery/filtering, fanin/fanout completeness (no message lost or duplicated),
and backoff bound invariants. Static gates on every PR: gofumpt, golangci-lint (govet,
staticcheck, errcheck, revive, gosec), govulncheck. Coverage gate: at least 85 percent
line coverage per package (go test -coverprofile). Benchmarks (go test -bench -benchmem) for
workerpool, ratelimit, middleware, and syncpool, recorded under docs/benchmarks;
syncpool.BufferPool asserts zero steady-state allocations via testing.AllocsPerRun.
Manual-only gates: none — every requirement above has a mechanical check.

Toolchain: built with go build (go modules), tested with go test (+ testify; rapid for property tests), checked with
go test -race (data-race detector), go vet, govulncheck, coverage target ≥ 85% line per package. Every functional and
non-functional requirement above maps to a CI gate (see [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)).
