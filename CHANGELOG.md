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
