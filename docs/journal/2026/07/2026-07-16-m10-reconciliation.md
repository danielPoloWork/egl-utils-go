# 2026-07-16 — Milestone 10 opens: spec v2 reconciliation (hybrid)

## What got done

- **Disposition decided by the maintainer** on the spec v2.0 gap analysis (PR #36, merged):
  **hybrid** — adopt the non-breaking deltas in v1.x; breaking deltas stay ledgered for a possible
  `/v2`. This session lays the governance foundation (roadmap item **10.1**):
- **Spec v2.0 imported under version control**: `.spec/` (untracked) copied **verbatim** to
  [`docs/specs/v2/`](../../specs/v2/) — `d4np-go.md` + its three ADRs, internal relative links
  intact — with a provenance README stating its normative status (the intake-frozen
  `01_spec_utils.md` remains the v1 contract; v2 deltas flow only through the reconciliation).
  Future spec revisions land as new imports with their own reconciliation, closing the root cause
  (a spec living outside the governed tree).
- **ADR-0030 — the reconciliation record**, three exhaustive buckets over the gap analysis:
  1. **Adopted in v1.x** → Milestone 10 (13 items): State(), Trigger(), Middleware()+ErrLimited,
     HashPasswordCost + argon2id note + cost benchmark, WithStructValidation, fuzzing + corpora +
     CI budget, depguard/go-mod-graph enforcement of ADR-0004's budget, 85% coverage gate, the NFR
     benchmark suite + nightly benchstat, cache mass-lifecycle test + NFR-06 bench
     (shard-only-if-proven), pubsub WithDropOldest, contrib/ submodules.
  2. **Deferred to `/v2`** (breaking under the v1.0.0 commitment, incl. spec §5's
     documented-behavior rule): errx rename, cache (V, bool), Stop→Close, pubsub API reshape,
     metrics without the Prometheus SDK, WaitForSignals(timeout, …), **and the bcrypt default
     10→12** (behavioral contract; the capability ships additively via HashPasswordCost in 10.5).
  3. **Deviations maintained** (each already argued in its ADR): proportional jitter + verbatim
     last error (0011), hand-rolled ratelimit engine (0012), no hidden shutdown timeout (0025),
     Get-enforced cache expiry (0021), yaml.v3 in core (v2's own YAML/zero-dep contradiction),
     slog default keys (0019).
- **ROADMAP Milestone 10** added (10.1 ☑ as this PR; 10.2–10.13 pending, per-item agent tags);
  README milestone table gains row 10 (in progress). Milestone releases as **v1.1.0** when complete
  (post-1.0 MINOR per §11).

## Where the project stands

v1.0.0 shipped (tag pushed; GitHub Release publish pending with the maintainer). M10 opened:
10.1 drafted on `docs/spec-v2-reconciliation`, awaiting the maintainer to open and merge (one PR at
a time). 12 items remain (10.2–10.13).

## How the next session resumes

Wait for the 10.1 PR to merge. Then proceed in roadmap order — **10.2 `circuitbreaker.State()`**
(lift the ADR-0010 deferral: exported `State` type + `String()`, thread-safe read of the
closed/open/half-open state; small ADR-0010 addendum or coverage inside ADR-0030 is already in
place, so the PR is code+tests+docs-sync only). Then 10.3 lifecycle.Trigger() (coordinator-scoped
trigger channel unblocking WaitForSignals — extend the existing seams), 10.4 ratelimit
Middleware()+ErrLimited, and onward. Standard footprint per PR: code, tests (goleak, 100%
coverage), CHANGELOG [Unreleased], ROADMAP checkbox, journal, lint. Portable Go under
`%TEMP%\go-portable`; `/v2` golangci-lint path; `-race` CI-only.
