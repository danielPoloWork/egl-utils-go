# ADR-0041: Adopt `it.d4np.utils.<component>` as the series' logical namespace

- **Status:** Accepted
- **Date:** 2026-07-27
- **Deciders:** Maintainer (Daniel Polo), with the architect agent
- **Related:** ADR-0002 (superseded), ADR-0003, ADR-0030 (`/v2` ledger), ADR-0040, AGENTS.md §5

## Context

ADR-0002 gave the EGL series one shared shape — the Maven-style tree
`src/main/<lang>/it/d4np/utils/` — on the premise that a fixed physical layout is what makes an
agent's mental model transferable between sibling repositories. Three years of the series later,
that premise no longer matches what the siblings actually ship:

| Repository | Source layout as shipped | Local decision |
|---|---|---|
| `egl-util-cpp` | `src/{main,test,bench,fuzz}/cpp/it/d4np/util/` | ADR-0002 *adopt the cross-language source layout* — tree kept |
| `egl-utils-c` | `d4np/{core,concurrency,ds,io,mem,str,sys}/` at the root | ADR-0002 *adopt a **module-oriented** source layout* — tree rejected |
| `egl-utils-go` | one feature package per directory at the module root | ADR-0003 — tree rejected |
| `egl-utils-java` | not yet scaffolded | Maven mandates `src/main/java/it/d4np/utils/` natively |

Two of the three scaffolded repositories abandoned the tree independently, each because its
language binds names to paths in a way the tree fights. Go is the sharpest case: an import path
**is** the directory path beneath the module root, so packages under the tree could only ever be
imported as `github.com/danielPoloWork/egl-utils-go/src/main/go/it/d4np/utils/workerpool` — the
incompatibility ADR-0003 resolved in favour of the short import.

Re-imposing the tree here was raised again and rejected, but the question exposed the real defect
in ADR-0002: it fixed the **directory shape** when what the series needs fixed is the
**namespace**. A physical layout cannot be a cross-language contract, because in several of these
languages the layout is not free — it is dictated by the build system (Maven), by the compiler's
include model (C/C++ headers), or by the module system (Go). A namespace can be, because every one
of these languages has *some* native way to spell one.

## Decision

The series' unit of identity is the **logical namespace `it.d4np.utils.<component>`**, not a
physical directory tree. Each repository realizes that namespace through its language's native
binding idiom, and `<component>` is spelled identically in every repository — the component names
are the contract, the directories are an implementation detail of each ecosystem.

| Language | Native binding idiom | `it.d4np.utils.workerpool` is realized as |
|---|---|---|
| Java | Maven source root + package | `src/main/java/it/d4np/utils/workerpool/` |
| C++ | source tree + `namespace` | `src/main/cpp/it/d4np/utils/workerpool/` |
| C | prefixed public headers | `d4np/workerpool/` → `d4np_workerpool_*` |
| Go | module path + package directory | `github.com/danielPoloWork/egl-utils-go/workerpool` |

**For this repository the module path does not change and no `/v2` is opened for namespace
reasons.** ADR-0003's idiomatic Go root layout is reaffirmed, and the
`src/main/go/it/d4np/utils/` tree is rejected permanently for Go rather than merely for the
duration of a milestone.

## Alternatives Considered

- **Re-impose the Maven tree in Go.** Rejected on arithmetic, not taste. The consumer import goes
  from 52 to 83 characters; because relocating packages breaks every existing import under the
  v1.0.0 stability commitment, the move itself forces a `/v2`, taking the import to 86. ADR-0040's
  nested modules inherit the prefix in both their module paths *and* their release tags
  (`src/main/go/it/d4np/utils/contrib/redishealth/v1.0.0`). No mechanism shortens any of this:
  a `go-import` meta tag maps an import prefix onto a *repository root*, and the remaining path
  elements are directories within that repository, so a vanity path cannot hide the prefix; a
  nested `go.mod` cannot either, since a subdirectory module's path must equal repository path plus
  subdirectory. ADR-0003 evaluated both. Finally, the specification's own first design principle is
  "Idiomatic Go" and its worked example imports `d4np-go/lifecycle` — the spec argues against the
  tree.
- **Vanity module path `go.d4np.it/utils/<component>`.** The only literal rendering of the
  namespace in Go source: reverse-DNS `it.d4np` → `d4np.it`, a `go.` label for the language, then
  `utils` and the component, so `it.d4np.utils.workerpool` ↔ `go.d4np.it/utils/workerpool` maps
  element for element while staying short. Rejected: it buys the spelling at the price of a
  permanent operational commitment — a domain that must keep resolving and a `go-import` meta tag
  that must keep being served for as long as anyone runs `go get`. If either lapses, resolution of
  *new* versions breaks for every consumer not already served by a warm module proxy, converting a
  documentation preference into a supply-chain dependency. It is also breaking regardless, since a
  module path change is a module *identity* change.
- **GitHub organisation `github.com/d4np/utils-go/<component>`.** Carries the brand with no domain
  and no hosting commitment. Rejected: it renders `d4np` but neither `it.` nor a clean `utils`, so
  it pays a repository transfer and a breaking import change for a partial match — the worst ratio
  of the three.

## Consequences

- **Nothing in the Go source tree moves.** No package relocation, no import rewrite, no tag-scheme
  change, no tooling change — `tools/consistency_lint.py` already carries `src_main: "."` per
  ADR-0003, and `tools/import_graph_lint.py` and `tools/coverage_gate.py` are unaffected. Consumers
  on `github.com/danielPoloWork/egl-utils-go/<package>` are untouched.
- **Conditional reopen, recorded in ADR-0030 §2.** The module-path move is free at a `/v2`
  boundary and only there: consumers rewrite every import at such a boundary anyway, so the vanity
  path would cost nothing extra. If ADR-0030's bucket 2 is ever opened, re-evaluate it then.
  Outside such a boundary the answer stays no.
- **ADR-0003's regeneration caveat is closed.** That ADR flagged that `orchestrator/project.yaml`
  still described the tree and that re-rendering would re-impose it, resolving the conflict only by
  declaring ADR precedence. The record is now amended in place with a dated note, so it still shows
  what the interview asked while the generator can no longer act on it.
- **One upstream action remains outside this repository's reach.** The EADOS bundle's Go profile
  (`.eados-core/orchestrator/profiles/go.yaml`) still asserts that the Go module is placed "under
  `src/main/go/<group>/<slug>`". `.eados-core/` is gitignored — the bundle is factory tooling, not
  this project's source — so the correction cannot be committed here and must be carried to the
  framework. Until it is, a regeneration from a *fresh* bundle can still reintroduce the claim;
  this ADR and the amended manifest are what a future agent should trust instead.
- **A live series inconsistency becomes visible and actionable.** `egl-util-cpp` ships
  `it/d4np/util` (singular) where this contract says `utils`. Under the component-name rule one of
  the two must move. That is a series-level decision affecting another repository and is therefore
  *not* taken here.
- The series loses physical uniformity as a stated goal and gains namespace uniformity in its
  place. An agent arriving in a sibling repository must now read that repository's layout ADR
  rather than assume the tree — which is already the situation in three of the four repositories,
  now made explicit rather than discovered.

## References

- ADR-0002 (superseded for this repository), ADR-0003 (idiomatic Go root layout).
- ADR-0030 §2 — the `/v2` ledger and this decision's addendum to it.
- ADR-0040 — contrib submodule paths and tag scheme, which inherit any module-path prefix.
- Go modules reference — module paths, `go-import` meta tags, subdirectory modules.
- AGENTS.md §5 (Source Tree & Layout).
