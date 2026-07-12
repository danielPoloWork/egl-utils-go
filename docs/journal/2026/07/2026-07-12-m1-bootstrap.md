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
