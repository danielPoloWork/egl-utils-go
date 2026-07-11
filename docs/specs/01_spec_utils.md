# Software Specification: Go Concurrency & Backend Utilities Library (Go 1.24+)

> Rendered from the intake interview (Phase 5). Frozen contract: diverging implementation
> updates this spec in the same PR or adds an ADR superseding the relevant section.

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
- Zero goroutine leaks: every goroutine-spawning component stops via context or close(done); per-component leak assertions (goleak) in tests
- Race-free: go test -race green in CI on every PR — the canonical concurrency gate
- Allocation-conscious hot paths: -benchmem benchmarks for pooled and middleware paths; syncpool.BufferPool asserts zero steady-state allocations via testing.AllocsPerRun
- Supply chain: govulncheck green; runtime deps limited to stdlib + golang.org/x/* + vetted few (prometheus/client_golang, a YAML parser); test-only deps: testify, goleak, rapid
- Portability: Tier-1 Linux/Windows/macOS; CI on Go 1.25 & 1.26; go.mod language floor 1.24
- Coverage: at least 80 percent line coverage enforced in CI
- Compatibility: SemVer, pre-1.0 milestone-driven; breaking changes to the public interface require a MAJOR-intent note in the PR


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
Packages live under the normative cross-language tree (src/main/go/it/d4np/utils);
go.mod placement reconciling consumer import ergonomics with that tree is decided and
recorded as an ADR in Milestone 1.

## 5. Public Interface

<!-- The API contract (the design "api" fold): each operation with its payload shapes, the error
     model (the failure taxonomy, not just the happy path), and the versioning / SemVer surface.
     A service/web project may keep the written-out contract under docs/api/ (capabilities.api_spec). -->
Consumers import via `import "github.com/danielPoloWork/egl-utils-go/workerpool"`. The public surface:

- workerpool: New(workers, queueSize int, opts ...Option) *Pool; (*Pool).Submit(ctx, Task) error; (*Pool).Stop(ctx) error; ErrQueueFull
- pubsub: NewBroker[T](opts ...Option) *Broker[T]; (*Broker[T]).Publish(topic string, msg T); (*Broker[T]).Subscribe(topic string, filter func(T) bool) (<-chan T, func())
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
Concurrency components additionally carry: a goleak assertion (no leaked goroutines
after Stop/Close/cancel), mandatory go test -race in CI, and deterministic clocks for
timing-sensitive logic (retry, ratelimit, cache TTL). Property-based tests (rapid) cover
pubsub delivery/filtering, fanin/fanout completeness (no message lost or duplicated),
and backoff bound invariants. Static gates on every PR: gofumpt, golangci-lint (govet,
staticcheck, errcheck, revive, gosec), govulncheck. Coverage gate: at least 80 percent
line coverage (go test -coverprofile). Benchmarks (go test -bench -benchmem) for
workerpool, ratelimit, middleware, and syncpool, recorded under docs/benchmarks;
syncpool.BufferPool asserts zero steady-state allocations via testing.AllocsPerRun.
Manual-only gates: none — every requirement above has a mechanical check.

Toolchain: built with go build (go modules), tested with go test (+ testify; rapid for property tests), checked with
go test -race (data-race detector), go vet, govulncheck, coverage target ≥ 80% line. Every functional and
non-functional requirement above maps to a CI gate (see [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)).
