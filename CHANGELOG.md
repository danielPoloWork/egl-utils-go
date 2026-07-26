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
- `lifecycle.Trigger()` — programmatic shutdown (roadmap 10.3, spec v2 item 21): unblocks a pending
  `WaitForSignals` exactly as a termination signal would, for code that decides to stop the process
  itself (a fatal background error, an admin endpoint, a supervisor command). Idempotent and safe for
  concurrent use; a `Trigger` that arrives before `WaitForSignals` latches instead of being lost.
  Additive — `Register`, `Shutdown`, and `WaitForSignals` keep their signatures (ADR-0030).

### Changed

### Deprecated

### Removed

### Fixed

- `go.mod` / `go.sum`: re-tidied after the `prometheus/client_golang` 1.24.1 bump (#44), which left
  the transitive `prometheus/common`, `prometheus/procfs`, and `protobuf` pins at their pre-bump
  versions and `go.sum` without the matching entries — `go build ./...` failed on a clean module
  cache with *missing go.sum entry for go.mod file*.

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
