# egl-utils-go

> Production-ready Go utilities for concurrency, resilience, HTTP middleware, configuration, and observability.

![Status](https://img.shields.io/badge/Status-v1.0.0-blue)

Part of the **Enterprise-Grade Libraries** series. A
library written in **Go 1.24+**, built and governed to an enterprise quality
bar: full CI matrix, static analysis, sanitizers, documented design decisions, and SemVer
releases.

## What it is

Provide a production-ready Go utilities module — advanced concurrency primitives,
resilience patterns, high-performance HTTP middleware, and API-development helpers —
that removes boilerplate and correctness risk (goroutine leaks, GC pressure, unsafe
shutdown) from Go backend services. Design philosophy (imported from the brief):
idiomatic Go throughout (channels, context.Context, the error interface); zero goroutine
leaks — every internal goroutine stops deterministically via context or close(done);
allocation-conscious hot paths via pointer discipline and sync.Pool object reuse.

The frozen specification is in
[`docs/specs/01_spec_utils.md`](docs/specs/01_spec_utils.md).

## Build, test, run

```bash
go build ./...
go test ./...
```

- **Toolchain:** go build (go modules), go test (+ testify; rapid for property tests), gofumpt (gofmt superset), golangci-lint (govet, staticcheck, errcheck, revive, gosec).
- **Supported platforms:** Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.24).
- Consumers import the public surface via: `import "github.com/danielPoloWork/egl-utils-go/workerpool"`.

See [`docs/development/local-build.md`](docs/development/local-build.md) for the full local
setup.

## How this project is run

| Document | Purpose |
|---|---|
| [`AGENTS.md`](AGENTS.md) | How AI agents (and humans) work in this repo — the contract. |
| [`ROADMAP.md`](ROADMAP.md) | The numbered plan and what is done. |
| [`docs/adr/`](docs/adr/) | Why it is built the way it is (Architecture Decision Records). |
| [`docs/patterns/`](docs/patterns/) | Design patterns adopted, rejected, or considered. |
| [`docs/workflow/`](docs/workflow/) | Git, documentation, release, and maintenance conventions. |
| [`CHANGELOG.md`](CHANGELOG.md) | User-visible changes per release. |
| [`SECURITY.md`](SECURITY.md) | How to report a vulnerability. |

## Milestones

| # | Title | Status |
|---|---|---|
| 1 | Project bootstrap & CI | ✅ done |
| 2 | Concurrency primitives | ✅ done |
| 3 | Resilience patterns | ✅ done |
| 4 | HTTP middleware | ✅ done |
| 5 | Configuration & environment | ✅ done |
| 6 | Structured logging | ✅ done |
| 7 | Caching & data helpers | ✅ done |
| 8 | Validation & security | ✅ done |
| 9 | Diagnostics & lifecycle | ✅ done |
| 10 | Spec v2 reconciliation (v1.x additive) | ⏳ in progress |


## License

MIT © 2026 Daniel Polo. See [`LICENSE`](LICENSE).
