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

### Changed

### Deprecated

### Removed

### Fixed

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
