# egl-utils-go

> Production-ready Go utilities for concurrency, resilience, HTTP middleware, configuration, and observability.

[![Go Reference](https://pkg.go.dev/badge/github.com/danielPoloWork/egl-utils-go/v2.svg)](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2)
[![CI](https://github.com/danielPoloWork/egl-utils-go/actions/workflows/ci.yml/badge.svg)](https://github.com/danielPoloWork/egl-utils-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/danielPoloWork/egl-utils-go/v2)](https://goreportcard.com/report/github.com/danielPoloWork/egl-utils-go/v2)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Status](https://img.shields.io/badge/Status-v2.0.1-blue)

The building blocks a Go backend service needs and the standard library leaves to you —
bounded worker pools, circuit breakers, rate limiting, HTTP middleware, caching, structured
logging, health and metrics endpoints — as **21 independent packages in one governed module**.

Assembling these yourself normally means a dependency per concern, each with its own release
cadence, licence and supply chain. This module is **three runtime dependencies in total**, and its
packages do not import one another — with **exactly one sanctioned exception**, `config` →
`validator` — so taking one brings at most one sibling, and usually none.

Every package is documented on [pkg.go.dev](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2)
with runnable examples, holds a ≥ 85% per-package test coverage floor, and runs under the race
detector on every change.

## Installation

```bash
go get github.com/danielPoloWork/egl-utils-go/v2
```

Requires **Go 1.25 or later**. Import each package by its own path:

```go
import "github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"
```

## Quickstart

A complete HTTP service with bounded background work, rate limiting, request logging, panic
recovery, a health endpoint and an ordered shutdown — in one file:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/health"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/lifecycle"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/ratelimit"
	"github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool"
)

func main() {
	log := slog.Default()

	// Four workers, a 64-slot queue, and a full queue fails fast instead of
	// parking the caller's goroutine — so saturation becomes a 503, not a backlog
	// of held connections.
	pool := workerpool.New(4, 64, workerpool.WithNonBlockingSubmit())
	limiter := ratelimit.NewLimiter(20, 40) // 20 requests/second, burst of 40

	app := http.NewServeMux()
	app.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		err := pool.Submit(r.Context(), func(context.Context) {
			// ... the slow part, off the request path
		})
		if err != nil {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	mux := http.NewServeMux()
	mux.Handle("/healthz", health.Handler())
	mux.Handle("/", middleware.Recoverer( // a panic becomes a clean 500
		middleware.RequestID( // correlation id before anything logs
			middleware.Logger(log)( // one structured line per request
				limiter.Middleware()(app), // 429 + Retry-After when over budget
			),
		),
	))

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() { _ = srv.ListenAndServe() }()

	// Hooks run in reverse registration order, bounded by the deadline you choose.
	lifecycle.Register(pool.Close)
	lifecycle.Register(srv.Shutdown)
	lifecycle.WaitForSignals(15*time.Second, os.Interrupt, syscall.SIGTERM)
}
```

**Next:** the [usage guide](docs/usage/README.md) answers "how do I…" for each package, and
[`examples/service`](examples/service) is the fuller version of the program above — runnable with
`cd examples/service && go run .`, and exercised by CI.

## Packages

**21 packages, three runtime dependencies, one import path each.** Take only what you need: no
package here imports another, with **exactly one sanctioned exception** — `config` imports
`validator`, because configuration with struct validation is what `config` is for
([ADR-0033](docs/adr/0033-config-struct-validation.md)). The rule is enforced in both directions by
[`tools/import_graph_lint.py`](tools/import_graph_lint.py): an unsanctioned edge fails the build,
and so does a sanctioned edge that has stopped existing — so the allowlist cannot outlive the
composition that justified it ([ADR-0035](docs/adr/0035-import-graph-enforcement.md)).

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

## Documentation

| Where | What you get |
|---|---|
| **[pkg.go.dev](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2)** | The reference: every exported identifier with its contract, and 55 runnable examples |
| **[Usage guide](docs/usage/README.md)** | Task-oriented recipes — "how do I bound concurrency / retry safely / shed load / shut down cleanly" |
| **[`examples/service`](examples/service)** | A complete HTTP service composing eight packages, runnable and exercised by CI |
| **[`contrib/`](contrib)** | Optional driver-backed `health.Check` probes for Redis and PostgreSQL |
| **[`docs/specs/01_spec_utils.md`](docs/specs/01_spec_utils.md)** | The frozen behavioural contract each package is built against |

**`contrib/` is deliberately outside this module.** Each probe is its own module with its own
`go.mod` and tags, so a consumer of the core inherits no database drivers
([ADR-0040](docs/adr/0040-contrib-submodules.md)).

## Compatibility and stability

- **Go 1.25 or later.** CI builds and tests on Go 1.25 and 1.26 across Linux, Windows and macOS.
- **Semantic Versioning.** The exported surface of `v2` is stable; breaking changes require a new
  major and a new import path.
- **`/v2` is the current major.** `v1` remains resolvable from the module proxy but receives no
  fixes — see [`SECURITY.md`](SECURITY.md) for the supported window, and the
  [`v2.0.0` release notes](docs/releases/v2.0.0.md) for the migration.
- **Three runtime dependencies**, all `golang.org/x` or `gopkg.in/yaml.v3`. Every release ships a
  CycloneDX SBOM with a provenance attestation
  ([ADR-0056](docs/adr/0056-build-time-supply-chain.md)).

## Contributing and support

- **Contributing:** [`CONTRIBUTING.md`](CONTRIBUTING.md) — setup, the gates that run before review,
  and when a change needs an ADR. By participating you agree to the
  [Code of Conduct](CODE_OF_CONDUCT.md).
- **Questions, ideas, capability proposals:** [Discussions](https://github.com/danielPoloWork/egl-utils-go/discussions).
- **Bugs:** [open an issue](https://github.com/danielPoloWork/egl-utils-go/issues/new/choose).
- **Security:** never in a public issue — see [`SECURITY.md`](SECURITY.md).

## Project governance

This library is part of the **Enterprise-Grade Libraries** series and is developed under a written
contract: every design decision is recorded as an ADR, every release is gated by the same CI, and
the plan is public.

<details>
<summary><strong>Where the project's own documents live</strong></summary>

| Document | Purpose |
|---|---|
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | The path in for a human contributor — setup, the gates, when a change needs an ADR. |
| [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) | Contributor Covenant 2.1, and how to report a concern privately. |
| [`AGENTS.md`](AGENTS.md) | How AI agents work in this repo — the agent contract. |
| [`ROADMAP.md`](ROADMAP.md) | The numbered plan and what is done. |
| [`ISSUES.md`](ISSUES.md) | The open issue backlog, newest first, with the tier recommended for each. |
| [`docs/adr/`](docs/adr/) | Why it is built the way it is (Architecture Decision Records). |
| [`docs/patterns/`](docs/patterns/) | Design patterns adopted, rejected, or considered. |
| [`docs/workflow/`](docs/workflow/) | Git, documentation, release, and maintenance conventions. |
| [`CHANGELOG.md`](CHANGELOG.md) | User-visible changes per release. |
| [`SECURITY.md`](SECURITY.md) | How to report a vulnerability. |
| [`docs/development/local-build.md`](docs/development/local-build.md) | Full local build and test setup. |

</details>

<details>
<summary><strong>Delivery milestones</strong> — all 14 complete</summary>

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

</details>

## License

MIT © 2026 Daniel Polo. See [`LICENSE`](LICENSE).
