# Local Build & Test

How to build, test, and check `egl-utils-go` on your machine. CI runs the same commands
on Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.24); reproducing them locally avoids a red round-trip.

## Prerequisites

- **Go 1.24+** toolchain.
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

# Cross-artifact congruence (run before drafting any PR)
python tools/consistency_lint.py
```

## Before you open a PR

1. `test -z "$(gofumpt -l .)"` and `golangci-lint run` are clean.
2. `go test ./...` passes; new/changed behavior is covered (≥ 80% line).
3. go test -race (data-race detector), go vet, govulncheck are green where applicable.
4. `python tools/consistency_lint.py` passes.
5. The relevant docs (README, ROADMAP, ADRs, patterns, changelog) are updated in the same
   PR — see [`../workflow/documentation.md`](../workflow/documentation.md).
