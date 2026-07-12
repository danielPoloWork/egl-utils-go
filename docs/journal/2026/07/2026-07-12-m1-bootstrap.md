# 2026-07-12 — Milestone 1 bootstrap

## What got done

- **Go module bootstrap** (roadmap 1.1–1.5): `go.mod` at the repository root
  (`module github.com/danielPoloWork/egl-utils-go`, language floor 1.24), root package
  `utils` (`doc.go`, `version.go` with `Version = "0.0.0"`), external-package SemVer smoke
  test (`version_test.go`), and the golangci-lint v2 config (`.golangci.yml`: defaults +
  revive + gosec). CI matrix was already live from the scaffold PR (#1) and the owner's
  Dependabot merges (#3–#5, notably golangci-lint-action v6→v9).
- **ADR-0003** — idiomatic Go root layout; supersedes ADR-0002 for this repo. Decision
  driver: Go binds import paths to directory paths, so the tree and the promised short
  import (`…/egl-utils-go/workerpool`) could not both hold. The `src/` tree is removed.
- **ADR-0004** — runtime dependency policy (stdlib + `golang.org/x` + exactly two vetted
  runtime deps; test-only testify/goleak/rapid). Registered as compliance control C-1.
- **Docs sync** for the layout change: AGENTS.md §4/§5/§10, CLAUDE.md, GEMINI.md, README,
  ROADMAP 1.1/1.2 text, patterns and benchmarks READMEs, spec §4, lint CONFIG.
- **Per-milestone agent guidance** added to ROADMAP.md (model × effort per milestone), at
  the maintainer's request.

## Where the project stands

Milestone 1 complete (pending this PR's merge); CI expected fully green for the first time
— build/test/race/bench jobs now have a module to build. Version stays `0.0.0`; the v0.1.0
release (M1 = a completed milestone → MINOR bump per AGENTS.md §11) is a separate release
PR the maintainer triggers.

## How the next session resumes

Wait for this PR to merge (one PR at a time), then either cut the v0.1.0 release PR per
`docs/workflow/release.md`, or start roadmap item **2.1 workerpool.Pool** on a fresh branch
from `master` (recommended tier per ROADMAP guidance: strongest model, max effort;
leak/race/bench coverage required).

## Addendum 7 — roadmap 2.6 test-only dependency adoption (same session)

PR #12 (semaphore) merged. Item 2.6 on `feat/test-deps-goleak-rapid`: adopted the three
ADR-0004 test-only deps and migrated the suites. `internal/leakcheck` (the interim runtime
goroutine-count guard) is deleted; every `leakcheck.Guard(t)` becomes `defer
goleak.VerifyNone(t)` across workerpool, pubsub, fanin, fanout, semaphore. The three seeded
`math/rand` randomized properties (fanin merge completeness, fanout split completeness,
pubsub delivery) are rewritten as `pgregory.net/rapid` properties — rapid draws the
topology and shrinks a counterexample to a minimal failing case; `testify/require` carries
the assertions (kept on the main goroutine only, since require's FailNow uses Goexit and is
not goroutine-safe — the fanout consumers record into per-output slices and assert after
`Wait`). Versions pinned by `go` directive vs the 1.24 floor: goleak v1.3.0 (go 1.20), rapid
v1.3.0 (go 1.23), testify v1.11.1 (go 1.17) — all below the floor, no readonly-precision
trap (contrast the 2.5 x/sync saga). Handoff, per the maintainer's call: this machine has
no Go toolchain and the transitive graph (testify → go-spew/go-difflib/yaml.v3/objx; goleak
→ kr/pretty/check.v1) is what only `go mod tidy` resolves reliably, so the branch ships the
code + a best-effort `go.mod` (three direct requires) and the maintainer runs `go mod tidy
&& go test ./...` to write `go.sum` + the indirect set. CI stays red until that lands (as
with 2.5's first push). The `.golangci.yml` G404 exemption is now inert (no test uses
math/rand) but left in place — harmless and cheap insurance for future seeded tests.

## Addendum 6 — roadmap 2.5 semaphore.Weighted (same session)

PR #11 (fanout) merged. Item 2.5 on `feat/semaphore`: `semaphore.Weighted`, a thin adapter
over `golang.org/x/sync/semaphore` (ADR-0009) — the spec's three-method surface
(`NewWeighted`/`Acquire`/`Release`) with the house loud-panic contract layered on
(non-positive capacity/weight panic); `x/sync`'s `TryAcquire` deliberately not re-exported
(not in spec §5). Catalogued as **Guarded Suspension** (patterns row 5, in-taxonomy this
time). This is the module's **first runtime dependency**, so it births `go.sum`. Two
wrinkles resolved this session: (1) no local Go toolchain → the maintainer initially took
the PR red and was to run `go mod tidy`, then redirected the agent to fetch canonical
checksums from `sum.golang.org` and commit `go.sum` directly (a wrong hash fails CI's
verification loudly, so this can't smuggle a bad artifact past the gate). (2) dependency go-directive
vs our **1.24 floor**. First tried `@latest` v0.22.0 (`go 1.25.0`) — too high. Then v0.19.0
(`go 1.24.0`) — CI still red: `go build -mod=readonly` rejects a `go 1.24.0` dep directive
when the main module says short-form `go 1.24`, demanding the patch-precise `go 1.24.0`
("go: updates to go.mod needed"). Rather than bump our directive (and every doc that says
"go 1.24"), pinned **v0.16.0** — newest x/sync on `go 1.23.0`, which `go 1.24` satisfies
unambiguously, floor string untouched. Lesson (doubly earned): hand-pinning a dep skips the
`go mod tidy` floor-bump that surfaces this — check the dep's go directive against the floor
BEFORE pinning, and note that a dep directive equal-in-minor but patch-precise (1.24.0) is
NOT satisfied by a short-form main directive (1.24) under readonly build.

## Addendum 5 — roadmap 2.4 fanout.Split (same session)

PR #10 (fanin) merged. Item 2.4 followed on `feat/fanout`: the spec's verb *distribute*
admits broadcast or work-distribution semantics — a one-way API choice — resolved as
**exactly-once distribution** on three convergent grounds (Go-pipelines vocabulary already
adopted in ADR-0007, spec §6's "no message lost or duplicated", and broadcast duplicating
pubsub.Broker's concern), with broadcast recorded as Rejected in ADR-0008. Shape: the
exact dual of fanin — one forwarder per output competing on the shared input, each closing
the output it owns (`len(outs)` goroutines, no closer). Completeness + per-output-order
tests including a seeded randomized property and a blocked-send cancellation proof.
Patterns row 4 (Fan-In / Fan-Out) is now complete: both halves implemented.

## Addendum 4 — roadmap 2.3 fanin.Merge (same session)

PR #9 (pubsub) merged after one real CI failure — gosec G404 on the seeded property test,
resolved with a G404+`_test.go`-scoped `.golangci.yml` exclusion (seeded reproducibility is
the point) — and after the maintainer's per-item agent-guidance tags landed verbatim in the
same PR. Item 2.3 followed on `feat/fanin`: the canonical forwarder-per-input fan-in with a
WaitGroup closer (ADR-0007), completeness + per-input-order tests including a seeded
randomized property and a blocked-send cancellation proof. Patterns row 4 (Fan-In /
Fan-Out, outside-taxonomy name recorded in the ADR) flipped to Implemented — the fanout
half completes it at 2.4.

## Addendum 3 — roadmap 2.2 pubsub.Broker (same session)

PR #8 (workerpool) merged, all checks green on first CI contact. Item 2.2 followed:
generic `pubsub.Broker[T]` with at-most-once per-subscription buffered delivery (the only
policy coherent with the ctx-less, error-less `Publish` contract — ADR-0006), observable
drops, zero broker goroutines, and an additive `Close` recorded in spec §5. The interim
leak guard moved to `internal/leakcheck`, shared by workerpool and pubsub until 2.6 lands
goleak. Patterns catalogue row 3 (Publish-Subscribe) flipped to Implemented.

## Addendum 2 — v0.1.0 tagged; roadmap 2.1 implemented (same session)

PR #7 merged; the agent pushed the annotated `v0.1.0` tag and the release workflow drafted
the GitHub Release (publishing remains the maintainer's click). Milestone 2 then opened
with item 2.1: the `workerpool` package (Thread Pool + Functional Options, ADR-0005),
black-box tests with the interim in-repo leak guard (ROADMAP 2.6 tracks the goleak
migration — no local Go toolchain to produce `go.sum` for test-only deps), and a
`BenchmarkSubmit` baseline. Patterns catalogue rows 1–2 flipped to Implemented.

## Addendum — v0.1.0 release prepared (same session)

PR #6 merged with all 8 CI checks green — the first fully-green matrix run. The v0.1.0
release PR was then prepared per `docs/workflow/release.md`: version constant bumped,
`[Unreleased]` rolled into `docs/changelog/v0/v0.1.0.md`, README badge refreshed, release
notes drafted under `docs/releases/v0.1.0.md`. After the maintainer merges: the agent tags
`v0.1.0` (carry-through), CI drafts the GitHub Release from the tag push, the maintainer
publishes. Then Milestone 2 opens with item 2.1 (workerpool.Pool).
