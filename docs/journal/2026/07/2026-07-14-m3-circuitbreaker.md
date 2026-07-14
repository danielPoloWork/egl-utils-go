# 2026-07-14 — Milestone 3 opens: circuitbreaker

## What got done

- **Roadmap 3.1 `circuitbreaker.Breaker`** (branch `feat/circuitbreaker`, draft PR #14,
  ADR-0010, patterns row 6): the closed/open/half-open breaker inside the spec-frozen
  `New`/`Do`/`ErrOpen` surface. Design pillars: consecutive-failure tripping (completion
  order, success resets); **lazy timerless transitions** (the open → half-open move is
  evaluated on the next admission against an injectable clock — the breaker owns no
  goroutines and no timers, so the zero-leak NFR holds by construction); **half-open probe
  budget = success threshold** (`successes + inFlight ≤ successThreshold`, so closing
  implies zero in flight); **generation-guarded accounting** (every transition bumps a
  generation; outcomes of calls that straddle a transition are discarded — stale closed
  calls and orphaned probes cannot corrupt the next episode's counters). Context-done
  calls return `ctx.Err()` unrun and uncounted; panics count as failure and propagate.
  Tests: black-box contract suite, fake-clock boundary/stale-generation suite, a rapid
  property against a sequential reference model, an 8-worker race hammer — **100%
  statement coverage** on the package.
- **Healed the red master inherited from 2.6** (folded into PR #14 with a callout): the
  squash-merge of PR #13 landed without the maintainer-side `go mod tidy` handoff, so
  `go.sum` held only the x/sync entries and `go.mod` lacked the indirect block — master's
  own CI run (29205790037) has been red since 2026-07-12, and PR #14's first push
  reproduced it verbatim. Fixed canonically, not by hand: downloaded the **portable Go
  1.26.5 toolchain** (zip under `%TEMP%\go-portable`, SHA256 verified against go.dev) and
  ran `go mod tidy` — +7 `go.mod` lines (indirect block), +20 `go.sum` lines.
- **First-ever local verification** on this machine, ending the write-blind era: `go
  build`, `go vet`, the full `go test ./...` suite (all packages green first run),
  `gofumpt -l` clean, per-package coverage. Only `-race` stays CI-only (needs cgo/gcc on
  Windows); the Linux race job covers it.
- README milestone-table sync M2 → done (left behind by 2.6), M3 → in progress; spec-map
  §2/§5 → 🚧.

## Where the project stands

PR #14 (draft) carries 3.1 plus the go.sum heal; once its CI matrix is green the
maintainer reviews and squash-merges, and master is green again for the first time since
the 2.6 merge. Version is still v0.1.0: **M2 completed at PR #13, so a v0.2.0 release PR
(MINOR per AGENTS.md §11) is available to cut whenever the maintainer wants** — the
maintainer's instruction this session was to continue with 3.1, so the release was not
cut here.

## How the next session resumes

Wait for PR #14 to merge (one PR at a time). Then either cut the pending v0.2.0 release
PR (M2 milestone bump — maintainer's call), or continue Milestone 3 with **3.2
retry.Backoff** (roadmap tier: Opus 4.8 · high; property-test the bound invariants,
honor context cancellation) on a fresh branch from `master`. The portable toolchain
under `%TEMP%\go-portable` makes local build/test/format verification possible — re-download
via go.dev if the temp dir was cleaned (checksum-verify the zip).

## Addendum 2 — roadmap 3.3 ratelimit.Limiter; Milestone 3 complete (same day)

PR #15 (retry) merged (`5c49670`). Item 3.3 on `feat/ratelimit` (draft PR #16, ADR-0012,
patterns row 8) closes Milestone 3: the spec-frozen `NewLimiter(rate float64, burst
int)` / `Allow()` / `Wait(ctx)` as a **hand-rolled lazy token bucket** — built, not
wrapped, because unlike 2.5 the spec names a construction ("built on Go timers"), not an
engine; `x/time/rate` was rejected in the ADR (uninjectable clock vs the §6
deterministic-clock mandate, another floor-pin dance). Float tokens brought current on
demand (no goroutines, no standing timers — the ADR-0010 lazy lineage); **Wait reserves
its token on arrival** (debt-based queue: arrival-order fairness, per-waiter exact
sleeps, no herding) and a canceled Wait repays its reservation. rapid property pins the
token-bucket law (`admitted ≤ burst + rate·elapsed`). First benchmark report lands under
`docs/benchmarks/` (spec §6 gap closed): ~25ns zero-alloc `Allow`, ~50ns zero-alloc
funded `Wait`; the report documents the burst-1/same-tick timer-path boundary honestly.
Patterns row renamed from the intake's "Token Bucket" to the in-taxonomy **Rate Limiting
/ Throttling** (catalogue rule; mechanism noted in the row). **M3 complete → README
milestone table flips M3 done; both v0.2.0 (M2) and now a v0.3.0-eligible M3 are uncut —
the maintainer decides whether to cut one release or two.**

## Addendum — roadmap 3.2 retry.Backoff (same day)

PR #14 merged (`f448bd7`) — master green for the first time since the 2.6 merge. Item
3.2 followed on `feat/retry` (draft PR #15, ADR-0011, patterns row 7): the spec-frozen
`Backoff(ctx, Policy, fn func(ctx) error)` with **symmetric proportional jitter**
(`Jitter ∈ [0,1]` is a ±half-range fraction; 0 = the deterministic schedule — the spec's
field demands a tunable knob, so AWS full/equal jitter were rejected in the ADR), a
**`MaxDelay` hard cap that survives jitter** (the §6 bound invariant: every sleep in
`[(1−J)·exp, min((1+J)·exp, MaxDelay)]`, pinned by a rapid property over generated
policies), overflow-safe doubling, loud policy/nil-fn validation, and **exhaustion
returning the last error verbatim** (no wrapper, no sentinel — errors.Is/As stay aimed
at the real cause). Deterministic testing via unexported `sleep`/`rand` seams on
`Policy` (in-package tests only; no globals, no exported surface growth). Jitter uses
`math/rand/v2` with a targeted, reasoned G404 `//nolint` (first production `nolint` in
the repo — jitter is not security-sensitive). Full local gauntlet green before push
(build, vet, all tests, gofumpt, coverage).
