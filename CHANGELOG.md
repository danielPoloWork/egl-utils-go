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

- **`examples / service` is now a required status check on `master`** — fourteen contexts instead of
  thirteen. It was the last CI job that ran without blocking anything, which is how
  [BUG-0002](docs/bugs/2026/08/BUG-0002-unbuffered-started-channel-deadlocks-two-examples-service-tests.md)
  went red on two pull requests and merged both, the second being the `v2.0.1` release. The context
  was added **after** the flake was fixed, because requiring an intermittently-failing job blocks the
  repository rather than protecting it. Governance only: no code, surface or behaviour changed.

### Deprecated

### Removed

- **The `v0.1.0` GitHub Release that `v2.0.1` announced was never published**, and the draft has been
  removed. `v0.1.0` keeps its git tag and, as before, has no Release. That version predates every
  feature package and its own notes say installing it is not recommended, so the Release would have
  published little beyond "do not use this", while
  [`docs/releases/v0.1.0.md`](docs/releases/v0.1.0.md) already holds the record. The `v2.0.1`
  [changelog](docs/changelog/v2/v2.0.1.md) and [notes](docs/releases/v2.0.1.md) carry a dated
  correction beside the original claim rather than in place of it. No code, surface or behaviour is
  affected.

### Fixed

- **Two `examples/service` tests could deadlock until the 10-minute test timeout**
  ([BUG-0002](docs/bugs/2026/08/BUG-0002-unbuffered-started-channel-deadlocks-two-examples-service-tests.md)).
  `TestReadinessFailsWhileTheQueueIsFull` and `TestOrderIsShedWhenTheQueueIsFull` announced "a worker
  has dequeued the first task" with a non-blocking send on an **unbuffered** channel, so whenever the
  worker reached the send before the test reached the receive there was no receiver, `select`'s
  `default` was taken, and the signal was silently dropped — after which the test waited forever.
  The channel is now buffered, which makes the first send land whether or not the receiver has
  arrived. **No consumer is affected:** `examples/service` is a separate module that is never tagged
  and is not part of the published `…/v2` module, and no production code changed.

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v2.0.1](docs/changelog/v2/v2.0.1.md) | 2026-08-08 | M14 — adoption: **55 runnable examples across all 21 packages** reach pkg.go.dev (which renders the tagged tree), `examples/service` as its own module, a verified `contrib/*` release act, the build-time supply chain pinned and gated with the project's first release artifact (a reproducible SBOM + provenance), `CONTRIBUTING.md`/`CODE_OF_CONDUCT.md`, the additive-capability ledger, and the two remaining NFR tails measured. **No code changed — no exported identifier, behaviour or dependency** |
| [v2.0.0](docs/changelog/v2/v2.0.0.md) | 2026-07-30 | M13 — the second major: feature packages under `pkg/` and the module path at `/v2` (every import changes), plus the seven deferred breaking changes discharged in one release — `errors`→`errx` with opt-in stacks, `cache.Get` comma-ok, `workerpool.Close`, the pubsub reshape, `metrics` without the Prometheus SDK (18 modules → 9), `WaitForSignals(timeout, ...)`, and bcrypt’s default cost 10 → 12 |
| [v1.1.1](docs/changelog/v1/v1.1.1.md) | 2026-07-27 | Two allocations removed from every `RequestID` request via a canonical header key, with `HeaderName` unchanged (ADR-0044); the `-race` CI jobs repaired after a day red (BUG-0001). No API change |
| [v1.1.0](docs/changelog/v1/v1.1.0.md) | 2026-07-27 | M10 — spec v2.0 reconciliation: observable breaker state, programmatic shutdown, 429 rate-limit middleware, configurable bcrypt cost, config tag validation, fuzzing, import-graph + coverage gates, the NFR suite, cache sharding (7.5×), pubsub drop-oldest, and the contrib/* health probes |
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
