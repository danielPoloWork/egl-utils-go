# ADR-0043: Gate spec section 5 against the real exported surface

- **Status:** Accepted
- **Date:** 2026-07-27
- **Deciders:** Maintainer (Daniel Polo), with the architect agent
- **Related:** ADR-0035 (import-graph enforcement), ADR-0036 (coverage gate), ADR-0040, ADR-0042, spec §5, ROADMAP 12.3

## Context

Milestone 11's read-through compared the frozen v1 specification against the as-built project and
found **nine divergences in a 164-line document, none of which had ever failed a build**. Four were
in §3/§6 and one, §5's public-interface enumeration, had fallen twelve identifiers behind what
v1.1.0 shipped — every one of them added by a PR that passed the full quality bar.

The gap is structural rather than careless. `consistency_lint.py`'s spec-map check verifies that
every spec section has a fulfilling roadmap item; it has no view of whether the section's *claims*
are true. Nothing else looks at the spec at all. So the one document the project calls a frozen
contract was the only artefact in the repository with no mechanical check on its content.

§5 is the section where that matters most, for a reason ADR-0042 and 12.1 made explicit: its closing
clause binds SemVer to the module's exported surface. A drifting enumeration inside a contract is
not a documentation defect — before 12.1 widened the clause, an identifier missing from the list was
an identifier outside the stability promise. The list is load-bearing, so it needs a lock.

## Decision

`tools/spec_api_lint.py` compares §5's enumeration against the module's real exported surface and
fails the build when they disagree, in **both** directions, because the two failures are different
bugs:

- **shipped but unlisted** — an exported identifier missing from §5. This is the failure that
  actually happened: Milestone 10 added public API across nine PRs and none updated the spec.
- **listed but gone** — §5 names something the module no longer exports. A stale promise is worse
  than a missing one, because a consumer can write code against it.

The authority for "what is exported" is `go doc -all`, the same rendering pkg.go.dev serves.
Const and var **blocks are walked member by member**, and exported struct fields and interface
methods count as surface. `contrib/*` is out of scope per ADR-0040 and the tool asserts its absence
from the root module rather than assuming it. The tool joins `consistency_lint.py`,
`import_graph_lint.py` and `coverage_gate.py` as the fourth policy checker, in the CI job that
already provisions Go.

## Alternatives Considered

- **Parse the Go sources directly in Python.** Rejected: it reimplements the compiler's notion of
  "exported" against Go's real grammar — build tags, embedded types, generic constraints, grouped
  declarations — and every gap becomes a false negative in the exact tool whose job is to have none.
  `go doc` already answers the question authoritatively, and shelling out to the toolchain is the
  house pattern (`import_graph_lint.py` uses `go list` and `go mod graph`; `coverage_gate.py` uses
  `go test`).
- **Scan only column-zero declarations**, as the throwaway checker written during 12.1 did.
  Rejected once measured: that script reported 110 identifiers and this tool reports **130**. The
  twenty it could not see were const/var block members, struct fields and interface methods —
  including `workerpool.ErrPoolClosed`, a genuine tenth divergence the read-through itself missed.
  A checker with a blind spot is worse than none, because it certifies the part it cannot see.
- **Forward direction only** (fail on unlisted, ignore stale). Rejected: cheaper and it would have
  caught M10's twelve, but it lets §5 keep promising a removed identifier indefinitely. Under
  ADR-0042 a removal is a MAJOR-only event, so a stale §5 entry is precisely the shape of claim the
  versioning contract must not make casually.
- **Require §5 to be machine-generated from `go doc`.** Rejected: §5's value is that it is written
  for a reader — grouped by concern, with the error model and the versioning clause in prose. A
  generated list would be complete and unread. Checking a hand-written section is the point.
- **A `go:generate` step or a test in the Go suite.** Rejected: the spec is a governance artefact,
  and the three existing policy checks are Python tools run together before every PR. A fourth in
  the same place is one habit; a fourth somewhere else is a second habit nobody keeps.

## Consequences

- **Adding exported API now requires updating §5 in the same PR**, or CI is red. That is the
  intended cost and the direct fix for how M10's additions accumulated. It also means an
  intentional API addition carries a visible diff in the contract, which is the paper trail
  ADR-0042 assumes exists.
- **Verified by deliberate violation**, per the ADR-0035/0036 precedent, in all three shapes: an
  identifier removed from §5 (caught as `ratelimit.ErrLimited` unlisted), a fabricated §5 entry
  (caught as `fanout.SplitBuffered` and `fanout.ErrNoSinks` gone), and a new exported function added
  to a package with the spec untouched — the real-world drift scenario — caught as `fanout.Drain`
  unlisted. Baseline: 130 identifiers across 22 packages, clean.
- **It found a divergence on its first run.** `workerpool.ErrPoolClosed` was exported, absent from
  §5, and invisible to the manual read-through because it is the second member of a `var (…)` block
  whose first member, `ErrQueueFull`, *was* listed. Fixed in the same PR.
- **The reverse direction recognises functions, methods and sentinel errors** — the shapes §5 writes
  unambiguously — not free prose. A deleted type is normally caught anyway, since its constructor
  and methods go with it, and a renamed one is caught by the forward direction. Stated in the
  tool's docstring rather than left for a reader to discover.
- **Only §5 is gated.** §1–§4 and §6 are prose about intent and strategy, with no machine-readable
  counterpart; the M11 read-through found real divergences there too (§4's isolation claim, §6's
  test-strategy counts) and no tool would have caught them. Periodic human read-through remains the
  control for those sections — this ADR narrows the surface that needs it, it does not remove it.
- Runtime is one `go list` plus one `go doc` per package, a few seconds, in a job that already sets
  up Go. No new dependency: Python standard library only, like the other three.

## References

- ADR-0035, ADR-0036 — the two prior policy tools and the verify-by-deliberate-violation precedent.
- ADR-0042 — the post-1.0 compatibility contract that makes §5 load-bearing.
- ADR-0040 — `contrib/*` versions independently and is outside the surface.
- Spec §5 and its Amendments block; ROADMAP 12.1–12.3.
- `docs/journal/2026/07/2026-07-27-m12-public-interface.md` — the read-through that produced this.
