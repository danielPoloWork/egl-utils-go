# ADR-0036: statement-coverage floor — 85%, enforced per package

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** AGENTS.md §10 (the quality bar, whose coverage row promised "finalized in an ADR" — this is that ADR), ADR-0030 (spec v2 reconciliation: 10.9 adopted), ADR-0035 (the sibling policy gate, same enforcement pattern), spec v2.0 §7 ("coverage gate ≥ 85%"), roadmap 10.9

## Context

AGENTS.md §10 has carried the row *"Coverage | new code ≥ 80% line (finalized in an ADR)"* since the
project's bootstrap — a provisional number with an explicit promise to settle it properly, and no CI
enforcement behind it. Spec v2 §7 raises the bar to "coverage gate ≥ 85%", and ADR-0030 put that in the
additive bucket as roadmap 10.9. So this decision both adopts the new number and discharges the
outstanding promise.

The measured starting point matters, because it determines whether a gate is real or decorative:

| Package | Coverage |
|---|---|
| `fanout` | 93.3% |
| `fanin` | 95.7% |
| `pubsub` | 96.4% |
| `retry` | 97.7% |
| `ratelimit` | 98.1% |
| the other 16 packages | 100.0% |

## Decision

The floor is **85% of statements, enforced per package**, by `tools/coverage_gate.py` in a CI job;
packages with no statements are skipped rather than counted as zero. AGENTS.md §10's coverage row is
updated to 85% and now points here.

## Alternatives Considered

- **A module-wide average.** The usual shape of a coverage gate, and the simplest to compute. Rejected
  because with 16 of 21 packages at 100% the module-wide figure sits around 99%: a package could rot to
  50% and the gate would still pass, so it could not fail for any realistic regression. It also lets a
  well-covered package subsidise a neglected one, which inverts what the gate is for. Per package, the
  threshold binds the *weakest* package — the only place a coverage floor does useful work.
- **Setting the floor at the current low-water mark (93%) instead of 85%.** Tempting: it locks in what
  the project actually achieves rather than a number 8 points below it. Rejected because §7 specifies 85
  and a floor should be the level below which review is *required*, not a ratchet that fails on ordinary
  fluctuation — a legitimately hard-to-reach defensive branch in a small package can move the figure
  several points, and a gate that fires on noise gets disabled. The real standard here is the house habit
  of 100% where it is reachable, which the per-package report makes visible on every run; 85% is the
  floor, not the target.
- **Enforcing "new code ≥ 85%" on the diff**, which is what the AGENTS row literally said. Rejected as
  disproportionate machinery for this repo: diff coverage needs a base-branch profile and a mapping from
  changed lines to profile blocks, and it silently passes a PR that deletes tests. Whole-package coverage
  catches both the untested addition and the removed test.
- **`-covermode=atomic` with a merged profile.** More precise under `-race`, and needed if coverage were
  ever computed across packages. Rejected as unnecessary: the gate reads the per-package figure
  `go test -cover` already prints, and the race detector has its own job.
- **Excluding packages or lines from measurement** (build tags, `//coverage:ignore`-style pragmas).
  Rejected: nothing currently needs it, and an exclusion mechanism is the standard way a coverage gate
  becomes meaningless. If a genuinely unreachable branch ever pushes a package under the floor, the honest
  move is to say so in the PR and change the number deliberately — which is what the failure message
  tells the author to do.

## Consequences

- CI gains a `coverage` job (parallel, ~1 minute) that fails any package below 85%. The gate is verified
  by deliberate violation, not assumed: raising the threshold to 96% makes `fanout` and `fanin` fail with
  the expected message, and the clean tree passes at 85%.
- Every run prints the per-package table when it fails, and `--report` prints it on demand, so the
  weakest package is always visible rather than buried in an average. That is the part most likely to
  change behaviour day to day.
- **The 8-point margin between the floor and the current low-water mark is deliberate slack**, not an
  invitation: the five sub-100% packages are all sub-100% for the same reason — a defensive branch that
  no test can reach through the package's public surface — and none of them is close to 85%.
- AGENTS.md §10 no longer carries an unfulfilled "finalized in an ADR" note. While editing that table, a
  stale claim was corrected alongside it: the module floor is Go **1.25** (`go.mod` says `go 1.25.0` and
  the CI matrix runs 1.25/1.26), not 1.24 as the row said.
- Same enforcement pattern as ADR-0035: a small Python tool under `tools/`, runnable locally, invoked by
  a named CI job — so a policy failure reads as its own job rather than as a mysterious test failure.
- Deferred: diff-scoped coverage if the module ever grows a package where whole-package measurement is
  too coarse, and raising the floor once the weakest package is deliberately improved.

## References

- Spec v2.0 §7 (coverage gate ≥ 85%), AGENTS.md §10 (quality bar), ADR-0030 (adoption bucket).
- `tools/coverage_gate.py`, the `coverage` job in `.github/workflows/ci.yml`.
- ADR-0035 (`tools/import_graph_lint.py`) — the pattern this follows.
