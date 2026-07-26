# 2026-07-26 — Milestone 10.8: import-graph enforcement

## What got done

- **Roadmap 10.8** (branch `feat/import-graph-enforcement`): ADR-0004's dependency rings and spec v2
  §3's layered import graph are now build-breaking rules instead of prose. **New
  [ADR-0035](../../../adr/0035-import-graph-enforcement.md)**.
- **Enforced twice, in two mechanisms with different reach** — and the second one is not belt-and-braces
  padding, it closes real blind spots:
  - **`depguard` in `.golangci.yml`**: each governed module confined to the package whose ADR bought it
    (`yaml.v3` → `config`, `client_golang` → `metrics`, `x/crypto` → `hash`, `x/sync` → `semaphore`);
    database-driver and cache-client SDKs denied by name (spec §3 — driver-backed probes belong in
    10.13's `contrib/*`); internal sibling imports denied everywhere except `config`, which gets a
    `list-mode: strict` allowlist of stdlib + `yaml.v3` + `validator`.
  - **`tools/import_graph_lint.py`** over the *resolved* graph: direct `go.mod` requirements against the
    ring allowlist, each package's direct imports against its budget, the internal edge set against
    exactly `{config → validator}`, and `go mod graph`'s edges out of our module against what `go.mod`
    declares.
- **The finding that justified the second mechanism: depguard does not report a blank import of a
  sibling package.** `import _ "…/egl-utils-go/cache"` from `retry` passes depguard cleanly — while the
  *same* blank import of `gopkg.in/yaml.v3` is caught. So `_` would have been a bypass for the
  architecture rule specifically. revive's `blank-imports` does object, but that is a style rule a
  justifying comment silences, so it is not an architectural guarantee. The graph tool reads the real
  import list and catches it. depguard is also blind to a new **direct module requirement** (a policy
  decision that is not yet an import) and cannot notice that a sanctioned exception has gone **dead**.
- **Every rule verified by deliberate violation, not assumed.** A plain sibling import, a blank sibling
  import, a governed module imported by the wrong package, a non-`validator` sibling imported by
  `config`, and a test-only dependency in production code each produce the expected failure; the clean
  tree passes both mechanisms. This mattered: the first draft of the internal-edge rule *looked* fine
  and I nearly shipped it on a run where only the yaml and crypto rules had actually fired — a rule that
  never fires is worse than no rule.
- **The rule expressed for internal edges is "a same-layer edge only where the spec mandates the
  composition"**, deliberately not "L2 may import L2". 10.6's `config → validator` (ADR-0033) is legal
  because spec item 13 defines config as "configuration with struct validation (via item 19)", and it is
  allowlisted as an exception in all three places (ADR, depguard rule, `ALLOWED_INTERNAL_EDGES`). The
  tool also fails if that edge ever **disappears**, so a dead exception cannot quietly outlive the
  composition that justified it.
- **Rejected pinning the whole production import closure** (`go list -deps`), which would catch a
  transitive dependency reaching production code. It includes `client_golang`'s own dependency tree, so
  every upstream bump would fail CI for a decision nobody made — and Dependabot has already broken this
  repo once that way (#44). The tool asserts *direct* imports, which are the decisions under review;
  `govulncheck` and the committed `go.sum` cover the transitive supply chain.
- Also rejected: a generic "derive each package's layer and enforce downward-only" model. With exactly
  one internal edge in the module it would be inference from a single data point, and it would silently
  permit every future L3 → L2 edge without anyone deciding.
- CI gains an `imports` job (~1 minute, parallel, Go + Python); depguard rides along in the existing
  `quality` job at no extra cost. Two small implementation notes now handled: `go mod graph` emits `go`
  and `toolchain` pseudo-module edges that are directives rather than requirements (skipped), and the
  tool's output is ASCII because the Windows console codepage mangles `→`/`§`/`—`.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...`, gofumpt clean,
  golangci-lint v2 (now including depguard) 0 issues, govulncheck 0 affecting, `consistency_lint.py` OK,
  `import_graph_lint.py` OK. No dependency change — this item adds enforcement, not code.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (8 of 13)**: 10.1 (#37), 10.2 (#38), 10.3 (#46), 10.4 (#47),
10.5 (#48), 10.6 (#49), and 10.7 (#50) merged; 10.8 drafted on `feat/import-graph-enforcement`, awaiting
the maintainer to open and merge. M10 releases as v1.1.0.

## How the next session resumes

Wait for the 10.8 PR to merge. Then **10.9 the CI coverage gate at ≥ 85%** (spec v2 §7, roadmap tags it
*low*): raise the §10 floor and enforce it in CI. Two things to settle first, because the naive version
either passes vacuously or blocks the milestone:

- **Measure per package or across the module?** Every package currently sits at or near 100%, so a
  module-wide 85% would pass without constraining anything; a per-package floor is the gate with teeth.
  Worth printing the current per-package numbers in the PR so the chosen threshold is evidence-based.
- **`ratelimit` is the known exception at 98.1%** (the residual is a defensive branch in `wait`'s
  production path that the fake-clock seam cannot reach) — still far above 85%, so no exclusion is
  needed; just do not let the gate be written in a way that demands 100%.

Standard footprint per PR (CHANGELOG `[Unreleased]`, ROADMAP checkbox, journal, lint). Portable Go under
`%TEMP%\go-portable` — in the Bash tool add it as the *unix* path
`/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs the `/v2` module path;
`-race` is CI-only, and `-fuzz` needs the restored `pkg/include` + `src/runtime/cgo` headers.
