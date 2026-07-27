# Roadmap — egl-utils-go

The project's plan as a numbered, checkbox-driven list. When an item completes in a PR,
flip its checkbox (`- [ ]` → `- [x]`) **in the same PR**. New work goes at the bottom of
its section with a fresh `<milestone>.<task>` number; never renumber.

- **Versioning start:** pre-1.0 milestone-driven.
- **Session journal:** see [`docs/journal/`](docs/journal/). Latest checkpoint:
  [2026-07-27 — Series namespace contract](docs/journal/2026/07/2026-07-27-series-namespace-contract.md).

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
- [x] 8.2 hash.HashPassword / hash.CheckPassword — bcrypt hashing and verification → [ADR-0024](docs/adr/0024-hash-password-design.md) — *agent: Opus 4.8 · high (as built) — completes Milestone 8; bcrypt default cost 10, per-hash salt, ErrPasswordTooLong (no truncation), constant-time verify → generic ErrMismatch; adds golang.org/x/crypto v0.48.0 (ring 2, floor-preserving pin); security-relevant → ADR + auditor sign-off + compliance C-4*


---

## Milestone 9 — Diagnostics & lifecycle

Graceful shutdown, health, metrics, and the core utility pair

> **Agent guidance:** Claude Fable 5 · effort **high** — cross-platform signal handling
> (Windows differs), ordered shutdown coordination, and the zero-allocation BufferPool
> proof (testing.AllocsPerRun) span concurrency, portability, and performance at once.

- [x] 9.1 lifecycle.GracefulShutdown — signal-coordinated ordered shutdown (SIGINT/SIGTERM) → [ADR-0025](docs/adr/0025-lifecycle-shutdown-design.md) — *agent: Fable 5 · xhigh (as built) — LIFO hooks (reverse-dependency order), run-all + errors.Join, exactly-once convergent Shutdown (concurrent callers wait and share the result), no hidden timeout (platform kill escalation bounds it), zero owned goroutines; injected signal seam makes tests deterministic on Windows (no kill(2))*
- [x] 9.2 health.Handler — dependency-probing health endpoint → [ADR-0026](docs/adr/0026-health-handler-design.md) — *agent: Opus 4.8 · medium (as built) — probes run concurrently with the request context, 200 all-pass / 503 any-fail; status-only JSON body (name + ok/fail, never the probe error — info-disclosure, threat model); loud panic on empty/dup name or nil probe*
- [x] 9.3 metrics.Prometheus — latency/request-count middleware with Prometheus exposition → [ADR-0027](docs/adr/0027-metrics-prometheus-design.md) — *agent: Opus 4.8 · high (as built) — request counter + latency histogram labelled (method, code); no path label and method normalized to a bounded set (cardinality DoS mitigation, threat model); adds prometheus/client_golang v1.23.2 (ring 3, ADR-0004, floor-preserving); one uncalled x/sys advisory knowingly kept to preserve the 1.24 floor*
- [x] 9.4 syncpool.BufferPool — bytes.Buffer pooling (zero steady-state allocations, bench) → [ADR-0028](docs/adr/0028-syncpool-bufferpool-design.md) — *agent: Opus 4.8 · high (as built) — sync.Pool of bytes.Buffer, reset-on-Put, discards buffers grown past a 64 KiB cap (memory-retention trap); AllocsPerRun zero-alloc assertion + bench (~17ns/0-alloc); adopts the Object Pool pattern (catalogue row 10)*
- [x] 9.5 errors.Wrap — stack-preserving error context helpers → [ADR-0029](docs/adr/0029-errors-wrap-design.md) — *agent: Opus 4.8 · medium (as built, completes the roadmap) — Wrap/Wrapf %w-transparent (errors.Is/As/Unwrap), one-time origin stack captured at the first wrap and inherited by later wraps, StackTracer + fmt.Formatter (%+v prints frames), Wrap(nil)=nil; package named errors imports stdlib as stderrors*


---

## Milestone 10 — Spec v2 reconciliation (v1.x additive adoption) — ✅ complete, released as v1.1.0

The non-breaking deltas of the imported spec v2.0 draft ([`docs/specs/v2/`](docs/specs/v2/)),
adopted per the hybrid disposition in [ADR-0030](docs/adr/0030-spec-v2-reconciliation.md) —
breaking deltas stay ledgered there for a possible `/v2`. Post-1.0: every item is additive under
the v1.0.0 API-stability commitment; the milestone released as
**[v1.1.0](docs/releases/v1.1.0.md)** on 2026-07-27, all 13 items delivered with no exported
signature changed.

Two findings are carried forward deliberately rather than silently absorbed, both from 10.10:
NFR-01's **0-allocs/op target is unachievable** (structural allocations in `context.WithValue`,
`Request.WithContext` and `Header.Set`), enforced as a ratchet budget at the measured floor and
needing a spec amendment; and `middleware.HeaderName` is **not Go's canonical header spelling**,
costing 2 allocations per request for no wire-format difference — measured but not changed, since
it is an API-visible constant under the v1 commitment.

> **Agent guidance:** Claude Opus 4.8 · effort **high** — additive API surfaces and CI plumbing;
> the NFR suite (10.10) and the contrib module topology (10.13) are the reasoning-heavy steps and
> carry their own tags.

- [x] 10.1 Governance — import the spec v2.0 draft verbatim under `docs/specs/v2/` and record the reconciliation disposition → [ADR-0030](docs/adr/0030-spec-v2-reconciliation.md) — *agent: Fable 5 · max (as built) — gap analysis + three-bucket disposition (adopt/defer/deviate)*
- [x] 10.2 circuitbreaker.State() — observable breaker state (v2 item 6; lifts the ADR-0010 deferral) → [ADR-0030](docs/adr/0030-spec-v2-reconciliation.md) — *agent: Opus 4.8 · medium (as built) — exported State type (StateClosed/Open/HalfOpen) + String(); (*Breaker).State() is a pure read-only observer that reflects the lazy time transition (open-past-cooldown reports half-open) without mutating state, advancing the generation, or admitting a probe*
- [x] 10.3 lifecycle.Trigger() — programmatic shutdown that unblocks WaitForSignals (v2 item 21, §6 example) → [ADR-0030](docs/adr/0030-spec-v2-reconciliation.md) — *agent: Opus 5 · medium (as built) — coordinator-scoped `triggered` channel closed once via sync.Once (idempotent, concurrency-safe); WaitForSignals selects over the signal channel and the trigger channel, so a Trigger arriving first latches rather than being lost; Register/Shutdown/WaitForSignals signatures unchanged*
- [x] 10.4 ratelimit.Middleware() + ErrLimited — 429-on-deny HTTP middleware over the existing engine (v2 item 8) → [ADR-0031](docs/adr/0031-ratelimit-middleware-design.md) — *agent: Opus 5 · medium (as built) — `(*Limiter).Middleware()` returns the house `func(http.Handler) http.Handler` decorator from the ratelimit package (spec §5 names it on the Limiter; net/http is stdlib so the L2 layer takes no new dependency); admits via Allow and never Wait, so a burst is shed rather than queued into parked goroutines; 429 + constant `Retry-After: ceil(1/rate)` precomputed per decorator, generic status-text body, no logging (a client-triggerable log line would be a flood amplifier); admit path 0 allocs; global-budget/per-client-fairness limitation documented (threat model, control C-5)*
- [x] 10.5 hash.HashPasswordCost (cost 10–31) + argon2id migration godoc note + cost-sizing benchmark (v2 item 20, §7) → [ADR-0032](docs/adr/0032-hash-password-cost-design.md) — *agent: Opus 5 · high (as built) — range validated LOCALLY, not delegated: bcrypt silently promotes sub-MinCost(4) values to the default and honours costs 4–9 verbatim (verified empirically, pinned by a test), so an out-of-range cost returns wrapped `ErrInvalidCost` and no hash; error not panic (unlike NewLimiter/Cors this has an error channel and runs per-call — `config.Load` precedent); `HashPassword` delegates at the default cost; adds `Cost()` so the mandated rehash-on-login note is actionable without importing bcrypt (the one deliberate widening of the item, flagged in the ADR); [cost-sizing report](docs/benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md) shows exact doubling (55 ms→887 ms for cost 10→14) and that verify costs the same as hash — the per-login DoS trade-off, mitigated by 10.4's middleware (C-5); control C-4 extended + 3 threat-model rows*
- [x] 10.6 config.WithStructValidation() — wire validator.Struct into config.Load as an option (v2 item 13) → [ADR-0033](docs/adr/0033-config-struct-validation.md) — *agent: Opus 5 · low (as built) — opt-in Option (implicit enablement rejected: a struct with no tags passes vacuously, so it would imply a guarantee that isn't there); **tags run BEFORE Validator and a tag failure skips Validate**, so a Validate method may assume every field is individually well-formed and check only cross-field invariants; validator's panics on tag misuse/non-struct T are NOT softened into errors (that would override ADR-0023 from outside). **Establishes the module's FIRST internal package edge (config → validator, L2 → L2) — see 10.8***
- [x] 10.7 Fuzzing — FuzzConfigLoader + FuzzValidatorTags, committed corpora, CI fuzz job (10-min budget) (v2 §7) → [ADR-0034](docs/adr/0034-fuzzing-strategy.md) — *agent: Opus 5 · high (as built) — `FuzzValidatorTags` could not use the naive "never panics" property because `validator.Struct` **panics by contract** on tag misuse (ADR-0023); the invariant is instead "any panic is a `validator: `-prefixed string and **never a `runtime.Error`**, any error is a ValidationErrors" — which also catches a malformed tag being *silently accepted*. Tag injection needs `reflect.StructOf`, whose type cache **never evicts**, so the tag space is deliberately bounded to ≤3 fragments from a 16-entry table (unbounded tag text would OOM the runner, not fuzz better). Corpus is hand-authored around hostile documents (YAML alias bombs, 200-deep nesting, BOM, invalid UTF-8, NUL, `${` expansions) rather than committed mutation artifacts. **Found and fixed a real contract violation: `Load` promised the zero T on error but returned the partially decoded struct** — a half-configured value escaping behind an error. Ran clean: 1.65M execs (validator), ~51k (config, file-write bound)*
- [x] 10.8 Import-graph enforcement — depguard rules for the ADR-0004 allowlist (yaml.v3→config only, prometheus→metrics only) + go mod graph CI assertion (v2 §3) → [ADR-0035](docs/adr/0035-import-graph-enforcement.md) — *agent: Opus 5 · medium (as built) — enforced TWICE with different reach: depguard per file (each governed module confined to its owning package, driver/redis SDKs denied, sibling imports denied except config which gets a `strict` allowlist of stdlib+yaml+validator) **plus** `tools/import_graph_lint.py` over the resolved graph (direct go.mod requires vs the rings, per-package direct imports, internal edge set, `go mod graph` vs the manifest). The second exists because **depguard does NOT report a blank import of a sibling** (verified — `import _ ".../cache"` passes while blank `yaml.v3` is caught), cannot see a new direct requirement, and cannot notice a dead exception. The `config → validator` edge is allowlisted as an exception, and the tool fails if it ever *disappears*. Rejected pinning the full `go list -deps` closure (every upstream bump would fail CI for a decision nobody made). Each rule verified by deliberate violation.*
- [x] 10.9 CI coverage gate ≥ 85% (v2 §7; raises the §10 floor) → [ADR-0036](docs/adr/0036-coverage-gate.md) — *agent: Opus 5 · low (as built) — enforced **per package**, not as a module-wide average: with 16 of 21 packages at 100% the module figure sits ~99%, so a package could rot to 50% and the gate would still pass — it could not fail. Per package the floor binds the weakest. Measured low-water mark is **fanout 93.3%** (then fanin 95.7, pubsub 96.4, retry 97.7, ratelimit 98.1); the 8-point margin is deliberate slack, since a gate that fires on noise gets disabled. `tools/coverage_gate.py` + a `coverage` CI job; no-statement packages skipped, not counted as zero; verified by deliberate violation. Also discharges AGENTS.md §10's outstanding "finalized in an ADR" and corrects its stale "module floor 1.24" → 1.25*
- [x] 10.10 NFR benchmark suite — NFR-01/02/03/04/06 benches + benchstat methodology + nightly regression workflow (>10% flags) (v2 §5) → [ADR-0037](docs/adr/0037-nfr-benchmark-methodology.md), [report](docs/benchmarks/2026-07-26-nfr-suite.md) — *agent: Opus 5 · max (as built) — split by KIND of claim: hardware-independent NFRs are **hard gates** in the test suite (NFR-01 allocations, NFR-04 ±1% accuracy exact via the fake clock, NFR-05), hardware-dependent throughput/latency is **measured and reported**, not gated (shared runners move microbenchmarks >10% between identical runs; a gate that fires on noise gets switched off). **Verdicts: NFR-03 met 12.7x (6.33M delivered/s), NFR-02 throughput met 4.4x (4.42M tasks/s), NFR-04 met EXACTLY (0.0000%), NFR-05 already gated; NFR-01 latency met (938ns of 1µs) but its 0-alloc target is UNACHIEVABLE (8 allocs — context.WithValue + r.WithContext + Header.Set are structural), replaced by an enforced ratchet budget; NFR-06 NOT MET (Get 79ns but 90/10 mix 350ns — a single RWMutex serialises readers on every Set: this is 10.11's sharding evidence); NFR-02/06 p99 UNVERIFIED — on Windows 100% of adjacent time.Now/Since pairs read 0ns, so the suite reports `tail-unmeasurable` rather than clock artifacts.** Follow-up found: `middleware.HeaderName` is non-canonical (`X-Request-ID` vs Go's `X-Request-Id`), costing 2 allocs/request in CanonicalMIMEHeaderKey for no wire-format difference — left to the maintainer as an API-visible change*
- [x] 10.11 cache hardening — 1 000-cache create/close goleak test + NFR-06 1M-entry p99 bench; shard internally only if the bench demands (v2 item 17) → [ADR-0038](docs/adr/0038-cache-sharding.md) — *agent: Opus 5 · medium (as built) — **the bench demanded it**: 10.10 measured 349.8 ns for the 90/10 mix vs 78.9 ns read-only, all from one `sync.RWMutex` serialising readers behind every `Set`. Sharded into **32 independently locked shards** keyed by `maphash.Comparable` with a **per-cache seed**; **NFR06Mixed 349.8 → 46.6 ns (7.5x), now under the 200 ns target**. Uncontended ops pay ~5 ns for the hash (GetHit 27.2→32.9, GetMiss 15.1→20.1, Set 52.0→60.5) — the trade is recorded, not hidden. **ONE sweeper per cache preserved** (per-shard sweepers would be 32 000 goroutines for 1 000 caches) and now pinned by `TestThousandCachesOwnOneGoroutineEach`; thousand-cache create/use/close, concurrent lifecycle, and 3-way concurrent idempotent Close all goleak-clean. `removeExpired` sweeps shard-by-shard so no lock spans the keyspace — harmless because ADR-0021 has `Get` enforce expiry. No API change, no dependency change, 100% coverage*
- [x] 10.12 pubsub.WithDropOldest — additive slow-subscriber policy option (default stays drop-newest + handler) (v2 item 2) → [ADR-0039](docs/adr/0039-pubsub-drop-oldest.md) — *agent: Opus 5 · medium (as built) — opt-in broker Option; on a full buffer the **oldest buffered** message is evicted so the subscriber sees the freshest (state-like streams: gauges, price ticks, progress). **Best-effort BY CONSTRUCTION**: evicting from a channel is receive-then-send, and a subscriber or publisher can act between them — retrying has no bound and would break ADR-0006's "Publish never blocks", so a lost race degrades to drop-newest for that one message. **The drop handler reports the EVICTED message** (the one actually lost) — reporting the incoming one would count safely-arrived messages as drops and break the accounting. **Invariant preserved under both policies: per subscription every message is delivered or reported dropped exactly once** (NFR-03's benchmark asserts it). Saturated-path cost 74.9 → 138.4 ns/op (~63 ns, only where messages are already being lost); NFR-03 throughput unchanged at ~6.4M delivered/s. No-op for rendezvous subscriptions (nothing to evict). Rejected: retry-until-success (unbounded), a per-subscription lock (adds a lock to the hot path to fix a race whose only cost is which message drops), a per-subscription policy (Subscribe is frozen), an enum option. **pubsub 96.4% → 100% coverage** — the gap was the option validators' panic branches*
- [x] 10.13 contrib/ nested submodules — contrib/redishealth + contrib/pgxhealth (own go.mod, independent tags) supplying health.Check probes (v2 item 22 / ADR-003) → [ADR-0040](docs/adr/0040-contrib-submodules.md) — *agent: Opus 5 · max (as built) — each submodule **requires the released core (v1.0.0) with NO `replace` and NO `go.work`**: a `replace` is ignored for dependents so CI would validate a configuration no consumer gets, and a committed workspace would switch all eight root CI jobs into workspace mode for the sake of one frozen struct. Exported constructors take the **driver's own type** (a consumer shouldn't write an adapter) while internals use a one-method interface, so every branch is testable to 100% without a live server; each module also constructs a real client/pool to prove the exported signature accepts the concrete types (neither dials eagerly). **Verified the core is untouched:** root go.mod/go.sum unchanged, `go list ./...` returns no contrib package, `go list -deps ./...` contains no redis/jackc path. **Extended the three things that silently ignore a nested module:** `import_graph_lint.py` now fails if a contrib dir holds Go files without a go.mod (the silent failure — those files join the root module and drag the driver in while every `./...` check keeps passing) and runs FIRST since later checks otherwise die on an opaque `go list` error; `coverage_gate.py` measures 23 packages (21 core + 2 contrib, both 100%); CI grows a `contrib` matrix job (build, vet, -race, gofumpt, golangci-lint, govulncheck) and Dependabot an entry per submodule. Shared `contrib/.golangci.yml` shadows the root depguard, which — verified — forbids the driver import *and* the health import a contrib module exists to make. **Milestone 10 complete**; contrib modules are NOT released by the v1.1.0 tag*


---

## Milestone 11 — Series namespace contract — ✅ complete

Governance only: no code, no version bump, no consumer-visible change. The series' shared
identity is restated as the **logical namespace `it.d4np.utils.<component>`** rather than
ADR-0002's physical tree, which two of the three scaffolded sibling repositories had already
abandoned independently — each because its language binds names to paths differently. Go
realizes the namespace at the module root, so imports are unchanged.

The milestone also discharges the **regeneration caveat ADR-0003 left open**: the manifest that
the factory re-renders from still asserted that packages live under `src/main/go/it/d4np/utils`,
so a regeneration would have silently re-imposed the layout ADR-0003 removed.

Pulling that thread exposed a second, unrelated class of drift, which 11.2 closes: the **frozen v1
specification had diverged from the as-built project in four places**. Three were facts that had
gone stale and are amended in place; the fourth — §3's compatibility clause — is a governance rule
that was *replaced* rather than drifted, and takes an ADR instead ([ADR-0042](docs/adr/0042-post-1.0-compatibility-contract.md)).
The spec now carries a dated **Amendments** block so that no post-freeze edit is silent.

> **Agent guidance:** Claude Opus 5 · effort **max** — a one-way, series-wide identity decision
> whose alternatives (vanity module path, GitHub organisation) are breaking and hard to unwind
> once published; the reasoning, not the diff, is the deliverable.

- [x] 11.1 Record the series logical namespace `it.d4np.utils.<component>` and close the ADR-0003
      regeneration caveat → [ADR-0041](docs/adr/0041-series-logical-namespace.md) — *agent: Opus 5 · max (as built) — the decision reframes ADR-0002's premise: a **physical** layout cannot be a cross-language contract, because in most of these languages the layout is not free (Maven dictates Java's, the include model dictates C/C++'s, the module system dictates Go's), whereas a **namespace** can be. Verified rather than assumed: `egl-util-cpp` keeps the tree, `egl-utils-c` rejected it in its own ADR-0002 (`d4np/<module>/` at the root), `egl-utils-java` is unscaffolded — so the tree was 1-for-3, not the norm. **Module path deliberately NOT changed**: the two options that would render the namespace literally in Go source (vanity `go.d4np.it/utils`, org `github.com/d4np/…`) are both module-*identity* changes and therefore breaking, and the vanity one converts a documentation preference into a permanent supply-chain obligation (a domain that must resolve and a `go-import` tag that must be served for as long as anyone runs `go get`). Recorded in the ADR-0030 ledger as declined-with-a-condition: **the move is free at a `/v2` boundary and only there**, since consumers rewrite every import at such a boundary anyway. `orchestrator/project.yaml` amended in place with a dated note (the interview record still shows what was asked; the generator can no longer act on it). **Two findings carried forward, neither fixed here:** the EADOS bundle's `go.yaml` profile still asserts the tree but `.eados-core/` is gitignored, so the correction must be carried upstream; and `egl-util-cpp` ships `it/d4np/util` (singular) against this contract's `utils` — a series-level call, not this repo's to make*
- [x] 11.2 Reconcile the frozen v1 spec §3/§6 with as-built reality — language floor, coverage gate, leak-detection mechanism, and the post-1.0 compatibility contract → [ADR-0042](docs/adr/0042-post-1.0-compatibility-contract.md) — *agent: Opus 5 · high (as built) — the sweep that began as "fix the stale 1.24 floor" turned up **four** divergences in a document declared frozen, and they are not the same kind of thing. Three were **facts that had drifted** and were amended in place under the spec's own divergence rule: the language floor (1.24 → 1.25, false since the M9/M10 dependency work), the coverage floor (80% → **85% per package** — "per package" is the operative half, since with most packages at 100% a module-wide average could never fail; stricter since 10.9/ADR-0036), and the §3/§6 hedge promising "an in-repo stack-based guard until ROADMAP 2.6 lands the test-only deps", which described a state that ended in M2. The fourth took the **ADR branch** instead: §3's compatibility clause made a breaking change *mergeable with a MAJOR-intent note*, while the v1.0.0 commitment makes it not mergeable into v1.x at all — a rule that was **replaced**, not a number that drifted, so the original text is **struck in place and retained** rather than rewritten, because overwriting it would erase the evidence that the promise to consumers ever changed. **ADR-0042 retires the MAJOR-intent note and makes ADR-0030 §2's ledger the only sanctioned destination for a breaking change** — the ledger stops being a filing cabinet and becomes the enforcement point. The frozen spec now carries a dated **Amendments** block that distinguishes the two kinds of post-freeze edit, so future divergences inherit the split. No code, tooling or CI change: the rule has been in force since v1.0.0 (bucket 2's seven deferred deltas are the proof) and is only now written down*

---

## Spec Coverage Map

Tracks which spec section is fulfilled by which roadmap item(s). Every spec section has a
row with at least one fulfilling item and a status glyph. Legend: ⏳ not started · 🚧 in
progress · ✅ done · ❎ N/A.

| Spec § | Requirement | Roadmap items | Status |
|--------|-------------|---------------|--------|
| §1 | Objective & business context | 1.1; delivered by M2–M9 | ✅ |
| §2 | Functional requirements | 2.1–9.5 (all 25 features) | ✅ |
| §3 | Non-functional requirements | 1.3, 1.4 (gates live); per-feature from M2; 11.2 (ADR-0042) | ✅ |
| §4 | Logical architecture | 1.1, 1.6 (ADR-0003); 11.1 (ADR-0041) | ✅ |
| §5 | Public interface | 2.1–9.5 | ✅ |
| §6 | Verification & test strategy | 1.2, 1.4 (framework + CI live); per-feature suites M2–M9; 11.2 | ✅ |
