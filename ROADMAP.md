# Roadmap — egl-utils-go

The project's plan as a numbered, checkbox-driven list. When an item completes in a PR,
flip its checkbox (`- [ ]` → `- [x]`) **in the same PR**. New work goes at the bottom of
its section with a fresh `<milestone>.<task>` number; never renumber.

- **Versioning start:** pre-1.0 milestone-driven.
- **Session journal:** see [`docs/journal/`](docs/journal/). Latest checkpoint:
  [2026-07-12 — M1 bootstrap](docs/journal/2026/07/2026-07-12-m1-bootstrap.md).

### Agent guidance (model × effort)

Each milestone carries an advisory **Agent guidance** line: the Claude model and effort
level recommended to implement it (Claude Code exposes `low · medium · high · max`; there
is no separate "extra" tier). Heuristic: concurrency-critical and security-relevant work
gets the strongest reasoning tier at the highest effort; well-trodden integration work runs
on a lighter tier. Advisory only — the quality bar (AGENTS.md §10) and the human review
gate remain the arbiter regardless of which model wrote the code.

---

## Milestone 1 — Project bootstrap & CI

The thinnest slice that compiles, tests, and ships under the full quality bar.

> **Agent guidance:** Claude Fable 5 · effort **max** — completed with this tier; the
> layout decision (ADR-0003) shapes every consumer import and was one-way once published.

- [x] 1.1 Lay down the build system (go build (go modules)) and a buildable skeleton — root
      layout per ADR-0003, which supersedes the `src/main/go/it/d4np/utils/` tree.
- [x] 1.2 Wire the test framework (go test (+ testify; rapid for property tests)) with one passing smoke test —
      co-located per ADR-0003 (`version_test.go`).
- [x] 1.3 Add formatter + linter configs (gofumpt (gofmt superset), golangci-lint (govet, staticcheck, errcheck, revive, gosec)) at the repo root.
- [x] 1.4 Stand up the CI matrix (Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.24)) with build + test + format + lint.
- [x] 1.5 Seed the version constant (const Version = "X.Y.Z") in `version.go`.
- [x] 1.6 Record the Go module layout decision as an ADR: module path github.com/danielPoloWork/egl-utils-go, go.mod placement vs the normative src/main/go tree, consumer import ergonomics → [ADR-0003](docs/adr/0003-adopt-idiomatic-go-root-layout.md).
- [x] 1.7 Record the dependency policy as an ADR: runtime = stdlib + golang.org/x/* + vetted few (prometheus/client_golang, YAML parser); test-only = testify, goleak, rapid; govulncheck as the supply-chain gate → [ADR-0004](docs/adr/0004-runtime-dependency-policy.md).


---

## Milestone 2 — Concurrency primitives

The five channel-native concurrency building blocks, leak-free and race-clean

> **Agent guidance:** Claude Fable 5 · effort **max** — goroutine lifecycle correctness,
> leak/race-freedom proofs, generics API design, and property-based tests are the hardest
> correctness surface in the project; use the strongest tier.

- [ ] 2.1 workerpool.Pool — bounded-queue goroutine pool with Submit/Stop contract (leak, race, bench coverage)
- [ ] 2.2 pubsub.Broker — filtered-subscription in-memory broker (property tests for delivery)
- [ ] 2.3 fanin.Merge — multi-channel merge (completeness property tests)
- [ ] 2.4 fanout.Split — parallel channel distribution (completeness property tests)
- [ ] 2.5 semaphore.Weighted — weighted admission wrapper over x/sync/semaphore


---

## Milestone 3 — Resilience patterns

Fail-fast, retry, and rate-limit protection for outbound calls

> **Agent guidance:** Claude Fable 5 · effort **high** — state machines under concurrency
> (closed/open/half-open), backoff bound invariants with jitter, and deterministic-clock
> testing are subtle; timing bugs here surface only under load.

- [ ] 3.1 circuitbreaker.Breaker — closed/open/half-open state machine with configurable thresholds
- [ ] 3.2 retry.Backoff — exponential backoff with jitter and context cancellation (bound invariant tests)
- [ ] 3.3 ratelimit.Limiter — token bucket on Go timers (deterministic-clock tests, bench)


---

## Milestone 4 — HTTP middleware

The four production middleware, composable as a standard decorator chain

> **Agent guidance:** Claude Opus 4.8 · effort **high** — a well-trodden decorator shape,
> but Recoverer's panic paths and CORS preflight edge cases reward careful reasoning.

- [ ] 4.1 middleware.RequestID — extract-or-generate request ID into the context
- [ ] 4.2 middleware.Logger — request logging with duration and bytes-written stats
- [ ] 4.3 middleware.Recoverer — panic recovery with clean 500 responses
- [ ] 4.4 middleware.Cors — configurable CORS header handling


---

## Milestone 5 — Configuration & environment

Safe configuration ingestion from files and environment

> **Agent guidance:** Claude Sonnet 4.6 · effort **medium** — mostly mechanical parsing and
> typed fallbacks. Note: this milestone selects and pins the YAML parser under ADR-0004's
> budget (a review point, not a coding challenge).

- [ ] 5.1 config.Loader — JSON/YAML/env loading with validation hooks
- [ ] 5.2 env.GetDefault — typed env reads with safe fallbacks


---

## Milestone 6 — Structured logging

JSON logging wired for aggregation and context propagation

> **Agent guidance:** Claude Sonnet 4.6 · effort **medium** — thin, well-specified wrappers
> over log/slog and context propagation.

- [ ] 6.1 logger.Structured — JSON logger for ElasticSearch / Loki ingestion
- [ ] 6.2 logger.Context — logger fields carried in context.Context


---

## Milestone 7 — Caching & data helpers

TTL caching and transactional SQL ergonomics

> **Agent guidance:** Claude Opus 4.8 · effort **high** — the TTL cache owns a cleanup
> goroutine (leak- and race-sensitive, goleak-gated) and db.Transaction must be correct on
> panic/rollback paths; both fail quietly when wrong.

- [ ] 7.1 cache.InMemory — TTL cache with periodic cleanup goroutine (leak-checked, bench)
- [ ] 7.2 db.Transaction — auto-rollback transaction helper (panic-path tests)


---

## Milestone 8 — Validation & security

Tag-driven validation and password hashing

> **Agent guidance:** Claude Opus 4.8 · effort **high** — the reflection-based tag grammar
> is fiddly, and hashing is security-relevant: under the enterprise posture this milestone
> carries an ADR and the security-auditor's review (AGENTS.md §7/§10).

- [ ] 8.1 validator.Struct — tag-driven struct validation (required, email, min, max, oneof)
- [ ] 8.2 hash.HashPassword / hash.CheckPassword — bcrypt hashing and verification


---

## Milestone 9 — Diagnostics & lifecycle

Graceful shutdown, health, metrics, and the core utility pair

> **Agent guidance:** Claude Fable 5 · effort **high** — cross-platform signal handling
> (Windows differs), ordered shutdown coordination, and the zero-allocation BufferPool
> proof (testing.AllocsPerRun) span concurrency, portability, and performance at once.

- [ ] 9.1 lifecycle.GracefulShutdown — signal-coordinated ordered shutdown (SIGINT/SIGTERM)
- [ ] 9.2 health.Handler — dependency-probing health endpoint
- [ ] 9.3 metrics.Prometheus — latency/request-count middleware with Prometheus exposition
- [ ] 9.4 syncpool.BufferPool — bytes.Buffer pooling (zero steady-state allocations, bench)
- [ ] 9.5 errors.Wrap — stack-preserving error context helpers



---

## Spec Coverage Map

Tracks which spec section is fulfilled by which roadmap item(s). Every spec section has a
row with at least one fulfilling item and a status glyph. Legend: ⏳ not started · 🚧 in
progress · ✅ done · ❎ N/A.

| Spec § | Requirement | Roadmap items | Status |
|--------|-------------|---------------|--------|
| §1 | Objective & business context | 1.1; delivered progressively by M2–M9 | 🚧 |
| §2 | Functional requirements | 2.1–9.5 | ⏳ |
| §3 | Non-functional requirements | 1.3, 1.4 (gates live); per-feature from M2 | 🚧 |
| §4 | Logical architecture | 1.1, 1.6 (ADR-0003) | 🚧 |
| §5 | Public interface | 2.1–9.5 | ⏳ |
| §6 | Verification & test strategy | 1.2, 1.4 (framework + CI live); per-feature suites from M2 | 🚧 |
