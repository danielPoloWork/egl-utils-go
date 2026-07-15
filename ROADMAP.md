# Roadmap — egl-utils-go

The project's plan as a numbered, checkbox-driven list. When an item completes in a PR,
flip its checkbox (`- [ ]` → `- [x]`) **in the same PR**. New work goes at the bottom of
its section with a fresh `<milestone>.<task>` number; never renumber.

- **Versioning start:** pre-1.0 milestone-driven.
- **Session journal:** see [`docs/journal/`](docs/journal/). Latest checkpoint:
  [2026-07-15 — M8 opens: validator.Struct](docs/journal/2026/07/2026-07-15-m8-validator.md).

### Agent guidance (model × effort)

Each milestone carries an advisory **Agent guidance** line — the milestone default — and
each roadmap item carries a per-step tag (`*agent: <model> · <effort>*`) naming the Claude
model and effort level recommended to implement that step. Tags may sit above or below the
milestone default; deviating tags carry a short rationale. On completed items (`[x]`) the
tag records the tier actually used. Model lineup (current as of 2026-07): **Claude Fable 5**
(strongest reasoning) for concurrency-critical and one-way API-design work; **Claude
Opus 4.8** for subtle but well-trodden correctness work; **Claude Sonnet 5** for
well-specified integration and mechanical work. Claude Haiku 4.5 is deliberately unused:
every item ships under the full quality bar (AGENTS.md §10) and Haiku lacks the `effort`
control. Effort scale (Claude Code): `low · medium · high · xhigh · max` — `max` where
correctness outweighs cost (leak/race proofs, one-way design decisions), `xhigh` for the
hardest coding steps, `high` the standard tier, `medium`/`low` for mechanical or trivial
work. Advisory only — the quality bar (AGENTS.md §10) and the human review gate remain the
arbiter regardless of which model wrote the code.

---

## Milestone 1 — Project bootstrap & CI

The thinnest slice that compiles, tests, and ships under the full quality bar.

> **Agent guidance:** Claude Fable 5 · effort **max** — completed with this tier; the
> layout decision (ADR-0003) shapes every consumer import and was one-way once published.

- [x] 1.1 Lay down the build system (go build (go modules)) and a buildable skeleton — root
      layout per ADR-0003, which supersedes the `src/main/go/it/d4np/utils/` tree. — *agent: Fable 5 · max (as built)*
- [x] 1.2 Wire the test framework (go test (+ testify; rapid for property tests)) with one passing smoke test —
      co-located per ADR-0003 (`version_test.go`). — *agent: Fable 5 · max (as built)*
- [x] 1.3 Add formatter + linter configs (gofumpt (gofmt superset), golangci-lint (govet, staticcheck, errcheck, revive, gosec)) at the repo root. — *agent: Fable 5 · max (as built)*
- [x] 1.4 Stand up the CI matrix (Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.24)) with build + test + format + lint. — *agent: Fable 5 · max (as built)*
- [x] 1.5 Seed the version constant (const Version = "X.Y.Z") in `version.go`. — *agent: Fable 5 · max (as built)*
- [x] 1.6 Record the Go module layout decision as an ADR: module path github.com/danielPoloWork/egl-utils-go, go.mod placement vs the normative src/main/go tree, consumer import ergonomics → [ADR-0003](docs/adr/0003-adopt-idiomatic-go-root-layout.md). — *agent: Fable 5 · max (as built)*
- [x] 1.7 Record the dependency policy as an ADR: runtime = stdlib + golang.org/x/* + vetted few (prometheus/client_golang, YAML parser); test-only = testify, goleak, rapid; govulncheck as the supply-chain gate → [ADR-0004](docs/adr/0004-runtime-dependency-policy.md). — *agent: Fable 5 · max (as built)*


---

## Milestone 2 — Concurrency primitives

The five channel-native concurrency building blocks, leak-free and race-clean

> **Agent guidance:** Claude Fable 5 · effort **max** — goroutine lifecycle correctness,
> leak/race-freedom proofs, generics API design, and property-based tests are the hardest
> correctness surface in the project; use the strongest tier.

- [x] 2.1 workerpool.Pool — bounded-queue goroutine pool with Submit/Stop contract (leak, race, bench coverage) → [ADR-0005](docs/adr/0005-workerpool-design.md) — *agent: Fable 5 · max (as built)*
- [x] 2.2 pubsub.Broker — filtered-subscription in-memory broker (property tests for delivery) → [ADR-0006](docs/adr/0006-pubsub-design.md) — *agent: Fable 5 · max (as built)*
- [x] 2.3 fanin.Merge — multi-channel merge (completeness property tests) → [ADR-0007](docs/adr/0007-fanin-design.md) — *agent: Fable 5 · high (as built)*
- [x] 2.4 fanout.Split — parallel channel distribution (completeness property tests) → [ADR-0008](docs/adr/0008-fanout-design.md) — *agent: Fable 5 · high (as built)*
- [x] 2.5 semaphore.Weighted — weighted admission wrapper over x/sync/semaphore → [ADR-0009](docs/adr/0009-semaphore-design.md) — *agent: Opus 4.8 · low (as built) — thin adapter; the concurrency is delegated to x/sync*
- [x] 2.6 Adopt the ADR-0004 test-only dependencies (goleak, testify, rapid): run go mod tidy from a Go-equipped environment to produce go.sum, then migrate the interim in-repo leak assertions to goleak — *agent: Opus 4.8 · low (as built) — mechanical dependency wiring and test migration*


---

## Milestone 3 — Resilience patterns

Fail-fast, retry, and rate-limit protection for outbound calls

> **Agent guidance:** Claude Fable 5 · effort **high** — state machines under concurrency
> (closed/open/half-open), backoff bound invariants with jitter, and deterministic-clock
> testing are subtle; timing bugs here surface only under load.

- [x] 3.1 circuitbreaker.Breaker — closed/open/half-open state machine with configurable thresholds → [ADR-0010](docs/adr/0010-circuitbreaker-design.md) — *agent: Fable 5 · xhigh (as built)*
- [x] 3.2 retry.Backoff — exponential backoff with jitter and context cancellation (bound invariant tests) → [ADR-0011](docs/adr/0011-retry-design.md) — *agent: Fable 5 · xhigh (as built)*
- [x] 3.3 ratelimit.Limiter — token bucket on Go timers (deterministic-clock tests, bench) → [ADR-0012](docs/adr/0012-ratelimit-design.md) — *agent: Fable 5 · xhigh (as built)*


---

## Milestone 4 — HTTP middleware

The four production middleware, composable as a standard decorator chain

> **Agent guidance:** Claude Opus 4.8 · effort **high** — a well-trodden decorator shape,
> but Recoverer's panic paths and CORS preflight edge cases reward careful reasoning.

- [x] 4.1 middleware.RequestID — extract-or-generate request ID into the context → [ADR-0013](docs/adr/0013-middleware-requestid-design.md) — *agent: Opus 4.8 · high (as built) — first HTTP middleware: adopts the Decorator pattern and crosses the first untrusted-input trust boundary, so it carries the pattern ADR, the threat-model pass, and compliance C-2; heavier than the medium tag anticipated*
- [x] 4.2 middleware.Logger — request logging with duration and bytes-written stats → [ADR-0014](docs/adr/0014-middleware-logger-design.md) — *agent: Opus 4.8 · high (as built) — status/bytes capture via an Unwrap-aware responseRecorder, status-derived levels, path-only logging (extends the threat model's Info-disclosure row, compliance C-2)*
- [x] 4.3 middleware.Recoverer — panic recovery with clean 500 responses → [ADR-0016](docs/adr/0016-middleware-recoverer-design.md) — *agent: Opus 4.8 · high (as built) — panic-to-clean-500 with no stack/panic leaked to the client (info-disclosure, C-2), server-side Error log via slog.Default (value + stack + request_id), http.ErrAbortHandler re-panicked, committed responses left intact; backfilled ADR-0015 (enterprise posture) to close the referenced-but-unwritten record*
- [x] 4.4 middleware.Cors — configurable CORS header handling → [ADR-0017](docs/adr/0017-middleware-cors-design.md) — *agent: Opus 4.8 · high (as built) — completes Milestone 4; CorsConfig deny-by-default, terminal 204 preflight, exact-origin echo + Vary, header/method reflection, loud panic on the Fetch-forbidden credentials+wildcard combo (new compliance control C-3)*


---

## Milestone 5 — Configuration & environment

Safe configuration ingestion from files and environment

> **Agent guidance:** Claude Sonnet 5 · effort **medium** — mostly mechanical parsing and
> typed fallbacks. Note: this milestone selects and pins the YAML parser under ADR-0004's
> budget (a review point, not a coding challenge).

- [x] 5.1 config.Loader — JSON/YAML/env loading with validation hooks → [ADR-0018](docs/adr/0018-config-loader-design.md) — *agent: Opus 4.8 · low (as built) — generic Load[T], extension-driven format, ${VAR} env expansion, Validator-interface hook; selected + pinned gopkg.in/yaml.v3 (already an indirect dep) under ADR-0004's budget*
- [x] 5.2 env.GetDefault — typed env reads with safe fallbacks — *agent: Opus 4.8 · low (as built) — completes Milestone 5; GetDefault + GetInt/GetBool/GetDuration, unset/empty/malformed all fall back silently (spec's "safe fallback" contract); trivial, no ADR (routine implementation, ADR §7)*


---

## Milestone 6 — Structured logging

JSON logging wired for aggregation and context propagation

> **Agent guidance:** Claude Sonnet 5 · effort **medium** — thin, well-specified wrappers
> over log/slog and context propagation.

- [x] 6.1 logger.Structured — JSON logger for ElasticSearch / Loki ingestion → [ADR-0019](docs/adr/0019-logger-structured-design.md) — *agent: Opus 4.8 · low (as built) — NewStructured returns a slog JSONHandler-backed *slog.Logger; WithWriter/WithLevel(Leveler)/WithSource/WithAttrs; slog default keys kept as the aggregator lingua franca; composes with middleware.Logger*
- [x] 6.2 logger.Context — logger fields carried in context.Context → [ADR-0020](docs/adr/0020-logger-context-design.md) — *agent: Opus 4.8 · low (as built) — completes Milestone 6; Field = slog.Attr alias + constructors, WithFields accumulates copy-on-write under an unexported key, FromContext enriches slog.Default; composes with NewStructured via slog.SetDefault*


---

## Milestone 7 — Caching & data helpers

TTL caching and transactional SQL ergonomics

> **Agent guidance:** Claude Opus 4.8 · effort **high** — db.Transaction must be correct on
> panic/rollback paths and fails quietly when wrong. The TTL cache (7.1) owns a cleanup
> goroutine (leak- and race-sensitive, goleak-gated) and rides the concurrency tier — see
> its per-item tag.

- [x] 7.1 cache.InMemory — TTL cache with periodic cleanup goroutine (leak-checked, bench) → [ADR-0021](docs/adr/0021-cache-inmemory-design.md) — *agent: Fable 5 · high (as built) — expiry enforced by Get (stale reads impossible regardless of sweep schedule); one sweeper goroutine, sync.Once Close, goleak-gated; fake-clock boundary tests; 0 allocs/op hot paths (~28ns Get)*
- [x] 7.2 db.Transaction — auto-rollback transaction helper (panic-path tests) → [ADR-0022](docs/adr/0022-db-transaction-design.md) — *agent: Opus 4.8 · high (as built) — completes Milestone 7; commit on nil, rollback+return on error (errors.Join if rollback fails), rollback+re-panic on panic; context-governed begin; loud nil; fake database/sql driver in tests (no sqlmock, ADR-0004)*


---

## Milestone 8 — Validation & security

Tag-driven validation and password hashing

> **Agent guidance:** Claude Opus 4.8 · effort **high** — the reflection-based tag grammar
> is fiddly, and hashing is security-relevant: under the enterprise posture this milestone
> carries an ADR and the security-auditor's review (AGENTS.md §7/§10).

- [x] 8.1 validator.Struct — tag-driven struct validation (required, email, min, max, oneof) → [ADR-0023](docs/adr/0023-validator-struct-design.md) — *agent: Opus 4.8 · xhigh (as built) — hand-rolled reflection (no framework, ADR-0004); literal rules (no implicit optional), rune-length min/max, nested-struct recursion with dotted paths, full aggregation via ValidationErrors; data violations returned, tag-misuse panics (two channels kept separate)*
- [ ] 8.2 hash.HashPassword / hash.CheckPassword — bcrypt hashing and verification — *agent: Opus 4.8 · high*


---

## Milestone 9 — Diagnostics & lifecycle

Graceful shutdown, health, metrics, and the core utility pair

> **Agent guidance:** Claude Fable 5 · effort **high** — cross-platform signal handling
> (Windows differs), ordered shutdown coordination, and the zero-allocation BufferPool
> proof (testing.AllocsPerRun) span concurrency, portability, and performance at once.

- [ ] 9.1 lifecycle.GracefulShutdown — signal-coordinated ordered shutdown (SIGINT/SIGTERM) — *agent: Fable 5 · xhigh — cross-platform signal handling plus ordered phases; the hardest of M9*
- [ ] 9.2 health.Handler — dependency-probing health endpoint — *agent: Opus 4.8 · medium — concurrent probes with per-check timeouts; moderate and well-trodden*
- [ ] 9.3 metrics.Prometheus — latency/request-count middleware with Prometheus exposition — *agent: Sonnet 5 · high — client_golang integration; label cardinality is the review point*
- [ ] 9.4 syncpool.BufferPool — bytes.Buffer pooling (zero steady-state allocations, bench) — *agent: Opus 4.8 · high — sync.Pool oversized-buffer retention trap plus the AllocsPerRun proof*
- [ ] 9.5 errors.Wrap — stack-preserving error context helpers — *agent: Opus 4.8 · medium — %w-compatible wrapping with one-time stack capture; well-specified*



---

## Spec Coverage Map

Tracks which spec section is fulfilled by which roadmap item(s). Every spec section has a
row with at least one fulfilling item and a status glyph. Legend: ⏳ not started · 🚧 in
progress · ✅ done · ❎ N/A.

| Spec § | Requirement | Roadmap items | Status |
|--------|-------------|---------------|--------|
| §1 | Objective & business context | 1.1; delivered progressively by M2–M9 | 🚧 |
| §2 | Functional requirements | 2.1–9.5 | 🚧 |
| §3 | Non-functional requirements | 1.3, 1.4 (gates live); per-feature from M2 | 🚧 |
| §4 | Logical architecture | 1.1, 1.6 (ADR-0003) | 🚧 |
| §5 | Public interface | 2.1–9.5 | 🚧 |
| §6 | Verification & test strategy | 1.2, 1.4 (framework + CI live); per-feature suites from M2 | 🚧 |
