# egl-utils-go

> Production-ready Go utilities for concurrency, resilience, HTTP middleware, configuration, and observability.

![Status](https://img.shields.io/badge/Status-v2.0.1-blue)

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

## Packages

**21 packages, three runtime dependencies, one import path each.** Take only what you need — nothing
here imports anything else here, so a single package pulls in no siblings
([ADR-0035](docs/adr/0035-import-graph-enforcement.md)).

Every package name below links to its **full documentation on pkg.go.dev** — every exported
identifier with its contract, plus **55 runnable `Example` functions** whose output is verified by
`go test`, so they cannot drift into fiction
([ADR-0053](docs/adr/0053-runnable-examples-convention.md)).

### Concurrency

| Package | What it does |
|---|---|
| [`workerpool`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool) | A bounded, context-aware goroutine pool. A fixed set of workers drains a bounded queue, so callers get backpressure — block until space frees, or fail fast with `ErrQueueFull` — instead of unbounded goroutine growth. |
| [`pubsub`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub) | An in-memory publish–subscribe broker over channels, with per-subscription filters and a choice of slow-subscriber policy. `Publish` never blocks on a slow subscriber. |
| [`fanin`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/fanin) | Merges multiple input channels into one — the fan-in half of the Go pipelines vocabulary. |
| [`fanout`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/fanout) | Distributes one input channel across multiple outputs — the fan-out half. |
| [`semaphore`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/semaphore) | A weighted counting semaphore for admission control: acquire weight before the work, release it after, with the total bounded to a fixed capacity. |

### Resilience

| Package | What it does |
|---|---|
| [`circuitbreaker`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/circuitbreaker) | Guards calls to an unreliable dependency with a closed/open/half-open state machine: fails fast with `ErrOpen` while the dependency recovers, then re-admits a bounded number of probes. |
| [`retry`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/retry) | Runs a call until it succeeds, its attempt budget is spent, or its context ends — sleeping between attempts with exponentially growing, jittered, hard-capped delays. |
| [`ratelimit`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/ratelimit) | A token bucket: burst capacity refilling continuously at *rate* per second. `Allow` fails fast, `Wait` queues, and `Middleware` answers `429` with a `Retry-After`. |

### HTTP

| Package | What it does |
|---|---|
| [`middleware`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware) | Composable `net/http` middleware as standard decorators — `RequestID`, `Logger`, `Recoverer` and `Cors`. The package documentation shows the order to compose them in, and why. |

### Configuration

| Package | What it does |
|---|---|
| [`config`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/config) | Loads typed configuration from a JSON or YAML file, with optional `${VAR}` expansion and post-load validation. |
| [`env`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/env) | Reads environment variables with safe typed fallbacks — string, int, bool and duration. |

### Observability and errors

| Package | What it does |
|---|---|
| [`logger`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/logger) | Builds structured `slog` loggers tuned for log aggregation, and carries per-request fields through a `context.Context`. |
| [`metrics`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/metrics) | Instruments HTTP handlers and serves what it records in Prometheus text exposition format — **without depending on the Prometheus SDK** ([ADR-0050](docs/adr/0050-metrics-without-the-sdk.md)). |
| [`health`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/health) | An HTTP health-check handler that runs dependency probes concurrently. The response names *which* check failed and never *why*, so an unauthenticated caller learns nothing about your infrastructure. |
| [`errx`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/errx) | Adds a context message and, **on request**, a captured call stack to an error — fully interoperable with `errors.Is`, `errors.As` and `errors.Unwrap`. Wrapping never touches the runtime unless you ask it to. |

### Data and storage

| Package | What it does |
|---|---|
| [`cache`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/cache) | A generic in-memory key–value cache with TTL expiry and a background sweeper. Sharded internally, so a write does not lock the whole keyspace ([ADR-0038](docs/adr/0038-cache-sharding.md)). |
| [`db`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/db) | Runs a function inside a SQL transaction: commit on success, roll back on error — and roll back then re-panic on panic, which is the case a hand-written `defer` usually gets wrong. |
| [`syncpool`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/syncpool) | A pool of reusable `*bytes.Buffer` values, to relieve GC pressure on temporary-buffer hot paths such as serialization and string building. |

### Validation and security

| Package | What it does |
|---|---|
| [`validator`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/validator) | Validates struct values against rules declared in `validate:"..."` struct tags, aggregating every failure rather than stopping at the first. |
| [`hash`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/hash) | Hashes and verifies passwords with bcrypt, at a work factor the module owns rather than inherits, with a cost-upgrade path for existing hashes. |

### Lifecycle

| Package | What it does |
|---|---|
| [`lifecycle`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/lifecycle) | Coordinates ordered shutdown of a process's resources — servers, pools, queues — on a termination signal or an explicit call. Hooks run in reverse registration order, under a deadline you choose. |

**Seeing them work together** is a different question from what any one package documents, and it has
its own answer: [`examples/service`](examples/service) is a runnable HTTP service composed from eight
of these packages. `cd examples/service && go run .` — no configuration needed.

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
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | The path in for a human contributor — setup, the gates, when a change needs an ADR. |
| [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) | Contributor Covenant 2.1, and how to report a concern privately. |
| [`AGENTS.md`](AGENTS.md) | How AI agents work in this repo — the agent contract. |
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
| 14 | Adoption: examples, the release act, and the supply chain | ✅ done |


## License

MIT © 2026 Daniel Polo. See [`LICENSE`](LICENSE).
