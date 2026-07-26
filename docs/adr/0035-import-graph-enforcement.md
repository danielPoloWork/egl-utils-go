# ADR-0035: import-graph enforcement — depguard per file, a resolved-graph assertion beside it

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0004 (the dependency rings this enforces), ADR-0033 (the one sanctioned internal edge), ADR-0003 (contrib/* submodules keep drivers out of the core), ADR-0009/0018/0024/0027 (the ADRs that justified each governed module), ADR-0030 (spec v2 reconciliation: 10.8 adopted), spec v2.0 §3 and §7, roadmap 10.8

## Context

ADR-0004 limits runtime dependencies to three rings and names exactly which module belongs where; spec
v2 §3 draws a layered import graph (L1 foundation, L2 services, L3 HTTP) with "arrows point downward
only" and "no package imports database drivers, redis clients, or prometheus SDK", and §7 asks for
`depguard` "enforcing the §3 import rules" plus a `go mod graph` CI assertion. Until now both were
prose: correct, reviewed, and unenforced. Two facts make enforcement timely.

**Roadmap 10.6 opened the first internal edge.** `config → validator` (ADR-0033) is legal because spec
item 13 defines config as "configuration with struct validation (via item 19)", but it is a *same-layer*
edge, and §3's downward-only rule does not describe it. Whatever is written down now becomes the rule
future edges are judged against, so the exception has to be encoded as an exception rather than as
permission for L2 generally.

**The core module's real graph is narrow and worth keeping so.** Four non-stdlib runtime modules, each
confined to the one package whose ADR bought it; every other feature package is stdlib-only, which is
what makes them independently adoptable. That property is invisible in review and easy to lose to a
single convenient import.

## Decision

Enforce both policies twice, in two mechanisms with different reach: **`depguard` rules in
`.golangci.yml`** confine each governed module to its owning package, forbid driver and cache-client
SDKs outright, and deny internal sibling imports everywhere except `config`, which gets a `strict`
allowlist of stdlib + `yaml.v3` + `validator`; and **`tools/import_graph_lint.py`** asserts the same
policies over the resolved graph — direct `go.mod` requirements against the ring allowlist, each
package's direct imports against the per-package budget, the internal edge set against exactly
`{config → validator}`, and `go mod graph`'s edges out of our module against what `go.mod` declares.

The rule expressed for internal edges is deliberately **"a same-layer edge only where the spec mandates
the composition"**, not "L2 may import L2".

## Alternatives Considered

- **depguard alone.** The obvious reading of §7, and it is where a violation should surface first
  (in-editor, on the offending line). Rejected as sufficient because it has three blind spots, one of
  which was found while writing this: **depguard does not report a blank import of a sibling package.**
  `import _ "…/egl-utils-go/cache"` from `retry` passes depguard — verified — while the same blank
  import of `gopkg.in/yaml.v3` is reported. revive's `blank-imports` also objects, but that is a style
  rule that a justifying comment silences, so on its own it is not an architectural guarantee. depguard
  also cannot see a **new direct module requirement** (a policy decision that is not yet an import), and
  cannot notice that a sanctioned exception has become **dead**.
- **The graph assertion alone.** Simpler, and it has no blind spots of its own. Rejected because it
  reports at the wrong time and place: a CI job failure naming a package, rather than a red squiggle on
  the import line with the ADR that forbids it. Defence in depth is cheap here, and the two mechanisms
  fail in different directions.
- **Deriving the layer of each package and enforcing "arrows point downward"** generically, rather than
  allowlisting the one edge. Rejected as more machinery than the situation earns: the module has *one*
  internal edge, so a layer model would be inference built on a single data point, and the allowlist
  states the truth — this composition is mandated, no other is — while a layer model would silently
  permit every future L3 → L2 and L2 → L1 edge without anyone deciding.
- **Pinning the whole production import closure** (`go list -deps`), which would catch a transitive
  dependency reaching production code. Rejected as brittle: the closure includes `client_golang`'s own
  dependencies, so every upstream bump would fail CI for a decision nobody made — and Dependabot bumps
  have already broken this repo once (#44). The tool asserts *direct* imports, which are the decisions
  under review, and `govulncheck` plus the committed `go.sum` cover the transitive supply chain.
- **Failing on any new `// indirect` requirement.** Same brittleness, no policy content: indirect
  requirements are consequences of the direct ones ADR-0004 governs.
- **A test in Go (`TestImportGraph`) instead of a Python tool.** Would run in the existing suite with no
  new CI job. Rejected for consistency with `tools/consistency_lint.py`, the established home for
  cross-artifact congruence checks that are policy rather than behaviour, and because a policy failure
  reads better as its own named CI job than as a unit-test failure.

## Consequences

- Two files now encode ADR-0004 and spec §3 as build-breaking rules, and **both must be edited together**
  to add a dependency or an internal edge. That is the intended friction: the edit lands in the same PR
  as the ADR that sanctions it, and the tool's error message says so.
- **The `config → validator` exception is now explicit in three places** — ADR-0033, the depguard rule,
  and `ALLOWED_INTERNAL_EDGES` — and the tool fails if the edge ever *disappears*, so a dead exception
  cannot quietly outlive the composition that justified it.
- Verified by deliberate violation rather than assumed: a sibling import (plain and blank), a governed
  module imported by the wrong package, and a test-only dependency in production code each produce the
  expected failure, and the clean tree passes both mechanisms. A rule that never fires is worse than no
  rule, so this check is part of the item, not an afterthought.
- CI gains an `imports` job (~1 minute, parallel). depguard runs inside the existing `quality` job at no
  extra cost.
- Rules apply to production files only (`!$test`). Tests legitimately import sibling packages and the
  test-only ring; the tool covers the part that still matters there — a test-only dependency must never
  appear in a production import.
- Driver and cache-client SDKs are denied by name (`lib/pq`, `jackc/pgx`, `go-sql-driver/mysql`,
  `redis/go-redis`, `go-redis/redis`) rather than by pattern, so the list needs extending when a new
  ecosystem appears. An allowlist over all third-party imports would be tighter, but the graph tool
  already provides exactly that at module level, which is the layer where the decision is made.
- **10.13's `contrib/*` submodules must stay outside this module's graph.** They carry their own `go.mod`
  (ADR-0003), so the driver denials above are what keeps the core clean once they exist; `./...` in the
  root module will not descend into them.
- Deferred: enforcing the test-only ring's *composition* (which test deps each package may use), and a
  generic layer model if the internal graph ever grows past a handful of sanctioned edges.

## References

- Spec v2.0 §3 (layered import graph, "CI-enforced via go mod graph + depguard"), §7 (static gates).
- ADR-0004 (rings), ADR-0033 (`config → validator`), ADR-0003 (contrib submodules), ADR-0009/0018/0024/0027
  (per-module justifications encoded in the allowlists).
- `.golangci.yml` (`linters.settings.depguard`), `tools/import_graph_lint.py`, the `imports` job in
  `.github/workflows/ci.yml`.
- depguard `list-mode` semantics (`strict` vs the default) and its `files` globs (`$all`, `$test`, `!`).
