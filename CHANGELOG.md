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
