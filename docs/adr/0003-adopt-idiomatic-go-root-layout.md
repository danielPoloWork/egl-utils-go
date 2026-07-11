# ADR-0003: Adopt the idiomatic Go root layout

- **Status:** Accepted
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), with the architect agent
- **Related:** ADR-0002 (superseded), AGENTS.md §5, ROADMAP 1.1/1.6, spec §4/§5

## Context

ADR-0002 adopted the series' Maven-style cross-language tree (`src/main/go/it/d4np/utils/`)
and, in the same document, promised consumers the short import form
`import "github.com/danielPoloWork/egl-utils-go/workerpool"` — a promise also rendered into
README.md, AGENTS.md §5, and spec §5. In Go these two statements are mutually incompatible:
an import path *is* the directory path under the module root, so code under the tree can only
be imported as `github.com/danielPoloWork/egl-utils-go/src/main/go/it/d4np/utils/workerpool`
(~70 characters), and no vanity-path or subdirectory-module mechanism shortens it. For a
public library whose stated objective is adoption by Go backend services — and whose design
philosophy line one is "idiomatic Go" — the import ergonomics are part of the public API.

## Decision

The module root is the repository root: `go.mod` declares
`module github.com/danielPoloWork/egl-utils-go` (language floor `go 1.24`). Each feature
lives in its own package directory at the root (`workerpool/`, `pubsub/`, `circuitbreaker/`,
…), subdivided by component exactly as ADR-0002 prescribed *inside* the old tree. The root
package `utils` carries module-wide metadata (`version.go`, `doc.go`). Tests are co-located
`_test.go` files (white-box in-package plus external `_test` packages as appropriate);
benchmarks are co-located `Benchmark*` functions run via `go test -bench`. The
`src/{main,test,bench}/go/it/d4np/utils/` tree is removed. This supersedes ADR-0002 **for
this repository**; ADR-0002's rationale stands for series siblings whose languages do not
bind import paths to directory layout.

## Alternatives Considered

- **Full tree with a root `go.mod`** — keeps ADR-0002 intact; every consumer import carries
  `/src/main/go/it/d4np/utils/`. Rejected: consumer-hostile for a public library and
  contradicts the short-import promise already rendered in three governing documents.
- **Subdirectory module with a suffixed module path** — `go.mod` inside the tree, module path
  carrying the full suffix. Rejected: identical long imports, plus subdirectory-prefixed
  release tags and per-job `working-directory` overrides in CI — strictly more ceremony for
  the same ergonomics.
- **Hybrid (packages at root, test/bench kept in the tree)** — preserves part of the series
  shape. Rejected: splits the test topology across two conventions; white-box tests must
  co-locate in Go regardless, so the tree would hold only a fraction of the suites.

## Consequences

- Consumers get `import "github.com/danielPoloWork/egl-utils-go/<package>"` — the promise in
  README/AGENTS/spec §5 is now true, and `version_test.go` (an external test package
  importing the module path) verifies it mechanically on every CI run.
- Series uniformity is broken for Go: this repo's shape diverges from tree-based siblings.
  Accepted as the cost of a consumable public library.
- `tools/consistency_lint.py` CONFIG now points at root paths (`version.go`); AGENTS.md
  §4/§5/§10, README, ROADMAP 1.1/1.2, the patterns and benchmarks docs, and spec §4 are
  updated in this same PR (the docs-sync rule).
- **Regeneration caveat:** `orchestrator/project.yaml` remains the historical interview
  record and still describes the tree; re-rendering from it would re-impose the old layout.
  On any conflict, this ADR wins — a future regeneration must honor it.

## References

- AGENTS.md §5 (Source Tree), §10 (quality bar).
- Go modules reference — module path / import path binding.
- ADR-0002 (superseded for this repository).
