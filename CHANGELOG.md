# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

### Changed

### Deprecated

### Removed

### Fixed

- CI's two `-race` jobs are green again, red on every push since 2026-07-26 — the `v1.1.0`
  release commit included ([BUG-0001](docs/bugs/2026/07/BUG-0001-race-detector-breaks-allocation-and-pool-identity-assertions.md)).
  No data race was ever involved: four test assertions measure allocation counts and `sync.Pool`
  object identity, both of which the race detector deliberately perturbs, so they described an
  instrumented binary rather than the one consumers run. They are now excluded from `-race` builds
  via `//go:build !race` and still gate on all four ordinary CI cells. Test-only change; no
  library behaviour is affected.

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v1.1.0](docs/changelog/v1/v1.1.0.md) | 2026-07-27 | M10 — spec v2.0 reconciliation: observable breaker state, programmatic shutdown, 429 rate-limit middleware, configurable bcrypt cost, config tag validation, fuzzing, import-graph + coverage gates, the NFR suite, cache sharding (7.5×), pubsub drop-oldest, and the contrib/* health probes |
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
