# Architecture Decision Records

One numbered Markdown file per decision, in the lightweight
[Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
format. Numbering is sequential and never reused or renumbered. Template:
[`template.md`](template.md).

Open an ADR when a choice affects the public surface or compatibility, when two reasonable
options exist and the rationale is non-obvious, when a **design pattern** is adopted, or
when superseding a prior decision. Do **not** open one for routine implementation details
or trivially reversible choices.

Status transitions: `Proposed` → `Accepted` → (`Superseded by ADR-XXXX` | `Deprecated`).

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](0002-adopt-cross-language-source-layout.md) | Adopt the cross-language source layout | Superseded by ADR-0003 |
| [0003](0003-adopt-idiomatic-go-root-layout.md) | Adopt the idiomatic Go root layout | Superseded by ADR-0045 |
| [0004](0004-runtime-dependency-policy.md) | Runtime dependency policy | Accepted (ring 3's membership superseded by ADR-0050: two runtime entries -> one; the policy itself stands) |
| [0005](0005-workerpool-design.md) | workerpool design — bounded pool, blocking-first admission, loud panics | Accepted (`Stop`/`ErrPoolClosed` *names* superseded by ADR-0048; every semantic it decided stands) |
| [0006](0006-pubsub-design.md) | pubsub design — at-most-once buffered delivery, no broker goroutines | Accepted (`Publish`/`Subscribe` signatures + the fixed drop-newest policy superseded by ADR-0049; every invariant stands) |
| [0007](0007-fanin-design.md) | fanin design — forwarder-per-input, cancel-or-drain contract | Accepted |
| [0008](0008-fanout-design.md) | fanout design — forwarder-per-output, exactly-once distribution | Accepted |
| [0009](0009-semaphore-design.md) | semaphore design — thin adapter over x/sync, first runtime dependency | Accepted |
| [0010](0010-circuitbreaker-design.md) | circuitbreaker design — lazy timerless transitions, generation-guarded accounting | Accepted |
| [0011](0011-retry-design.md) | retry design — proportional jitter, hard cap, last error verbatim | Accepted |
| [0012](0012-ratelimit-design.md) | ratelimit design — hand-rolled lazy token bucket, reservation-model Wait | Accepted |
| [0013](0013-middleware-requestid-design.md) | HTTP middleware foundation — Decorator chain and RequestID design | Accepted |
| [0014](0014-middleware-logger-design.md) | middleware.Logger design — ResponseWriter capture, status-derived levels, path-only logging | Accepted |
| [0015](0015-enterprise-governance-posture.md) | Enterprise governance posture — a raised compliance bar orthogonal to the domain | Accepted |
| [0016](0016-middleware-recoverer-design.md) | middleware.Recoverer design — panic-to-500, no stack to the client, ErrAbortHandler passthrough | Accepted |
| [0017](0017-middleware-cors-design.md) | middleware.Cors design — CorsConfig shape, deny-by-default, loud credential/wildcard guard | Accepted |
| [0018](0018-config-loader-design.md) | config.Loader design — generic Load, extension-driven format, gopkg.in/yaml.v3 selected | Accepted |
| [0019](0019-logger-structured-design.md) | logger.Structured design — slog JSON handler, functional options, default keys kept | Accepted |
| [0020](0020-logger-context-design.md) | logger.Context design — Field alias, accumulating context fields, slog.Default base | Accepted |
| [0021](0021-cache-inmemory-design.md) | cache.InMemory design — lazy expiry on Get, one sweeper goroutine, deterministic Close | Accepted (`Get` signature + constructor name superseded by ADR-0047) |
| [0022](0022-db-transaction-design.md) | db.Transaction design — rollback on error and panic, re-panic, joined rollback errors | Accepted |
| [0023](0023-validator-struct-design.md) | validator.Struct design — reflection tag grammar, literal rules, panic on tag misuse | Accepted |
| [0024](0024-hash-password-design.md) | hash password design — bcrypt at default cost, per-hash salt, constant-time verify | Accepted |
| [0025](0025-lifecycle-shutdown-design.md) | lifecycle.GracefulShutdown design — LIFO hooks, exactly-once convergent Shutdown, no hidden timeout | Accepted |
| [0026](0026-health-handler-design.md) | health.Handler design — concurrent probes, 200/503, status-only body (no error leak) | Accepted |
| [0027](0027-metrics-prometheus-design.md) | metrics.Prometheus design — bounded-cardinality labels, client_golang pin, uncalled-vuln trade-off | Accepted (surface, implementation + pin superseded by ADR-0050; the cardinality decisions stand and are re-enforced there) |
| [0028](0028-syncpool-bufferpool-design.md) | syncpool.BufferPool design — sync.Pool of bytes.Buffer, reset on return, discard oversized | Accepted |
| [0029](0029-errors-wrap-design.md) | errors.Wrap design — %w-transparent wrapping, one-time origin stack, errors package name | Superseded by ADR-0046 |
| [0030](0030-spec-v2-reconciliation.md) | Spec v2.0 reconciliation — hybrid adoption: additive deltas in v1.x, breaking deferred to /v2 | Accepted |
| [0031](0031-ratelimit-middleware-design.md) | ratelimit HTTP middleware — 429 shed via Allow (never Wait), constant Retry-After, ErrLimited sentinel | Accepted |
| [0032](0032-hash-password-cost-design.md) | configurable bcrypt cost — floor of 10 enforced locally (upstream accepts weak costs silently), error not panic, rehash-on-login | Accepted |
| [0033](0033-config-struct-validation.md) | config.WithStructValidation — opt-in tag validation, tags before Validate, and the module's first internal package edge | Accepted |
| [0034](0034-fuzzing-strategy.md) | fuzzing strategy — contract-shaped invariants (runtime.Error vs documented panic), bounded tag space, hand-authored corpus | Accepted |
| [0035](0035-import-graph-enforcement.md) | import-graph enforcement — depguard per file plus a resolved-graph assertion; one sanctioned internal edge | Accepted |
| [0036](0036-coverage-gate.md) | statement-coverage floor — 85% enforced per package (a module-wide average could not fail) | Accepted |
| [0037](0037-nfr-benchmark-methodology.md) | NFR benchmark methodology — gate the hardware-independent NFRs, report the rest; NFR-01's 0-alloc target unachievable | Accepted |
| [0038](0038-cache-sharding.md) | cache sharding — 32 shards hashed with maphash.Comparable; 7.5x on the mixed path, ~5ns tax uncontended | Accepted |
| [0039](0039-pubsub-drop-oldest.md) | pubsub.WithDropOldest — opt-in slow-subscriber policy, best-effort by construction (Publish must not block) | Accepted (the option's name + shape superseded by ADR-0049's `SlowSubscriberPolicy`; its best-effort reasoning is load-bearing there) |
| [0040](0040-contrib-submodules.md) | contrib/* nested submodules — require the released core, no replace/workspace; the module boundary is the enforcement | Accepted |
| [0041](0041-series-logical-namespace.md) | series logical namespace `it.d4np.utils.<component>` — realized per language, Go keeps the module root; the module-path move is free only at a /v2 boundary | Accepted |
| [0042](0042-post-1.0-compatibility-contract.md) | post-1.0 compatibility contract — v1.x is frozen for every exported identifier; the MAJOR-intent note is retired and the /v2 ledger is the only destination for a breaking change | Accepted |
| [0043](0043-spec-api-lint.md) | spec §5 gated against `go doc` — fails both on shipped-but-unlisted and listed-but-gone; the fourth policy checker | Accepted |
| [0044](0044-canonical-header-key-for-map-access.md) | canonical header key for map access — 2 allocs/request removed without touching HeaderName's exported value, so a PATCH rather than the MAJOR 10.10 assumed | Accepted |
| [0045](0045-pkg-layout-and-v2.md) | feature packages under `pkg/` and the module's second major — the Maven tree built, measured at 86-char imports, and reverted; `/v2` empties the ADR-0030 ledger | Accepted |
| [0046](0046-errx-opt-in-stacks.md) | errx — off the stdlib name, stack capture opt-in via `WithStack`, traces as `[]Frame` resolved lazily; measuring showed v1's `Wrap` paid 276 ns to *find* a stack it already had | Accepted |
| [0047](0047-cache-comma-ok.md) | `cache.Get` → `(V, bool)`, `NewInMemory` → `New`, `ErrNotFound` removed — the error channel carried one bit, and Go spells that bit comma-ok | Accepted |
| [0048](0048-workerpool-close.md) | `workerpool.Stop` → `Close`, `ErrPoolClosed` → `ErrClosed` — one shutdown verb for the module, but `ctx` stays: the pool is the only shutdown that waits on work the caller wrote | Accepted |
| [0049](0049-pubsub-reshape.md) | the pubsub reshape — context-scoped subscriptions, a `Publish` that reports without ever blocking, and a three-valued slow-subscriber policy; `topic` kept against the ledger's shorthand | Accepted |
| [0050](0050-metrics-without-the-sdk.md) | metrics without the SDK — the exposition format written directly, `New() *Recorder` replacing `prometheus.Registerer`, and ring 3 down to one entry; nine modules left the graph | Accepted |
