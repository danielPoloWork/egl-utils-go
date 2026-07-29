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
> - *2026-07-27* — **§5 reconciled with the module's actual exported surface, and its versioning
>   clause widened.** §5 had never been updated after Milestone 10, so twelve identifiers shipped in
>   v1.1.0 were absent (`(*Breaker).State` and the `State` type, `lifecycle.Trigger`,
>   `(*Limiter).Middleware` and `ErrLimited`, `HashPasswordCost`/`Cost`/`ErrInvalidCost`,
>   `config.WithStructValidation`, `pubsub.WithDropOldest`), along with pre-M10 omissions
>   (`middleware.HeaderName`, `errors.StackTracer`, `validator.ValidationErrors`/`FieldError`,
>   `logger.Field` and its constructors, `workerpool.Task`, every `WithX` option constructor, and
>   the root `Version`). One listed signature was **wrong**: `NewBroker[T](opts ...Option)` — the
>   real option type is generic, `Option[T]`, so code written to the spec would not compile.
>   Rebuilt from `go doc` output rather than by hand. **The closing versioning clause said "SemVer
>   over all exported identifiers *above*", which made the enumeration the boundary of the promise
>   and so left every unlisted identifier outside it — narrower than the v1.0.0 changelog's "API
>   stability for every exported identifier". It now binds the whole exported surface and states
>   that the list is a map, not the boundary**, which widens the spec to match what was already
>   published rather than changing the promise. ROADMAP 12.1.
> - *2026-07-27* — **§4's "packages compose only through stdlib contracts … adoptable in
>   isolation" SUPERSEDED** by [ADR-0033](../adr/0033-config-struct-validation.md). `go list` shows
>   `config` imports `validator` — the module's one internal package edge, mandated by spec item 13
>   (validation inside configuration loading) and live since 10.6. Struck in place rather than
>   rewritten, per the 11.2 precedent for a replaced rule, but **no new ADR**: ADR-0033 already
>   holds the decision, so this is a pointer to it. Worth stating precisely, because the edge is a
>   *governed exception*, not an erosion — `import_graph_lint.py` fails both if an unsanctioned
>   edge appears and if this one disappears (ADR-0035). ROADMAP 12.2.
> - *2026-07-27* — **§6's test-strategy counts and §3's dependency list corrected.** rapid was
>   credited with three areas and runs in **eight** packages (adds circuitbreaker, middleware,
>   ratelimit, validator); benchmarks were credited to four packages and exist in **seven** (adds
>   cache, hash, pubsub); §3's dependency sentence omitted `prometheus/client_model`, a direct
>   `go.mod` require that no non-test file imports. Understatements rather than false claims, but a
>   spec that under-describes its own gates invites re-deriving them. ROADMAP 12.2.
> - *2026-07-27* — **§4 layout and §5 import path updated for v2.0.0**
>   ([ADR-0045](../adr/0045-pkg-layout-and-v2.md)). Feature packages move from the module root to
>   `pkg/`, and the module opens its second major:
>   `import "github.com/danielPoloWork/egl-utils-go/v2/pkg/<component>"`. The twenty-one-package
>   root had grown to forty entries; `pkg/` costs a consumer 7 characters where the series' Maven
>   tree — built, measured and reverted — costs 34, for the same tidy root. **This document is
>   amended rather than replaced by a v2 spec**: it has been the module's live contract since
>   intake, it already carries the amendment mechanism, and forking it would leave two documents
>   disagreeing about one codebase. Each remaining Milestone 13 item amends §5 again for its own
>   API change; `tools/spec_api_lint.py` (ADR-0043) makes that mechanical rather than remembered.
>   ROADMAP 13.1.
> - *2026-07-27* — **§5 gains `workerpool.ErrPoolClosed`, and §5 is now mechanically gated**
>   ([ADR-0043](../adr/0043-spec-api-lint.md)). The identifier was found by `tools/spec_api_lint.py`
>   on its first run, having survived both Milestone 10 and the M11 read-through: it is the *second*
>   member of a `var (…)` block whose first member, `ErrQueueFull`, was listed, so every scan that
>   looked at column-zero declarations skipped it. From this entry on, §5 and `go doc` disagreeing
>   is a red build in both directions — an exported identifier missing from §5, or §5 naming one
>   the module no longer exports. ROADMAP 12.3.
> - *2026-07-28* — **the `errors` package becomes `errx`, and stack capture becomes opt-in.
>   SUPERSEDED, not amended**, by [ADR-0046](../adr/0046-errx-opt-in-stacks.md), which supersedes
>   [ADR-0029](../adr/0029-errors-wrap-design.md). Three changes, all breaking, all inside the
>   `/v2` boundary ([ADR-0030](../adr/0030-spec-v2-reconciliation.md) §2, item 25): the package no
>   longer shadows `errors` from the standard library; `Wrap`/`Wrapf` no longer capture a call
>   stack, which is now requested explicitly with `WithStack`; and a trace reads as `[]Frame`
>   through `Frames`, so no consumer handles program counters or imports `runtime`. §2's feature
>   line and §4's `core` layer are updated for the rename. **§5 is rewritten in place rather than
>   struck**, unlike 11.2's compatibility clause: §5 is a mechanical mirror of `go doc` that
>   `tools/spec_api_lint.py` compares identifier by identifier, so a struck-through signature would
>   either fail the gate or be counted twice. The *decision* lives in the ADR; §5 records only what
>   the module exports today. ROADMAP 13.2.

> - *2026-07-28* — **`cache.Get` returns `(V, bool)`, `NewInMemory` becomes `New`, and
>   `ErrNotFound` is removed. SUPERSEDED, not amended**, by
>   [ADR-0047](../adr/0047-cache-comma-ok.md), which supersedes **only the `Get` signature and
>   constructor name** of [ADR-0021](../adr/0021-cache-inmemory-design.md) — the rest of that ADR,
>   including the Get-enforced expiry model this depends on, stands unchanged. `/v2` boundary
>   ([ADR-0030](../adr/0030-spec-v2-reconciliation.md) §2, item 17). A cache miss is an ordinary
>   outcome, not a failure, so it is reported comma-ok as Go readers expect; §5's error-model line no
>   longer cites `ErrNotFound` among the sentinels, because **nothing returns it and an exported
>   sentinel no code path produces is a promise with no behaviour behind it**. Absence and expiry
>   remain deliberately indistinguishable — they were already one outcome under the error API, and
>   separating them would promise something about eviction timing. ROADMAP 13.3.

> - *2026-07-29* — **`workerpool.Stop` becomes `Close` and `ErrPoolClosed` becomes `ErrClosed`.
>   SUPERSEDED, not amended**, by [ADR-0048](../adr/0048-workerpool-close.md), which supersedes
>   **only those two names** in [ADR-0005](../adr/0005-workerpool-design.md) — every semantic that ADR
>   decided is preserved. `/v2` boundary ([ADR-0030](../adr/0030-spec-v2-reconciliation.md) §2,
>   item 1). Two of the module's three goroutine-owning types already said `Close`; `workerpool` was
>   the outlier, and a consumer wiring all three had to remember which one was not. **`Close` keeps
>   its `ctx`**, against the literal reading of the v2 gap-analysis target `Close() error`: that
>   table's own gap column flags the method and sentinel names and never the parameter, and dropping
>   the context would make a `Close` that waits on caller-supplied tasks choose between an unbounded
>   wait and a hidden timeout — which [ADR-0025](../adr/0025-lifecycle-shutdown-design.md) refused on
>   purpose. The vocabulary is therefore uniform on the verb and deliberately not on the signature:
>   the pool is the only shutdown in the module that waits for work the caller wrote. Note the
>   accepted cost — **`*Pool` does not satisfy `io.Closer`**. ROADMAP 13.4.

> - *2026-07-29* — **the pubsub API reshaped: context-scoped subscriptions, `Publish(ctx, …) error`,
>   `ErrSlowSubscriber`, and an explicit slow-subscriber policy. SUPERSEDED, not amended**, by
>   [ADR-0049](../adr/0049-pubsub-reshape.md), which supersedes the two signatures and the fixed
>   drop-newest policy of [ADR-0006](../adr/0006-pubsub-design.md) — every *invariant* that ADR
>   decided stands — and the option name of [ADR-0039](../adr/0039-pubsub-drop-oldest.md), whose
>   reasoning it absorbs rather than replaces. `/v2` boundary
>   ([ADR-0030](../adr/0030-spec-v2-reconciliation.md) §2, item 2). This is the milestone's largest
>   entry and the only one reopening an *invariant* rather than a name: **"Publish never blocks" was
>   previously argued from the absence of a context and an error return, and is now an explicit
>   promise instead.** The new parameters are given narrow meanings so they cannot erode it — `ctx`
>   is consulted **once, before anything is delivered** (so a cancelled publish delivers to nobody,
>   all-or-nothing rather than a partial fan-out) and is never waited on; the `error` reports what
>   already happened and never that the publisher waited. `Subscribe` returns only a channel, since
>   cancelling the context *is* the unsubscribe; `context.AfterFunc` rather than a goroutine per
>   subscription is what keeps ADR-0006's "the broker owns no goroutines" literally true.
>   `WithDropOldest[T]()` becomes one value of `SlowSubscriberPolicy`, which the gap analysis
>   requires to carry **`Disconnect`** as well — an enum because two booleans would make an illegal
>   combination expressible. **`topic` is kept** against the ledger's literal `Subscribe(ctx, filter)`,
>   on the 13.4 tie-breaker: the gap column flags the API *shape* and never mentions topics, and
>   removing topic routing would be a feature deletion wearing a signature change's clothes.
>   ROADMAP 13.5.

> - *2026-07-29* — **the Prometheus SDK is gone from the module: `metrics` writes the exposition
>   format directly, and §3's dependency budget loses a ring-3 entry. SUPERSEDED, not amended**, by
>   [ADR-0050](../adr/0050-metrics-without-the-sdk.md), which supersedes the surface, the
>   implementation and the dependency pin of [ADR-0027](../adr/0027-metrics-prometheus-design.md) —
>   **that ADR's cardinality decisions stand and are re-enforced there** — and ring 3's membership in
>   [ADR-0004](../adr/0004-runtime-dependency-policy.md), whose count of "exactly two runtime
>   entries" is struck in place. `/v2` boundary
>   ([ADR-0030](../adr/0030-spec-v2-reconciliation.md) §2, item 23). **This is the only Milestone 13
>   item that changes the dependency graph, and the change is larger than one module: nine of
>   eighteen left** (`client_golang`, `client_model` and the seven transitive requirements that
>   existed only to serve them — `go.sum` 50 lines → 24). §5's metrics line becomes
>   `New() *Recorder` with `Middleware()` and `Handler()` methods: the `prometheus.Registerer`
>   parameter becomes ownership, which also closes a wart ADR-0027 had recorded rather than wanted —
>   `Prometheus(myReg)` paired with a package-level `Handler()` served the *default* registry, so any
>   custom registry silently exposed the wrong metrics. **The cost is stated rather than discovered:
>   the endpoint no longer serves the 37 metric families `promhttp` supplied free** (29 `go_*`, 6
>   `process_*`, 2 `promhttp_*` — measured, not assumed), so a consumer wanting runtime metrics
>   imports a client library itself and mounts it at a second path, which puts those nine modules in
>   the build that asked for them instead of in every consumer's. Conformance to the format is
>   pinned by a golden file **captured from the reference encoder while it was still a dependency**,
>   so the check outlives the library. ROADMAP 13.6.

> - *2026-07-29* — **`WaitForSignals` takes a shutdown timeout. SUPERSEDED, not amended**, by
>   [ADR-0051](../adr/0051-lifecycle-shutdown-timeout.md), which supersedes **two points** of
>   [ADR-0025](../adr/0025-lifecycle-shutdown-design.md) — its "no hidden timeout" decision and its
>   rejected default-timeout alternative — and **nothing else**. `/v2` boundary
>   ([ADR-0030](../adr/0030-spec-v2-reconciliation.md) §2, item 21). **This is the one Milestone 13
>   item that overturns a deviation the module took deliberately and documented as such**, so the
>   ROADMAP required it be overturned explicitly rather than quietly. What makes that possible is
>   that ADR-0025's objections were narrower than its heading: it refused a deadline that was
>   *hidden*, *library-invented*, *default* and would *silently truncate* what the operator
>   configured — every one of those adjectives describing a deadline **the caller did not choose**.
>   A mandatory first parameter is none of them, so **the reasoning is preserved and only the
>   conclusion reverses**. Its substantive claim survives too: **`0` imposes no deadline**, which is
>   byte-for-byte v1's behaviour and remains the *recommended* posture under systemd or Kubernetes,
>   where a second number in the application would duplicate a grace period already configured one
>   level up and the shorter of the two would silently win. Note the gap column here reads
>   "bounded-deadline **philosophy**", unlike 13.4's and 13.5's, which named only signatures — the
>   same tie-breaker that protected `Close`'s `ctx` and `Subscribe`'s `topic` points, here, at making
>   the change. The bound is **cooperative and does not abandon the sequence**: a hook honouring its
>   context winds up early, one ignoring it still completes, and the remaining hooks run either way,
>   because ADR-0025's run-every-hook decision is untouched. ROADMAP 13.7.

## 1. Objective & Business Context

Provide a production-ready Go utilities module — advanced concurrency primitives,
resilience patterns, high-performance HTTP middleware, and API-development helpers —
that removes boilerplate and correctness risk (goroutine leaks, GC pressure, unsafe
shutdown) from Go backend services. Design philosophy (imported from the brief):
idiomatic Go throughout (channels, context.Context, the error interface); zero goroutine
leaks — every internal goroutine stops deterministically via context or close(done);
allocation-conscious hot paths via pointer discipline and sync.Pool object reuse.

## 2. Functional Requirements

- workerpool.Pool — configurable goroutine pool with a bounded task queue and explicit Submit/Close lifecycle
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
- errx.Wrap — attach context to errors; opt-in call-stack capture via WithStack preserves the original failure site for tracing


## 3. Non-Functional Requirements

<!-- Scalability / load budgets belong here as NUMBERS, not adjectives (the design "scalability"
     fold): a value per hard NFR axis — throughput / concurrency, p99 latency, memory ceiling,
     target FPS, cold-start budget — each phrased so CI could prove a violation. -->
- Idiomatic Go: gofumpt-clean and golangci-lint (govet, staticcheck, errcheck, revive, gosec) green on every PR
- Zero goroutine leaks: every goroutine-spawning component stops via context or close(done); per-component leak assertions in tests (go.uber.org/goleak)
- Race-free: go test -race green in CI on every PR — the canonical concurrency gate
- Allocation-conscious hot paths: -benchmem benchmarks for pooled and middleware paths; syncpool.BufferPool asserts zero steady-state allocations via testing.AllocsPerRun
- Supply chain: govulncheck green; runtime deps limited to stdlib + golang.org/x/* + one vetted third party (a YAML parser, `gopkg.in/yaml.v3`, for config.Load); test-only deps: testify, goleak, rapid. ADR-0050 removed both Prometheus modules with the /v2 major - `client_golang` from the runtime ring and `client_model` from the test-only ring - taking nine of the module's eighteen modules with them, since seven transitive requirements existed only to serve the metrics SDK
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
framework: ~~packages compose only through stdlib contracts (context.Context,
net/http.Handler, error), so each is adoptable in isolation.~~ — **the absolute is
SUPERSEDED by [ADR-0033](../adr/0033-config-struct-validation.md)**: packages compose
through stdlib contracts (context.Context, net/http.Handler, error) with **exactly one
sanctioned internal edge**, `config → validator`, which spec item 13 mandates by
requiring validation inside configuration loading. The edge is not an erosion of the
rule but a governed exception to it: `tools/import_graph_lint.py` fails both when an
unsanctioned edge appears **and when this one disappears**, and the rule it enforces is
"same-layer edges only where the spec mandates the composition", not "L2 is a
free-for-all" (ADR-0035). Every other package remains adoptable in isolation; adopting
`config` also brings `validator`, which costs nothing since both ship in this module.

  concurrency:   workerpool | pubsub | fanin | fanout | semaphore
  resilience:    circuitbreaker | retry | ratelimit
  http:          middleware (RequestID, Logger, Recoverer, Cors) | health | metrics
  config/env:    config | env
  logging:       logger (Structured, Context)
  storage:       cache | db
  validation:    validator | hash
  lifecycle:     lifecycle (GracefulShutdown)
  core:          syncpool | errx

Concurrency components own their goroutines and stop deterministically (context /
close(done)); construction uses functional options for forward compatibility. HTTP
concerns follow the standard func(http.Handler) http.Handler decorator chain.
Packages live under `pkg/` — one directory per feature package — per
[ADR-0045](../adr/0045-pkg-layout-and-v2.md) (superseding ADR-0003's root layout and, through
it, the cross-language tree); go.mod sits at the repository root with module path
github.com/danielPoloWork/egl-utils-go/v2, and module metadata (doc.go, version.go) stays
beside it rather than under `pkg/`.

## 5. Public Interface

<!-- The API contract (the design "api" fold): each operation with its payload shapes, the error
     model (the failure taxonomy, not just the happy path), and the versioning / SemVer surface.
     A service/web project may keep the written-out contract under docs/api/ (capabilities.api_spec). -->
Consumers import via `import "github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"`, and the
module root as `import "github.com/danielPoloWork/egl-utils-go/v2"` for `utils.Version`. The public surface:

- utils (root): const Version — the released version, kept in lockstep with the tag and the changelog
- workerpool: New(workers, queueSize int, opts ...Option) *Pool; (*Pool).Submit(ctx, Task) error; (*Pool).Close(ctx) error; WithNonBlockingSubmit() Option; WithPanicHandler(func(recovered any)) Option; Task func(ctx); ErrQueueFull; ErrClosed
- pubsub: NewBroker[T any](opts ...Option[T]) *Broker[T] — note the option type is **generic**; (*Broker[T]).Publish(ctx, topic string, msg T) error — never blocks; the error reports what was already lost (ADR-0049); (*Broker[T]).Subscribe(ctx, topic string, filter func(T) bool) <-chan T — the context is the subscription's lifetime, so cancel is the unsubscribe; (*Broker[T]).Close(); WithSubscriberBuffer[T](n int) Option[T]; WithDropHandler[T](func(topic string, msg T)) Option[T]; WithSlowSubscriberPolicy[T](p SlowSubscriberPolicy) Option[T]; SlowSubscriberPolicy int with DropNewest/DropOldest/Disconnect (ADR-0049, absorbing ADR-0039); ErrSlowSubscriber; ErrClosed
- fanin: Merge[T any](ctx, ins ...<-chan T) <-chan T
- fanout: Split[T any](ctx, in <-chan T, outs ...chan<- T)
- semaphore: NewWeighted(capacity int64) *Weighted; (*Weighted).Acquire(ctx, weight int64) error; (*Weighted).Release(weight int64)
- circuitbreaker: New(opts ...Option) *Breaker; (*Breaker).Do(ctx, func() error) error; (*Breaker).State() State (ADR-0030); State uint8 with StateClosed/StateOpen/StateHalfOpen and String(); WithFailureThreshold(n int) Option; WithOpenTimeout(d time.Duration) Option; WithSuccessThreshold(n int) Option; ErrOpen
- retry: Backoff(ctx, policy Policy, fn func(ctx) error) error — Policy{MaxAttempts, BaseDelay, MaxDelay, Jitter}
- ratelimit: NewLimiter(rate float64, burst int) *Limiter; (*Limiter).Allow() bool; (*Limiter).Wait(ctx) error; (*Limiter).Middleware() func(http.Handler) http.Handler (ADR-0031); ErrLimited
- middleware: RequestID(next http.Handler) http.Handler; RequestIDFrom(ctx) string; const HeaderName
- middleware: Logger(l *slog.Logger) func(http.Handler) http.Handler — logs method, path, status, duration, bytes
- middleware: Recoverer(next http.Handler) http.Handler — 500 on panic, stack to the structured logger
- middleware: Cors(cfg CorsConfig) func(http.Handler) http.Handler — CorsConfig{AllowedOrigins, AllowedMethods, AllowedHeaders, ExposedHeaders []string, AllowCredentials bool, MaxAge time.Duration}
- config: Load[T any](path string, opts ...Option) (T, error) — JSON/YAML/env with validation; WithStructValidation() Option (ADR-0033); WithoutEnvExpansion() Option; Validator interface{ Validate() error }; ErrUnsupportedFormat
- env: GetDefault(key, fallback string) string; GetInt/GetBool/GetDuration variants
- logger: NewStructured(opts ...Option) *slog.Logger — JSON handler tuned for log aggregation; WithWriter(io.Writer) Option; WithLevel(slog.Leveler) Option; WithSource() Option; WithAttrs(...slog.Attr) Option
- logger: WithFields(ctx, ...Field) context.Context; FromContext(ctx) *slog.Logger; Field = slog.Attr (alias) with String/Int/Bool/Duration/Any constructors
- cache: New[K comparable, V any](ttl time.Duration, opts ...Option) *Cache[K, V]; Get(key K) (V, bool) — comma-ok, false when absent or expired; Set/Delete; Close(); WithCleanupInterval(d time.Duration) Option (ADR-0047, supersedes ADR-0021's Get signature)
- db: Transaction(ctx, db *sql.DB, fn func(*sql.Tx) error) error — commit on nil, rollback on error or panic
- validator: Struct(v any) error — tag grammar: required, email, min, max, oneof; ValidationErrors []*FieldError with Error() and Unwrap() []error; FieldError{Field, Tag, Param string}
- hash: HashPassword(pw string) (string, error); HashPasswordCost(pw string, cost int) (string, error) (ADR-0032); CheckPassword(pw, hash string) error; Cost(hash string) (int, error); ErrMismatch; ErrPasswordTooLong; ErrInvalidCost
- lifecycle: Register(fn func(ctx) error); WaitForSignals(timeout time.Duration, sigs ...os.Signal) — timeout bounds the whole shutdown sequence and becomes the hooks' context deadline, measured from the signal; 0 imposes none and leaves the bound to the platform (ADR-0051); Shutdown(ctx) error; Trigger() (ADR-0030)
- health: Handler(checks ...Check) http.Handler — Check{Name string, Probe func(ctx) error}
- metrics: New() *Recorder; (*Recorder).Middleware() func(http.Handler) http.Handler; (*Recorder).Handler() http.Handler — the exposition endpoint for that Recorder, Prometheus text format 0.0.4 written directly with no client library (ADR-0050)
- syncpool: NewBufferPool() *BufferPool; (*BufferPool).Get() *bytes.Buffer; (*BufferPool).Put(*bytes.Buffer)
- errx: Wrap(err error, msg string) error; Wrapf(err, format, args...) error — message only, no capture; WithStack(err error) error — opt-in capture, idempotent, nil-transparent; Frames(err error) []Frame — lazily resolved, nil when nothing was captured; Frame{Function, File string; Line int}; StackTracer interface{ StackTrace() []Frame } — the extension point Frames searches; errors.Is/As/Unwrap compatible (ADR-0046, supersedes ADR-0029)
- Error model: exported sentinel errors per package (ErrQueueFull, ErrOpen, ErrLimited, ...); an ordinary absence is reported comma-ok rather than as a sentinel (ADR-0047); context cancellation surfaces ctx.Err(); all wrapping is errors.Is/As transparent
- Versioning surface: SemVer over **every exported identifier of the module**, whether or not it is enumerated above; MAJOR = any breaking change to those signatures or their documented behavioral contracts. The enumeration is a reader's map, not the boundary of the promise — the boundary is what `go doc` reports. `contrib/*` submodules are outside it, versioning independently (ADR-0040).


## 6. Verification & Test Strategy

Every functional requirement maps to package-level table-driven unit tests (go test);
the Spec Coverage Map in ROADMAP.md keeps one row per spec section (spec-map lint gate).
Concurrency components additionally carry: a leak assertion (no leaked goroutines
after Stop/Close/cancel, asserted with go.uber.org/goleak), mandatory go test -race in CI, and deterministic
clocks for timing-sensitive logic (retry, ratelimit, cache TTL). Property-based tests (rapid) run in
eight packages: pubsub delivery/filtering, fanin/fanout completeness (no message lost or duplicated),
backoff bound invariants, breaker state-machine transitions, request-ID generation, rate-limit
admission, and the validator tag grammar. Static gates on every PR: gofumpt, golangci-lint (govet,
staticcheck, errcheck, revive, gosec), govulncheck. Coverage gate: at least 85 percent
line coverage per package (go test -coverprofile). Benchmarks (go test -bench -benchmem) for
workerpool, ratelimit, middleware, syncpool, cache, hash, and pubsub, with the measured
reports recorded under docs/benchmarks;
syncpool.BufferPool asserts zero steady-state allocations via testing.AllocsPerRun.
Manual-only gates: none — every requirement above has a mechanical check.

Toolchain: built with go build (go modules), tested with go test (+ testify; rapid for property tests), checked with
go test -race (data-race detector), go vet, govulncheck, coverage target ≥ 85% line per package. Every functional and
non-functional requirement above maps to a CI gate (see [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml)).
