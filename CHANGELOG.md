# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

- **An [issue index](ISSUES.md)** — the open backlog as a checkbox list, each line carrying its
  severity badge, the GitHub issue it mirrors, and the model/effort tier recommended to resolve it,
  under the same convention `ROADMAP.md` already uses. It opens with 43 entries: the findings of a
  seven-member release review board run against `master` versus `v2.0.1`, filed as issues
  `#106`–`#148` into a tracker that had been empty. Ordering is **chronological, not by priority** —
  the position of a line is its age, so severity travels on the line rather than in the sort. The
  `ROADMAP.md` model lineup moves from Opus 4.8 to **Opus 5** in the same change, because two
  documents naming different lineups is exactly the drift an index like this exists to prevent;
  `(as built)` tags naming Opus 4.8 are historical records and stay as written.
  *Documentation and process only: no code, surface or behaviour changed.*
- **A [usage guide](docs/usage/README.md)** — task-oriented recipes answering "how do I…" for every
  package, with the smallest code that answers each. It fills the layer this project was missing:
  between a one-line install and per-identifier reference documentation, there was nothing that
  showed the packages doing a job. Every snippet is derived from code CI compiles and runs — the
  package examples and `examples/service` — rather than written from memory.
- **A rewritten README front page.** It now opens the way a library should: what the module is for,
  `go get`, and a complete runnable service in ~60 lines using `workerpool`, `ratelimit`,
  `middleware`, `health` and `lifecycle` together — and that service is written as production code
  rather than as illustration. All four `http.Server` timeouts are stated (`ReadHeaderTimeout` is
  what closes Slowloris, and gosec's G112 fires without it); a failed listener is routed into the
  shutdown path through `lifecycle.Trigger` instead of having its error discarded, so a bind failure
  stops the process rather than leaving it up, healthy-looking and serving nothing; and liveness and
  readiness are **separate** endpoints, `/readyz` exercising the real admission path through
  `pool.Submit` so a saturated instance is taken out of the load balancer rather than kept in it by
  a probe that verifies nothing. Adds
  installation, documentation, compatibility and stability, and contributing/support sections;
  standard Go badges (Go Reference, CI, Go Report Card, Go version, licence); and moves the
  delivery milestones and the project's internal document index into a collapsed **Project
  governance** section, so the visible page addresses someone evaluating the library rather than
  someone auditing the process.
- **A `Packages` section in the README** — all 21 feature packages, grouped by what they are for,
  each with a sentence on what it does and a link to its **full documentation on pkg.go.dev**, where
  every exported identifier and all 55 runnable examples live. The front door described the module
  and never listed what was in it, so a reader had to guess at the inventory or browse `pkg/`. It
  also states the property that makes the list usable: no package here imports another, with exactly
  one sanctioned exception (`config` → `validator`), so taking one brings at most one sibling — and
  names `import_graph_lint.py` as what enforces that in both directions, an unsanctioned edge and a
  dead allowlist entry failing alike. `consistency_lint.py` gains a twelfth check asserting the
  section names exactly the packages that exist, in both directions — a hand-written table of 21
  rows is precisely what goes stale when the twenty-second arrives.

### Changed

- **Signed commits are now required on `master`.** Every commit on the branch already satisfied it —
  the repository is squash-only, so GitHub creates and signs the squash commit itself — which is why
  [ADR-0056](docs/adr/0056-build-time-supply-chain.md) §(e) recorded it as free. Enabling it closes
  the paths that remained: an administrator's direct push, or a future change of merge strategy.
  Release **tags** are unaffected and remain annotated-but-unsigned, deliberately and for the reasons
  in that ADR. Governance only: no code, surface or behaviour changed.
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

- **`pkg/lifecycle`'s package documentation taught a discarded listener error.** Its opening example
  ran the server as `go func() { _ = server.ListenAndServe() }()`, so a reader who copied it got a
  process that survives a bind failure — up, passing liveness, serving nothing — which is precisely
  the case the same doc's `Trigger` paragraph exists to answer. The example now checks the error
  against `http.ErrServerClosed` and calls `lifecycle.Trigger` on anything else, giving `Trigger`
  the use it was documented for. Documentation only: no exported symbol, signature or behaviour
  changed. This one is worth noting where the README quickstart's identical defect is not, because
  pkg.go.dev renders Go doc comments and not Markdown — this text has been in front of consumers
  since `v1.0.0`, in every published version of the module.
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
