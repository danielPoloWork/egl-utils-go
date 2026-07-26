# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

- `circuitbreaker.State` / `(*Breaker).State()` — observable breaker state (roadmap 10.2, spec v2
  item 6): an exported `State` type (`StateClosed`/`StateOpen`/`StateHalfOpen`, with `String()`)
  and a `State()` accessor. `State()` is a pure read-only observer — it reflects the lazy,
  time-based transition (an open breaker whose cool-down has elapsed reports `StateHalfOpen`)
  without performing it, so polling it for metrics never admits a probe, mutates the breaker, or
  advances the generation. Additive; the existing `Do`/`ErrOpen` surface is unchanged (ADR-0030,
  lifting ADR-0010's deferral).

### Changed

### Deprecated

### Removed

### Fixed

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
