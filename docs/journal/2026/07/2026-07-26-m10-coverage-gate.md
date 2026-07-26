# 2026-07-26 — Milestone 10.9: per-package coverage gate

## What got done

- **Roadmap 10.9** (branch `feat/coverage-gate`): spec v2 §7's "coverage gate ≥ 85%" is now enforced.
  **New [ADR-0036](../../../adr/0036-coverage-gate.md)**, which also discharges AGENTS.md §10's
  long-standing promise that the coverage number would be "finalized in an ADR" — it never had been.
- **Enforced per package, not as a module-wide average — this is the whole decision.** Measured first:
  16 of 21 packages are at 100%, so the module-wide figure sits around 99%. A module-wide 85% gate could
  therefore never fail; a package could rot to 50% and CI would stay green. Per package, the floor binds
  the **weakest** package, which is the only place a coverage floor does useful work.
- **Measured the ground truth rather than trusting the note I left last session.** I had recorded
  `ratelimit` (98.1%) as the low-water mark; it is not. The real order is **fanout 93.3%**, fanin 95.7%,
  pubsub 96.4%, retry 97.7%, ratelimit 98.1%, everything else 100%. Worth having checked — the threshold
  decision rests on that number.
- **Kept the floor at 85% rather than ratcheting it to the observed 93%.** §7 specifies 85, and a floor
  should mark the level below which review is required, not track current achievement: a single
  hard-to-reach defensive branch in a small package moves the figure by several points, and a gate that
  fires on ordinary fluctuation gets disabled. The house habit of 100% where it is reachable is the
  actual standard, and the per-package table keeps it visible on every run; 85% is the floor, not the
  target. The 8-point margin is deliberate slack.
- Also rejected: **diff-scoped coverage** (what the old AGENTS row literally said) — it needs a
  base-branch profile and a changed-line mapping, and it silently passes a PR that *deletes* tests, which
  whole-package measurement catches; **`-covermode=atomic` with a merged profile** — unnecessary when the
  gate reads the figure `go test -cover` already prints; and **any exclusion mechanism** (build tags,
  ignore pragmas) — nothing needs one, and exclusions are the standard way a coverage gate becomes
  meaningless. If an unreachable branch ever pushes a package under the floor, the honest move is to say
  so in the PR and change the number deliberately, which is what the failure message instructs.
- `tools/coverage_gate.py` follows ADR-0035's pattern: a small Python tool under `tools/`, runnable
  locally, invoked by a named CI job so a policy failure reads as its own job. Packages with no
  statements (the root, which carries only the module doc and version) are **skipped, not counted as
  zero**. `--report` prints the table on demand.
- **Verified by deliberate violation**, as with 10.8: raising the threshold to 96% makes `fanout` and
  `fanin` fail with the expected per-package message and exit 1; at 85% the tree passes. A gate that
  cannot fail is not a gate.
- Two documentation corrections in AGENTS.md §10 while editing that table: the coverage row (80% →
  85% per package, pointing at ADR-0036), and a stale claim next to it — the build-matrix row said
  "module floor 1.24" while `go.mod` declares `go 1.25.0` and the CI matrix runs 1.25/1.26. Corrected to
  1.25 rather than left standing in the contract table.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...`, gofumpt clean,
  golangci-lint v2 0 issues, govulncheck 0 affecting, `consistency_lint.py` OK, `import_graph_lint.py`
  OK, `coverage_gate.py` OK. No dependency change and no library code touched — this item adds a gate.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (9 of 13)**: 10.1 (#37), 10.2 (#38), 10.3 (#46), 10.4 (#47),
10.5 (#48), 10.6 (#49), 10.7 (#50), and 10.8 (#51) merged; 10.9 drafted on `feat/coverage-gate`, awaiting
the maintainer to open and merge. M10 releases as v1.1.0.

CI now runs eight jobs: `build` (4-cell matrix), `consistency`, `quality` (gofumpt + golangci-lint incl.
depguard + race + govulncheck), `benchmark`, `coverage`, `imports`, `fuzz`, and `race`.

## How the next session resumes

Wait for the 10.9 PR to merge. Then **10.10 the NFR benchmark suite** — the milestone's reasoning-heavy
item (roadmap tags it *Fable 5 · high*): benches for NFR-01/02/03/04/06, a `benchstat` methodology, and a
nightly regression workflow that flags >10% drift. Points to settle before writing code:

- **NFR-01 (middleware chain ≤ 1 µs and 0 allocs/op on the non-logging path; Logger ≤ 3 allocs/op)** is
  the one with a hard allocation target, and `ratelimit`'s middleware already has a 0-alloc assertion via
  `testing.AllocsPerRun` (10.4) — reuse that pattern rather than trusting a benchmark's `-benchmem`.
- **Gating on a developer workstation's numbers is the trap.** `docs/benchmarks/` already records that
  local runs are "informational, not a gating baseline", and the existing `benchmark` CI job is unpinned
  and unpoliced. A >10% regression gate needs a recorded baseline on comparable hardware; GitHub runners
  are noisy, so decide explicitly whether the nightly job *fails* or merely *reports*, and say which in
  the ADR. Spec §5 names a reference machine (Ryzen 7 5800X) we do not have.
- `docs/benchmarks/` has a template and index to follow, and two reports already exist (ratelimit hot
  paths, bcrypt cost sizing) — NFR reports should match that shape.
- Note the `benchmark` job's `go-version` expression references `matrix.toolchain` with no matrix
  defined; it resolves to 1.26 by accident. Worth tidying while touching that job.

Standard footprint per PR (CHANGELOG `[Unreleased]`, ROADMAP checkbox, journal, lint). Portable Go under
`%TEMP%\go-portable` — in the Bash tool add it as the *unix* path
`/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs the `/v2` module path;
`-race` is CI-only, and `-fuzz` needs the restored `pkg/include` + `src/runtime/cgo` headers.
