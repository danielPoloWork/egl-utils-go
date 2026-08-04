# egl-utils-go

> Production-ready Go utilities for concurrency, resilience, HTTP middleware, configuration, and observability.

![Status](https://img.shields.io/badge/Status-v2.0.0-blue)

Part of the **Enterprise-Grade Libraries** series. A
library written in **Go 1.25+**, built and governed to an enterprise quality
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
- **Supported platforms:** Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.25).
- Consumers import the public surface via: `import "github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"`,
  and the module root as `import "github.com/danielPoloWork/egl-utils-go/v2"` for `utils.Version`.
- **[`contrib/`](contrib) holds separate modules**, each with its own `go.mod` — driver-backed
  `health.Check` probes for Redis and PostgreSQL, kept out of this module so a consumer inherits
  no driver dependencies. `./...` does not descend into a nested module, so they are built and
  tested from their own directories (`cd contrib/redishealth && go test ./...`) and version
  independently. See [`contrib/README.md`](contrib/README.md) and
  [ADR-0040](docs/adr/0040-contrib-submodules.md).
- **[`examples/`](examples) holds runnable programs**, also as separate modules. Start with
  [`examples/service`](examples/service) — one HTTP service composed from eight packages, showing
  what a package's documentation cannot: the middleware chain order, the operational endpoints kept
  out of it, liveness and readiness as two different questions, and an ordered shutdown. Run it with
  `cd examples/service && go run .` — no configuration needed. See
  [ADR-0054](docs/adr/0054-examples-service-module.md). Every package additionally carries runnable
  `Example` functions on pkg.go.dev ([ADR-0053](docs/adr/0053-runnable-examples-convention.md)).

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
| 10 | Spec v2 reconciliation (v1.x additive) | ✅ done |
| 11 | Governance: namespace contract & spec reconciliation | ✅ done |
| 12 | Public-interface reconciliation | ✅ done |
| 13 | `/v2` — `pkg/` layout and the ledger emptied | ✅ done |
| 14 | Adoption: examples, the release act, and the supply chain | 🚧 in progress |


## License

MIT © 2026 Daniel Polo. See [`LICENSE`](LICENSE).
