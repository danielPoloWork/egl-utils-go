# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

- `workerpool.Pool` — bounded-queue worker pool (roadmap 2.1): blocking or fail-fast
  `Submit`, context-aware `Stop` with full drain, opt-in panic containment (ADR-0005).
- `pubsub.Broker[T]` — filtered in-memory publish-subscribe broker (roadmap 2.2):
  at-most-once buffered delivery with observable drops, no broker goroutines, additive
  `Close` (ADR-0006).
- `fanin.Merge[T]` — multi-channel merge with completeness and per-input ordering
  guarantees, cancel-or-drain consumer contract (roadmap 2.3, ADR-0007).
- `fanout.Split[T]` — exactly-once multi-channel work distribution with per-output input
  ordering; forwarder-per-output, closes the outputs on input-close or cancel
  (roadmap 2.4, ADR-0008).
- `semaphore.Weighted` — weighted admission control, a thin house-contract adapter over
  `golang.org/x/sync/semaphore` with loud panics on misuse (roadmap 2.5, ADR-0009). Adds
  the module's first runtime dependency, `golang.org/x/sync` v0.16.0 (newest release on a
  `go 1.23` directive, so the module's `go 1.24` floor is preserved unchanged).
- `circuitbreaker.Breaker` — closed/open/half-open circuit breaker (roadmap 3.1):
  consecutive-failure tripping, lazy timerless state transitions (no goroutines, no
  timers), generation-guarded outcome accounting, half-open probe budget equal to the
  success threshold (ADR-0010).
- `retry.Backoff` — retrying execution with exponential backoff and proportional jitter
  (roadmap 3.2): total-attempt budget, hard `MaxDelay` cap that survives jitter,
  overflow-safe doubling, context cancellation honored before the first call and during
  every sleep (ADR-0011).
- `ratelimit.Limiter` — token-bucket rate limiter (roadmap 3.3, completes Milestone 3):
  lazy refill with no background goroutines or timers, fail-fast `Allow` and queueing
  `Wait` with arrival-order reservations, canceled waits repay their token; ~25ns
  zero-allocation admission (first report under `docs/benchmarks/`) (ADR-0012).

### Changed

- Test infrastructure (roadmap 2.6, dev-facing only — no change to the consumer surface):
  adopted the ADR-0004 test-only dependencies. The interim in-repo goroutine-leak guard
  (`internal/leakcheck`) is retired in favor of `go.uber.org/goleak`; the randomized
  completeness/delivery tests for `fanin`, `fanout`, and `pubsub` are rewritten as
  `pgregory.net/rapid` properties (automatic shrinking); assertions use
  `github.com/stretchr/testify`.

### Deprecated

### Removed

### Fixed

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
