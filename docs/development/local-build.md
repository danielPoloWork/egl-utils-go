# Local Build & Test

How to build, test, and check `egl-utils-go` on your machine. CI runs the same commands
on Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.25); reproducing them locally avoids a red round-trip.

## Prerequisites

- **Go 1.25+** toolchain.
- **Build system:** go build (go modules).
- **Package manager:** go modules (go.mod / go.sum).
- **Formatter / linter:** gofumpt (gofmt superset), golangci-lint (govet, staticcheck, errcheck, revive, gosec).
- **Docs:** godoc / pkg.go.dev (for the API docs build).

## Commands

```bash
# Build
go build ./...

# Test
go test ./...

# Format check
test -z "$(gofumpt -l .)"

# Lint
golangci-lint run

# Benchmark
go test -bench=. -benchmem ./...

# The four policy checkers (run all before drafting any PR; each gates CI)
python tools/consistency_lint.py     # cross-artifact congruence
python tools/import_graph_lint.py    # dependency rings + internal edges
python tools/coverage_gate.py        # >= 85% statements per package
python tools/spec_api_lint.py        # spec section 5 <-> exported surface
```

## Nested modules

`contrib/*` ([ADR-0040](../adr/0040-contrib-submodules.md)) and `examples/*`
([ADR-0054](../adr/0054-examples-service-module.md)) each carry their own `go.mod`, and **`./...`
from the repository root does not descend into a nested module** — that boundary is what keeps their
dependencies out of the core's graph, and it also means none of the commands above touch them. Build
and test each from its own directory, which is what the `contrib / <module>` and
`examples / <module>` CI jobs do:

```bash
for m in contrib/redishealth contrib/pgxhealth examples/service; do
  (cd "$m" && go build ./... && go vet ./... && go test -race ./... && go mod tidy -diff)
done
```

`import_graph_lint.py` asserts the boundary itself: every one of those directories must have a
`go.mod` whose path matches it, with no `replace`, no committed `go.work` at the repository root, and
the core required at a released tag. Do not add a `go.work` for local convenience — it switches every
root-level `go`, `golangci-lint` and `govulncheck` invocation into workspace mode, and the lint fails
on it deliberately.

## Before you open a PR

1. `test -z "$(gofumpt -l .)"` and `golangci-lint run` are clean.
2. `go test ./...` passes; new/changed behavior is covered (≥ 85% statements, per package).
3. go test -race (data-race detector), go vet, govulncheck are green where applicable.
4. All four policy checkers pass: `consistency_lint.py`, `import_graph_lint.py`,
   `coverage_gate.py`, `spec_api_lint.py`.
5. Every nested module under `contrib/` and `examples/` builds, vets and tests from its own
   directory — the root's `./...` does not reach them.
6. The relevant docs (README, ROADMAP, ADRs, patterns, changelog) are updated in the same
   PR — see [`../workflow/documentation.md`](../workflow/documentation.md).
