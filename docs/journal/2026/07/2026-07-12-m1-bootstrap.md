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

## Addendum — v0.1.0 release prepared (same session)

PR #6 merged with all 8 CI checks green — the first fully-green matrix run. The v0.1.0
release PR was then prepared per `docs/workflow/release.md`: version constant bumped,
`[Unreleased]` rolled into `docs/changelog/v0/v0.1.0.md`, README badge refreshed, release
notes drafted under `docs/releases/v0.1.0.md`. After the maintainer merges: the agent tags
`v0.1.0` (carry-through), CI drafts the GitHub Release from the tag push, the maintainer
publishes. Then Milestone 2 opens with item 2.1 (workerpool.Pool).
