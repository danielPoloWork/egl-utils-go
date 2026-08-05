# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

- Runnable examples on pkg.go.dev for `workerpool`, `pubsub`, `fanin`, `fanout`, `semaphore`,
  `circuitbreaker`, `retry` and `ratelimit` — 13 examples, each with output verified by `go test`
  (roadmap 14.2, [ADR-0053](docs/adr/0053-runnable-examples-convention.md)). Documentation only: no
  exported identifier, behaviour or dependency changed, and the examples are `_test.go` files a
  consumer never compiles. They become visible only on the next tagged version, since pkg.go.dev
  renders the tagged tree.
- Runnable examples for `middleware`, `health`, `metrics`, `logger` and `lifecycle` — 12 more,
  including a package-level example of the middleware chain in the order it should be composed
  (roadmap 14.3). Same terms: documentation only, nothing exported or behavioural changed.
- Runnable examples for `config`, `env`, `cache`, `db`, `validator`, `hash`, `syncpool` and `errx` —
  29 more, completing the set at **55 examples across all 21 packages** (roadmap 14.4). `db`'s run
  over a stub `driver.Connector` built on `database/sql/driver`, so the transaction helper's
  commit/rollback/panic contract is demonstrated without the module gaining a driver dependency.
  Same terms: documentation only, nothing exported or behavioural changed.
- [`examples/service`](examples/service) — a runnable HTTP service composing eight packages, as a
  module of its own that requires the released core with no `replace` (roadmap 14.5,
  [ADR-0054](docs/adr/0054-examples-service-module.md)). It shows what a package's documentation
  cannot: the middleware chain order, the operational endpoints kept outside it, liveness and
  readiness as two different questions, a readiness probe that exercises a real admission path, and
  shutdown hooks registered in dependency order. Its `go.mod` has one `require` line and no
  indirect requirements, which is the module's dependency policy made visible. Nothing in the core
  imports it and the core's `go.mod`/`go.sum` are unchanged.
- `.github/workflows/contrib-release.yml` — a `contrib/<name>/vX.Y.Z` tag is now verified by CI
  instead of by hand (roadmap 14.6, [ADR-0055](docs/adr/0055-contrib-release-workflow.md)).
  `release.yml`'s `v*.*.*` filter never matched a submodule tag, so the first two submodule releases
  ran nothing at all. The new workflow derives the module directory from the tag and refuses a tag
  that names no module, whose `go.mod` declares a different path, that omits the `/vN` suffix Go
  requires from `v2`, or that points at a commit unreachable from `master`; then it builds, vets,
  runs `go test -race` and `go mod verify` in that directory. It **drafts no GitHub Release** — the
  annotated tag stays the record, and leaving the tag unpublished is what keeps delete-and-repush
  available when the run is red. No consumer-visible change: no Go file, exported identifier,
  behaviour or dependency is touched.
- **A CycloneDX SBOM attached to each release, with a provenance attestation** — the first artifact
  ever attached to a release of this project (roadmap 14.7,
  [ADR-0056](docs/adr/0056-build-time-supply-chain.md)). `egl-utils-go-v<X.Y.Z>.cdx.json` inventories
  the module's **runtime** dependencies — exactly the three a consumer links, which is
  [ADR-0004](docs/adr/0004-runtime-dependency-policy.md)'s policy in machine-readable form — and is
  byte-reproducible from the tag, asserted by `cmp` on every pull request rather than claimed.
  `actions/attest-build-provenance` binds the document's digest to the workflow and commit that
  produced it; verify with `gh attestation verify`. The attestation covers **the SBOM, not the
  module**: what a consumer resolves is already anchored in `sum.golang.org`, and claiming otherwise
  would put a weaker guarantee beside a stronger one. Licence detection is deliberately off — it is
  wrong on all three components.

### Changed

### Deprecated

### Removed

### Fixed

### Security

- **The build-time supply chain is now a gated policy rather than a half-applied one** (roadmap 14.7,
  [ADR-0056](docs/adr/0056-build-time-supply-chain.md), control C-6). Every GitHub Action is pinned
  to a 40-character commit digest with its release in a `# vX.Y.Z` comment: **21 of 36 references
  were floating on mutable tags**, including `actions/checkout` at two sites in a file that pinned it
  at eleven others. No workflow grants a token scope any more (`permissions: {}`) and each of the
  thirteen jobs declares its own — `release.yml`'s `contents: write` used to sit at the *workflow*
  level, where every job added later would have inherited it; exactly one job now holds write access.
  Two new `consistency_lint.py` checks (`action-pins`, `workflow-permissions`) fail the build on a
  reintroduced tag, a missing permissions block, or an unallowlisted `write`, and they ride the
  already-required `consistency / lint` context rather than a new job that would not have been
  required until someone edited branch protection by hand. Both verified by deliberate violation
  across ten cases — the tenth being a blind spot in the new check itself, which could not see a
  scope documented with a trailing comment and so passed green on the one job it exists to police.
  Tags stay **annotated and unsigned**, deliberately and with the reasoning recorded; the commits on
  `master` turn out to be signed already, by GitHub's web-flow key.

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v2.0.0](docs/changelog/v2/v2.0.0.md) | 2026-07-30 | M13 — the second major: feature packages under `pkg/` and the module path at `/v2` (every import changes), plus the seven deferred breaking changes discharged in one release — `errors`→`errx` with opt-in stacks, `cache.Get` comma-ok, `workerpool.Close`, the pubsub reshape, `metrics` without the Prometheus SDK (18 modules → 9), `WaitForSignals(timeout, ...)`, and bcrypt’s default cost 10 → 12 |
| [v1.1.1](docs/changelog/v1/v1.1.1.md) | 2026-07-27 | Two allocations removed from every `RequestID` request via a canonical header key, with `HeaderName` unchanged (ADR-0044); the `-race` CI jobs repaired after a day red (BUG-0001). No API change |
| [v1.1.0](docs/changelog/v1/v1.1.0.md) | 2026-07-27 | M10 — spec v2.0 reconciliation: observable breaker state, programmatic shutdown, 429 rate-limit middleware, configurable bcrypt cost, config tag validation, fuzzing, import-graph + coverage gates, the NFR suite, cache sharding (7.5×), pubsub drop-oldest, and the contrib/* health probes |
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
