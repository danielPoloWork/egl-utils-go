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
  `actions/attest` binds the document's digest to the workflow and commit that
  produced it; verify with `gh attestation verify`. The attestation covers **the SBOM, not the
  module**: what a consumer resolves is already anchored in `sum.golang.org`, and claiming otherwise
  would put a weaker guarantee beside a stronger one. Licence detection is deliberately off — it is
  wrong on all three components.
- **[`CONTRIBUTING.md`](CONTRIBUTING.md) and [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)** — the
  repository had an agent contract and no written path in for a human (roadmap 14.9).
  `CONTRIBUTING.md` carries what nothing else did: that the four policy tools run **before** the pull
  request rather than in review, the exact `gofumpt` version CI pins (`@latest` formats differently
  and fails the build), the one-item-at-a-time rule with its mechanical reason — a squash merge
  leaves the merged branch as no ancestor of `master`, so a stacked branch is a rebase waiting to
  happen — the type-label set, the five triggers that make a change need an ADR, and how to propose
  a capability. The Code of Conduct is Contributor Covenant 2.1. Both are linked from the issue
  chooser and the README. Documentation only: no Go file, exported identifier, behaviour or
  dependency changed.
- **The [additive-capability ledger](docs/adr/0057-additive-capability-ledger.md)** — every
  capability this project deliberately deferred, in one place, each with the **trigger** that would
  schedule it (roadmap 14.10, [ADR-0057](docs/adr/0057-additive-capability-ledger.md)). Until now
  those decisions lived as one-line notes in the Consequences section of 26 different ADRs, findable
  only by grep. §A holds 49 open capabilities across the public surface, §B the internal deferrals
  (seven at adoption, eight once 14.11 registered one), §C the eleven that had already been built
  without the deferring ADR ever saying so, and
  §D two that turned out not to be additive and belong to a future major. If you have been working
  around a gap, the ledger is where to check whether it is already known — and the useful thing to
  send is the trigger: what you need, what you do instead today, and what that costs
  (`CONTRIBUTING.md` §7). `consistency_lint.py` gains an eleventh check so a new deferral cannot be
  recorded only in prose. Documentation and tooling only: no Go file, exported identifier, behaviour
  or dependency changed.
- **A `v0.1.0` GitHub Release**, backfilled from
  [`docs/releases/v0.1.0.md`](docs/releases/v0.1.0.md) (roadmap 14.11). The tag has existed since
  2026-07-12 with no Release while every later tag had one. Nothing about the tagged code changed and
  no artifact is attached; **`v2.0.0` remains the latest release**, and installing `v0.1.0` is not
  recommended — it predates every feature package.
- **[ADR-0058](docs/adr/0058-no-documentation-site.md) — no documentation site**, recorded as a
  decision rather than left as an unapplied setup step (roadmap 14.11). `pkg.go.dev` is the doc site
  for a Go library and already renders 55 verified runnable examples; GitHub Pages from `docs/` would
  publish only `docs/` and so 404 on every link to `AGENTS.md`, `CONTRIBUTING.md` and
  `CODE_OF_CONDUCT.md`, which are root files. A curated subset site is registered as a deferred
  capability with its trigger, not dismissed.

### Changed

- **The last two unverified NFRs now have measured tails** (roadmap 14.8,
  [ADR-0037](docs/adr/0037-nfr-benchmark-methodology.md) amended,
  [report](docs/benchmarks/2026-07-26-nfr-suite.md) updated in place). **NFR-02's `Submit` p99 is met at
  176 ns against a 2 µs target** — conservatively, since the pipeline is consumer-limited and the
  measured tail includes queue back-pressure the NFR's "uncontended" excludes. **NFR-06's p99 is not
  met: 887 ns against 200 ns** with no oversubscription at all, so the shortfall is the code's latency
  under concurrent load rather than the runner's core count. The measurement was never missing:
  `nfr-nightly` had been publishing both percentiles on Linux since the suite was built, into an
  artifact nobody opened. The workflow now prints the tail lines to both the run summary and the job
  log, and warns when a tail comes back `tail-unmeasurable` on a clock that should manage it.
  `BenchmarkNFR06GetTailPerCore` is added, because a wall-clock batch timed inside one of 8 goroutines
  on 4 cores measures **residency**, not service time — aggregate 97.1 ns/op against a 743 ns/op batch
  p50 in the same benchmark line, a factor of exactly the goroutine count. The same arithmetic means
  10.11's "NFR-06 met at the mean, 46.6 ns" compared a throughput figure against a latency target;
  that is flagged for the maintainer as a spec question, and ADR-0038's sharding result is unaffected.
  Benchmarks and documentation only: no exported identifier, behaviour or dependency changed.
- **[`SECURITY.md`](SECURITY.md)'s supported-versions table now describes the versions that exist**
  (roadmap 14.9). It had said "until `egl-utils-go` reaches `v1.0.0`, only the latest released minor
  line receives security fixes" and listed `0.x` rows — false since `v1.0.0` and misleading since
  `v2.0.0`. Supported is now the latest released `v2.x`; `v1.1.1` stays resolvable from the proxy, as
  every published Go version does, and receives no fixes. The same file also records that its private
  reporting form receives code-of-conduct reports, so one is not mistaken for a misfiled
  vulnerability.

### Deprecated

### Removed

### Fixed

- **Private vulnerability reporting is enabled on the repository** (roadmap 14.11). It was off.
  [`SECURITY.md`](SECURITY.md) names GitHub's private reporting form as the way to report a
  vulnerability, and [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) designates the same form for conduct
  reports — with the setting disabled, that form was reachable only by users who can already create a
  draft security advisory, so **an outside reporter had no route at all**. If you tried to report
  something privately and could not, this is why; please try again.
- **The repository has a description, twelve topics and a homepage** pointing at
  [pkg.go.dev](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2) (roadmap 14.11). All
  three were empty, because `docs/workflow/github-setup.md` had never documented them — undocumented
  setup is unapplied setup.

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
