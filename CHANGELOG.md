# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

> **The next release is `v2.0.0`** — a major. Feature packages moved under `pkg/` and the module
> path gained its `/v2` suffix ([ADR-0045](docs/adr/0045-pkg-layout-and-v2.md)), so **every consumer
> import changes**. The migration is mechanical:
> `…/egl-utils-go/<pkg>` → `…/egl-utils-go/v2/pkg/<pkg>`. Milestone 13 also empties the
> [ADR-0030](docs/adr/0030-spec-v2-reconciliation.md) §2 ledger in the same major; entries below
> accumulate until the release rolls them.

### Added

### Changed

- **BREAKING** — **`lifecycle.WaitForSignals` takes a shutdown timeout as its first argument**
  ([ADR-0051](docs/adr/0051-lifecycle-shutdown-timeout.md), supersedes ADR-0025's "no hidden timeout"
  decision and its rejected default-timeout alternative, and nothing else). Migration:
  `lifecycle.WaitForSignals(os.Interrupt, syscall.SIGTERM)` →
  `lifecycle.WaitForSignals(10*time.Second, os.Interrupt, syscall.SIGTERM)`, **or `0` for exactly the
  previous behaviour**. The timeout bounds the whole shutdown sequence — it becomes the deadline on
  the context every hook receives — and is **measured from the moment the signal arrives**, not from
  the call, so a long-running process still gets its full budget. **`0` imposes no deadline**, leaving
  the bound to the platform's kill escalation; that is not a compatibility shim but the recommended
  posture wherever systemd or Kubernetes already enforces a grace period, since a second number in
  the application would be free to drift from the first and the shorter would silently win. A
  negative timeout panics at the call. **The bound is cooperative and never abandons the sequence:** a
  hook that honours its context winds up early, one that ignores it still runs to completion, and an
  expired deadline does not skip the remaining hooks — ADR-0025's run-every-hook-and-join-errors
  decision is untouched, along with LIFO ordering, exactly-once convergent `Shutdown`, the loud
  panics and the zero-owned-goroutines property.
- **BREAKING** — **`metrics` no longer depends on the Prometheus SDK**; it writes the text exposition
  format directly, and `prometheus.Registerer` is gone from the public API
  ([ADR-0050](docs/adr/0050-metrics-without-the-sdk.md), supersedes ADR-0027's surface,
  implementation and dependency pin, and ring 3's membership in ADR-0004). Migration:
  `h := metrics.Prometheus(prometheus.DefaultRegisterer)(appHandler)` → `rec := metrics.New()` then
  `h := rec.Middleware()(appHandler)`; `metrics.Handler()` → `rec.Handler()`.
  **The dependency graph halves — 18 modules to 9**, `go.sum` from 50 lines to 24: `client_golang`,
  `client_model` and the seven transitive modules that existed only to serve them all leave, so
  consumers of this module no longer inherit a metrics SDK, a protobuf runtime or `golang.org/x/sys`.
  A `Recorder` owns its counters, so the middleware and the endpoint are provably the same state —
  which fixes a v1 wart where `Prometheus(myReg)` paired with `Handler()` silently exposed the
  *default* registry — and a double install is two independent recorders rather than a panic.
  **What is given up, deliberately: the endpoint no longer serves the 37 metric families `promhttp`
  supplied for free** (29 `go_*`, 6 `process_*`, 2 `promhttp_*`). A consumer wanting runtime or
  process metrics imports a Prometheus client itself and mounts its handler at a second path, which
  puts that dependency in the builds that actually want it. **Everything ADR-0027 decided about
  cardinality is unchanged** — two families, `(method, code)` labels only, never the request path,
  the method normalized to nine verbs plus `other`, and the standard latency buckets verbatim — and
  the exposition output is verified byte-for-byte against the reference encoder by a golden file
  captured while the SDK was still present. Recording is now allocation-free (223.0 → 63.4 ns/op,
  1 alloc → 0) and a scrape allocates 3 objects instead of 436; see
  [the report](docs/benchmarks/2026-07-29-metrics-without-the-sdk.md).
- **BREAKING** — the **pubsub API is reshaped**: subscriptions are context-scoped, `Publish` takes a
  context and returns an error, and the slow-subscriber policy is explicit
  ([ADR-0049](docs/adr/0049-pubsub-reshape.md), supersedes the two signatures and the fixed
  drop-newest policy of ADR-0006 and the option name of ADR-0039). Migration:
  `ch, unsub := br.Subscribe(topic, f)` → `ch := br.Subscribe(ctx, topic, f)`, replacing the
  `unsub()` call with cancelling `ctx`; `br.Publish(topic, msg)` → `br.Publish(ctx, topic, msg)`;
  `pubsub.WithDropOldest[T]()` → `pubsub.WithSlowSubscriberPolicy[T](pubsub.DropOldest)`.
  **`Publish` still never blocks** — that promise was previously a consequence of having neither a
  context nor an error to return, and is now explicit: `ctx` is consulted **once, before anything is
  delivered** (a cancelled publish delivers to nobody, all-or-nothing rather than a partial fan-out)
  and is never waited on, while the error reports only what already happened — `ErrSlowSubscriber`
  when at least one subscription lost a message, `ErrClosed` on a closed broker, and `nil` otherwise.
  A topic with no subscribers is not an error. **Cancelling a subscription's context is the
  unsubscribe**, so the returned `func()` is gone; `context.AfterFunc` keeps the broker's
  zero-goroutine guarantee intact, and subscribing with an already-cancelled context or to a closed
  broker returns an already-closed channel. The new `WithSlowSubscriberPolicy` chooses between
  `DropNewest` (the zero value, so the default is unchanged), `DropOldest` and the new
  **`Disconnect`**, which sheds a subscription that has fallen behind rather than picking a message
  to lose — its buffered messages stay receivable. The accounting invariant holds under all three:
  while a subscription is registered, every message published to it is either delivered or reported
  to the drop handler, exactly once. **A caller who ignores the new error gets exactly v1's
  behaviour**, so the migration can be done in two passes.
- **BREAKING** — `workerpool.Stop` is now **`Close`** and `ErrPoolClosed` is now **`ErrClosed`**
  ([ADR-0048](docs/adr/0048-workerpool-close.md), supersedes those two names in ADR-0005 and nothing
  else). Two of the module's three goroutine-owning types already said `Close`; the pool was the
  outlier. Migration: `p.Stop(ctx)` → `p.Close(ctx)`; `workerpool.ErrPoolClosed` →
  `workerpool.ErrClosed`. **`Close` keeps its context** — the pool is the only shutdown in the module
  that waits for caller-supplied work, so the caller bounds that wait rather than the pool inventing a
  hidden timeout (ADR-0025). The accepted cost: **`*Pool` does not satisfy `io.Closer`**. No
  behavioural change — admission policy, idempotence, the drain guarantee and the panic policy are
  byte-for-byte those of ADR-0005.
- **BREAKING** — `cache.Get` now returns **`(V, bool)`** instead of `(V, error)`, `NewInMemory` is
  now **`New`**, and **`ErrNotFound` is removed**
  ([ADR-0047](docs/adr/0047-cache-comma-ok.md), supersedes ADR-0021's `Get` signature and
  constructor name). `ErrNotFound` was the only error `Get` could return, so the error channel
  carried a single bit — which Go spells comma-ok. Migration:
  `v, err := c.Get(k); if err == nil` → `v, ok := c.Get(k); if ok`;
  `cache.NewInMemory[K, V](ttl)` → `cache.New[K, V](ttl)`. **No behavioural change** — the condition
  deciding presence is unchanged, a present-but-expired entry still reads as absent, and absence and
  expiry remain deliberately indistinguishable. The sentinel is removed rather than kept, because
  `errors.Is(err, cache.ErrNotFound)` would otherwise still compile and simply never be true.
- **BREAKING** — the `errors` package is now **`errx`** (`…/v2/pkg/errx`), and **stack capture is
  opt-in** ([ADR-0046](docs/adr/0046-errx-opt-in-stacks.md), supersedes ADR-0029). `Wrap`/`Wrapf`
  attach a message and no longer capture a call stack; request one explicitly with the new
  `WithStack(err)`, which is nil-transparent and idempotent. A trace is read with the new
  `Frames(err) []Frame` — `Frame{Function, File, Line}`, so no consumer touches program counters or
  imports `runtime` — and `StackTracer.StackTrace()` returns `[]Frame` instead of `[]uintptr`.
  Frames resolve lazily on first read. Migration: `errors` → `errx`; add `WithStack` where a trace
  is wanted; replace `runtime.CallersFrames(st.StackTrace())` with a `range` over `Frames(err)`.
  Wrapping an error still keeps the trace pointed at the original failure site, and the chain stays
  `errors.Is`/`As`/`Unwrap`-transparent.
- **BREAKING** — feature packages moved from the module root to `pkg/`, and the module path is now
  `github.com/danielPoloWork/egl-utils-go/v2`
  ([ADR-0045](docs/adr/0045-pkg-layout-and-v2.md), supersedes ADR-0003). Module metadata
  (`doc.go`, `version.go`) stays beside `go.mod`, so `…/v2` remains importable for `utils.Version`.
  No exported identifier changed — only where it lives. `contrib/*` is unaffected and still targets
  the core's v1 line until `v2.0.0` is tagged (ADR-0040 requires the *released* core).

### Deprecated

### Removed

### Fixed

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v1.1.1](docs/changelog/v1/v1.1.1.md) | 2026-07-27 | Two allocations removed from every `RequestID` request via a canonical header key, with `HeaderName` unchanged (ADR-0044); the `-race` CI jobs repaired after a day red (BUG-0001). No API change |
| [v1.1.0](docs/changelog/v1/v1.1.0.md) | 2026-07-27 | M10 — spec v2.0 reconciliation: observable breaker state, programmatic shutdown, 429 rate-limit middleware, configurable bcrypt cost, config tag validation, fuzzing, import-graph + coverage gates, the NFR suite, cache sharding (7.5×), pubsub drop-oldest, and the contrib/* health probes |
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
