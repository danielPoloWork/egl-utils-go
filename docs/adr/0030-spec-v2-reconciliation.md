# ADR-0030: Spec v2.0 reconciliation — hybrid adoption: additive deltas in v1.x, breaking deferred to /v2

- **Status:** Accepted
- **Date:** 2026-07-16
- **Deciders:** Maintainer (Daniel Polo — disposition chosen explicitly), architect agent
- **Related:** `docs/specs/02_spec_v2_gap_analysis.md` (the full delta matrix);
  `docs/specs/v2/` (the imported v2.0 draft + its ADR-001/002/003); `docs/specs/01_spec_utils.md`
  (the intake-frozen contract v1.0.0 certifies); ROADMAP Milestone 10; AGENTS.md §7 (spec/impl
  never drift), §11 (SemVer); spec §5 versioning surface ("MAJOR = any breaking change to these
  signatures **or their documented behavioral contracts**")

## Context

After v1.0.0 shipped, a revised specification (**v2.0**, "Reviewed draft", 2026-07-14) was found in
the untracked `.spec/` folder — authored two days after intake and never reconciled with the frozen
spec the library was built against. The gap analysis classified every delta. v1.0.0 carries a fresh
SemVer API-stability commitment, and the module's own versioning rule counts **documented behavioral
contracts** as part of the breaking surface — which constrains what can be adopted in v1.x.

The maintainer chose the **hybrid** disposition: adopt the non-breaking deltas in v1.x; everything
breaking waits for an (undecided) `/v2`.

## Decision

Three buckets, exhaustive over the gap analysis. Bucket 1 becomes **Milestone 10** in the ROADMAP,
one item per PR, released as **v1.1.0** when complete.

### 1 · Adopted in v1.x (additive, non-breaking) — Milestone 10

| M10 | Delta (v2 ref) | Shape of the addition |
|---|---|---|
| 10.1 | Governance (§7 of the gap analysis) | This PR: v2 draft imported verbatim to `docs/specs/v2/`; this ADR |
| 10.2 | Circuit-breaker observability (item 6) | `circuitbreaker.State()` (+ a `State` type with `String()`) — read-only, thread-safe |
| 10.3 | Programmatic shutdown (item 21) | `lifecycle.Trigger()` — unblocks a pending `WaitForSignals` (idempotent); satisfies the v2 §6 example without touching existing signatures |
| 10.4 | Rate-limit middleware (item 8) | `(*Limiter).Middleware() func(http.Handler) http.Handler` (429 on deny) + exported `ErrLimited` |
| 10.5 | Password-hash cost (item 20, §7) | Additive `hash.HashPasswordCost(pw, cost)` (accepted range 10–31 per v2's "min accepted 10"); godoc note recommending argon2id for new systems with the tag-and-rehash migration path; a cost-sizing benchmark so deployers can size the factor. **Security-relevant → extends ADR-0024/control C-4, auditor sign-off on that PR** |
| 10.6 | Config ↔ validator wiring (item 13) | Additive `config.WithStructValidation()` option running `validator.Struct` on the decoded value |
| 10.7 | Fuzzing (§7) | `FuzzConfigLoader` + `FuzzValidatorTags`, committed corpora, CI fuzz job with the 10-minute PR budget |
| 10.8 | Import-graph enforcement (§3) | `depguard` rules enforcing **ADR-0004's** allowlist (stdlib + `golang.org/x/*` everywhere; `gopkg.in/yaml.v3` only in `config`; `prometheus/*` only in `metrics`) + a `go mod graph` CI assertion |
| 10.9 | Coverage gate (§7) | CI-enforced coverage ≥ 85 % (raises the AGENTS.md §10 floor of 80 %) |
| 10.10 | NFR benchmark suite (§5) | Benchmarks for NFR-01/02/03/04/06 + `benchstat` methodology + a nightly workflow flagging >10 % regressions |
| 10.11 | Cache hardening (item 17, non-breaking parts) | The 1 000-cache create/close goleak test; the NFR-06 1 M-entry p99 benchmark; internal sharding **only if** that benchmark demands it (ADR-0021's own condition) |
| 10.12 | Pub/sub slow-subscriber policy (item 2, non-breaking part) | Additive `pubsub.WithDropOldest[T]()` option (default remains drop-newest with the observable drop handler) |
| 10.13 | Contrib probes (item 22 / v2 ADR-003) | Nested submodules `contrib/redishealth`, `contrib/pgxhealth` (own `go.mod`, independent tags) supplying `health.Check` probes — driver deps never touch the core module |

### 2 · Deferred to a `/v2` major (breaking under the v1.0.0 commitment)

`errors`→`errx` rename with opt-in `WithStack`/`[]Frame` (item 25) · `cache.Get` → `(V, bool)` and a
`New` alias (item 17) · `workerpool.Stop` → `Close` (item 1) · the pubsub API reshape —
ctx-subscription, `Publish(ctx) error`, `ErrSlowSubscriber` (item 2) · removing the Prometheus SDK
and `prometheus.Registerer` from the metrics API (item 23) · `WaitForSignals(timeout, sigs...)`
signature (item 21) · **bcrypt default cost 10 → 12** (item 20): API-compatible but a documented
behavioral contract (login latency ×4), so under spec §5's own rule it is major-only; the capability
ships additively in 10.5 (`HashPasswordCost(pw, 12)`). No `/v2` is scheduled by this ADR; if opened,
it follows Go's `/v2` module path rule.

### 3 · Deviations maintained (documented judgment calls, no change)

- **Proportional jitter + last-error-verbatim** in retry (v2: full jitter, attempt-wrapped) — kept
  per ADR-0011; both are documented contracts now under the v1 stability rule anyway.
- **Hand-rolled rate-limit engine** (v2 ADR-002: wrap `x/time/rate`) — kept per ADR-0012
  (injectable clock, deterministic tests); the public API already matches v2's shape, so an engine
  swap remains possible invisibly if ever wanted. 10.4 adds the ergonomics v2 actually asks for.
- **No hidden shutdown timeout** (v2: bounded deadline default) — kept per ADR-0025; the bounded
  behaviour is reachable today via `Shutdown(ctx)` with a deadline context, and 10.3's `Trigger()`
  satisfies the v2 example.
- **Get-enforced cache expiry** (sweeper as reclamation only) — kept per ADR-0021.
- **`gopkg.in/yaml.v3` in core** — v2 is internally contradictory here (item 13 requires YAML;
  v2 ADR-003 allows stdlib+`x/` only, where no YAML parser exists); ADR-0004's bounded budget stands
  as the pragmatic resolution. 10.8 makes that budget mechanically enforced.
- **slog default keys** (v2: RFC 3339 UTC/source tuning) — kept per ADR-0019; `WithReplaceAttr`
  remains the deferred escape hatch.

## Alternatives Considered

- **Freeze-only** (record deviations, adopt nothing) — cheapest, but discards genuinely valuable,
  risk-free v2 content (fuzzing, NFR verification, observability, enforcement). Rejected by the
  maintainer.
- **Full v2 adoption via an immediate `/v2`** — cleanest alignment, but obsoletes a one-day-old 1.0
  and forks the module path for corrections most consumers don't need urgently. Rejected for now;
  bucket 2 keeps the ledger for whenever a `/v2` is warranted.
- **Adopting the behavioral deltas in v1.x** (cost 12 default, full jitter, wrapped last error) —
  API-compatible, but spec §5's own versioning rule counts documented behavior as breaking surface;
  a fresh 1.0 should not open with contract erosion. Rejected; capabilities delivered additively
  where possible (10.5).

## Consequences

- The spec lineage is whole and governed: frozen v1 contract + imported v2 draft + gap analysis +
  this disposition; future spec revisions land as new imports with their own reconciliation.
- Milestone 10 (13 items) enters the ROADMAP under the standard cadence — one item per PR, full
  quality bar, v1.1.0 at completion (post-1.0 MINOR per §11).
- The v1.0.0 API-stability commitment survives intact; every bucket-2 item has a recorded home.
- ADR-0004's dependency budget gains mechanical enforcement (10.8) — the drift class that produced
  this reconciliation becomes harder to repeat.

## References

- `docs/specs/02_spec_v2_gap_analysis.md` · `docs/specs/v2/` (imported draft + ADR-001/002/003).
- ADR-0004, -0011, -0012, -0019, -0021, -0024, -0025 (the maintained deviations' rationales).
- AGENTS.md §7, §11; `docs/specs/01_spec_utils.md` §5 (versioning surface).
