# examples/service — one HTTP service, composed

A small, complete service built from `egl-utils-go` and nothing else. It exists
to show the part no package's documentation can: **the arrangement.**

Every package in the library carries runnable `Example` functions for its own
calls ([ADR-0053](../../docs/adr/0053-runnable-examples-convention.md) — 55 of
them across 21 packages). What none of them can show is a decision about two
packages at once: which middleware goes outside which, which endpoints stay out
of the chain entirely, what a readiness probe should actually probe, and the order
shutdown hooks have to be registered in. Each of those is correct or wrong only in
composition, so it has no home in either package's docs.

## Running it

No configuration required — every setting has a working default.

```bash
cd examples/service
go run .
```

```bash
curl -i -X POST localhost:8080/orders    # 202, with an X-Request-ID
curl -s  localhost:8080/healthz          # liveness: is the process up?
curl -s  localhost:8080/readyz           # readiness: can it take work?
curl -s  localhost:8080/metrics          # what the middleware recorded
```

Then press Ctrl-C and watch the shutdown run its hooks in reverse registration
order: the listener closes first, then the worker pool drains.

| Variable | Default | What it does |
|---|---|---|
| `SERVICE_ADDR` | `:8080` | listen address |
| `SERVICE_DEBUG` | `false` | debug-level logging |
| `SERVICE_WORKERS` | `4` | worker-pool size |
| `SERVICE_QUEUE_SIZE` | `64` | queued tasks before requests are shed |
| `SERVICE_RATE_LIMIT` | `20` | admitted requests per second |
| `SERVICE_BURST` | `40` | burst the bucket starts with |
| `SERVICE_ALLOWED_ORIGIN` | `https://app.example.com` | the one CORS origin |
| `SERVICE_SHUTDOWN_GRACE` | `15s` | bound on the whole shutdown sequence |

A malformed value is treated as unset, so a typo degrades one setting to its
default instead of failing the process — that is `env`'s contract, and
`TestLoadConfigFallsBackPerField` pins it.

## What it composes

Eight packages, and one thing each:

| Package | Used for |
|---|---|
| [`env`](../../pkg/env) | configuration, with a safe fallback per field |
| [`logger`](../../pkg/logger) | the base JSON logger, and per-request fields carried on the context |
| [`middleware`](../../pkg/middleware) | `Recoverer`, `RequestID`, `Logger`, `Cors` |
| [`ratelimit`](../../pkg/ratelimit) | the 429 admission path on user traffic only |
| [`metrics`](../../pkg/metrics) | the recorder and the scrape endpoint |
| [`health`](../../pkg/health) | liveness and readiness, as two different things |
| [`workerpool`](../../pkg/workerpool) | bounded background work with backpressure |
| [`lifecycle`](../../pkg/lifecycle) | ordered shutdown on `SIGINT`/`SIGTERM` |

It deliberately does **not** import all twenty-one. A demo that imported
everything would make every package look mandatory and would teach nothing about
composition; the packages it leaves out — `cache`, `retry`, `circuitbreaker`,
`pubsub`, `db`, `hash`, `validator` and the rest — each have their own runnable
examples, and none of them changes the shape of the service skeleton.

## The four decisions worth copying

### 1. Chain order, outermost first

```
Recoverer → RequestID → Logger → metrics → ratelimit → Cors → mux
```

- **Recoverer outside everything**, so a panic anywhere inside — including inside
  another middleware — still becomes a 500.
- **RequestID before Logger**, so the correlation id exists before anything logs.
- **Logger and metrics outside the limiter**, so a shed request is logged *and*
  counted. The rate of 429s is precisely what tells an operator the limit is set
  wrong; a limiter inside the recorder makes its own effect invisible.
- **Cors closest to the handler**, so the terminal 204 it writes for a preflight
  is logged and counted like any other response.
- **The chain wraps the mux, not each route**, so a 404 or 405 the mux itself
  produces is logged and counted too.

### 2. Operational endpoints stay out of the application chain

`/healthz`, `/readyz` and `/metrics` are behind `Recoverer` and nothing else.
Every omission is a decision:

- **Not rate-limited.** A limiter that answers 429 to a readiness probe gets a
  healthy instance killed by its orchestrator, and a scrape must not spend user
  traffic's tokens.
- **Not counted.** A 15-second scrape would add ~5 800 requests a day to
  `http_requests_total` that the service never served for anyone.
- **Not logged.** A probe every 10 seconds plus a scrape every 15 is ~14 000 lines
  a day correlated with no user request.
- **Not CORS-negotiated.** No browser calls them.

`Recoverer` stays, because a panicking probe must not take the process down.

### 3. Liveness and readiness answer different questions

`/healthz` carries **no checks at all** — it answers "this process is running".
`/readyz` carries the dependency probe. Giving the dependency probe to liveness is
the classic mistake: one dependency's blip then restarts every instance at once,
turning a partial outage into a restart storm. Readiness failing only removes one
instance from the load balancer, which is the correct response.

And the readiness probe **exercises the real admission path** rather than
returning `nil`: it submits a no-op task through the same `Submit` the request
handler uses, so the pool's own errors are already the three answers an
orchestrator needs — `ErrClosed` (shutting down, stop routing here),
`ErrQueueFull` (saturated, route elsewhere), `nil` (can take work). A probe that
succeeds unconditionally documents the wiring and verifies nothing.

For an *out-of-process* dependency the probe needs that dependency's driver, which
the core module is not allowed to import — that is what [`contrib/`](../../contrib)
is for.

### 4. Registration order is dependency order

```go
lifecycle.Register(svc.pool.Close)   // registered first  → closed last
lifecycle.Register(srv.Shutdown)     // registered last   → closed first
```

Hooks run in reverse registration order, so this reads like a stack of `defer`s.
The direction is the point: stopping the server first means no new request can
enqueue work while the pool drains. Closing the pool first would leave the
listener accepting orders it can only answer with a 503.

`WaitForSignals`' first argument bounds the whole sequence, measured from the
moment the signal arrives rather than from the call — so a service up for a week
still gets its full grace period. Passing `0` imposes no deadline and leaves the
bound to the platform's kill escalation, which is the better choice when a grace
period is already configured there
([ADR-0051](../../docs/adr/0051-lifecycle-shutdown-timeout.md)).

## What adopting this library costs a consumer

`go.mod` here has **one `require` line and no indirect requirements at all**, and
`go list -deps .` resolves 203 packages of which **zero are third-party** — every
one is either the standard library or `egl-utils-go` itself. That is not a claim
about this demo; it is the module's dependency policy
([ADR-0004](../../docs/adr/0004-runtime-dependency-policy.md)) made visible, and
this is the smallest program that can demonstrate it.

The tests are stdlib-only for the same reason: no `testify`, no `goleak`. A module
whose whole point is showing what a single dependency buys you should not need a
second one to test itself.

## Tests

```bash
go test ./...
go test -race ./...
```

Twelve tests, and they are not decoration: every claim above is asserted by one,
so a rearrangement that breaks the advice fails the build. The pointed ones:

- `TestOperationalEndpointsAreNotRateLimited` — with the bucket set to one token,
  the second order is refused while `/healthz`, `/readyz` and `/metrics` answer
  five times each.
- `TestRecorderCountsSheddingAndIgnoresScrapes` — the 429 appears in the
  exposition, the scrape does not.
- `TestLivenessIgnoresDependenciesThatReadinessReports` — with the pool closed,
  `/healthz` is 200 and `/readyz` is 503; and the 503 body names *which* check
  failed and never why, so an unauthenticated endpoint leaks no internals.
- `TestPanickingTaskDoesNotStopTheWorker` — with a single worker, the task
  submitted after the panicking one still runs.
- `TestBackgroundWorkCompletesAfterTheResponse` — the handler's background work
  outlives the request, which is why it must not be built on `r.Context()`.

No test sleeps. Where ordering matters, the test waits on a channel the task
itself signals — the same rule
[ADR-0053](../../docs/adr/0053-runnable-examples-convention.md) imposes on the
package examples, and for the same reason: in a module built to remove
goroutine-timing bugs, documentation that sleeps in order to work teaches the
habit the library replaces.

## Why this is its own module

Because a directory of `.go` files with no `go.mod` joins the root module, and
then everything this service imports is in the *library's* dependency graph while
every `./...` check at the repository root keeps passing. See
[`../README.md`](../README.md) and
[ADR-0054](../../docs/adr/0054-examples-service-module.md);
`tools/import_graph_lint.py` enforces it.
