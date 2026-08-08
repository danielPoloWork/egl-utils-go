# Usage guide

Task-oriented recipes: **"how do I…"** for each package, with the smallest code that answers it.

This sits between the [README](../../README.md)'s quickstart and the full reference on
[pkg.go.dev](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2), which documents every
exported identifier and carries 55 runnable examples. When a recipe here leaves you wanting the
detail — the option you did not use, the error you might get back — the package's reference page is
the next stop, and every heading links to it.

```bash
go get github.com/danielPoloWork/egl-utils-go/v2
```

**Contents**

- [Concurrency](#concurrency) — [background work](#run-background-work-without-unbounded-goroutines) · [bound concurrency](#bound-how-many-things-run-at-once) · [pipelines](#fan-work-out-and-back-in) · [publish/subscribe](#broadcast-events-in-process)
- [Resilience](#resilience) — [retry](#retry-a-flaky-call) · [circuit breaker](#stop-calling-a-dependency-that-is-down) · [rate limiting](#shed-load-before-it-reaches-your-handler)
- [HTTP](#http) — [middleware chain](#compose-http-middleware-in-the-right-order)
- [Observability](#observability) — [structured logs](#get-structured-logs-correlated-per-request) · [metrics](#expose-prometheus-metrics) · [health](#answer-liveness-and-readiness-probes) · [errors](#add-context-to-an-error)
- [Configuration](#configuration) — [files](#load-configuration-from-a-file) · [environment](#read-environment-variables-safely) · [validation](#validate-a-struct)
- [Data](#data) — [cache](#cache-expensive-lookups) · [transactions](#run-a-function-in-a-sql-transaction) · [buffers](#reuse-buffers-on-a-hot-path)
- [Security](#security) — [passwords](#hash-and-verify-passwords)
- [Lifecycle](#lifecycle) — [shutdown](#shut-down-in-the-right-order)

---

## Concurrency

### Run background work without unbounded goroutines

`go doSomething()` on a request path is unbounded: enough traffic and you have a million goroutines.
[`workerpool`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/workerpool) gives you
a fixed number of workers draining a bounded queue.

```go
pool := workerpool.New(4, 64, workerpool.WithNonBlockingSubmit())
defer pool.Close(context.Background())

err := pool.Submit(ctx, func(context.Context) {
    // the slow part, off the request path
})
if err != nil {
    // ErrQueueFull — the queue is saturated. Shed the request; do not wait.
}
```

**Choose the submission mode deliberately.** The default blocks until the queue has space, which is
right for a batch job draining a fixed work list. On an HTTP path use `WithNonBlockingSubmit()`:
blocking there parks the request's goroutine *and its connection*, turning a saturated pool into an
unbounded backlog of held connections. Failing fast turns it into a `503` the caller can retry.

Add `workerpool.WithPanicHandler(func(recovered any) { ... })` in a long-lived service, so one bad
task does not take the process down — but log the panic, or you have traded a crash for silence.

### Bound how many things run at once

When the work is not queued but concurrent — say, fan-out to an upstream that will not tolerate more
than *N* in flight — use
[`semaphore`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/semaphore).

```go
sem := semaphore.NewWeighted(10)

if err := sem.Acquire(ctx, 1); err != nil {
    return err // the context ended while waiting
}
defer sem.Release(1)
```

Weights let one expensive call reserve more of the budget than a cheap one.

### Fan work out and back in

[`fanout`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/fanout) splits one
channel across several; [`fanin`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/fanin)
merges several into one. Both stop when the context ends, and neither leaks a goroutine.

```go
// Split takes send-ownership of the outputs and closes each when the input is
// done — so you range over them and never close one yourself.
first, second := make(chan Job), make(chan Job)
fanout.Split(ctx, in, first, second) // starts the forwarders, returns immediately

merged := fanin.Merge(ctx, resultsA, resultsB) // many streams → one
for r := range merged { /* ... */ }
```

Which output receives which value depends on whichever consumer is ready first, so only the totals
are deterministic — do not build ordering assumptions on top of `Split`.

### Broadcast events in process

[`pubsub`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub) is an in-memory
broker over channels. **`Publish` never blocks on a slow subscriber** — that is the whole point, and
it is why the slow-subscriber policy is a choice you make rather than a surprise you discover.

```go
broker := pubsub.NewBroker[Event]()
defer broker.Close()

// Cancelling this context is the unsubscribe — there is no second lifetime to manage.
events := broker.Subscribe(ctx, "orders", func(e Event) bool { return e.Region == "eu" })

_ = broker.Publish(ctx, "orders", Event{Region: "eu"})
for e := range events { /* ... */ }
```

---

## Resilience

### Retry a flaky call

[`retry`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/retry) runs a call until
it succeeds, its attempt budget is spent, or its context ends — with exponential backoff and jitter.

```go
policy := retry.Policy{MaxAttempts: 3}

err := retry.Backoff(ctx, policy, func(context.Context) error {
    return callUpstream(ctx)
})
```

**Jitter is not decoration.** Without it, every instance that failed at the same moment retries at
the same moment, and the retry storm finishes what the outage started.

Retry only what is worth retrying: a `404` will still be a `404` in 200 ms. Wrap the call so
permanent failures return early rather than burning the budget.

### Stop calling a dependency that is down

Retrying a dead dependency just moves the queue. A
[`circuitbreaker`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/circuitbreaker)
fails fast while it recovers, then re-admits a bounded number of probes.

```go
breaker := circuitbreaker.New(circuitbreaker.WithFailureThreshold(5))

err := breaker.Do(ctx, func() error {
    return callUpstream(ctx)
})
// ErrOpen means the breaker refused without calling — serve a fallback.
```

`breaker.State()` is safe to read for a dashboard or a readiness signal.

### Shed load before it reaches your handler

[`ratelimit`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/ratelimit) is a token
bucket: `burst` capacity refilling at `rate` per second.

```go
limiter := ratelimit.NewLimiter(20, 40) // 20/s sustained, absorb bursts of 40

if !limiter.Allow() {
    // over budget — refuse, do not queue
}
```

As HTTP middleware it answers `429` with a `Retry-After` header:

```go
mux.Handle("/api/", limiter.Middleware()(apiHandler))
```

`Allow` fails fast; `Wait(ctx)` queues for the next token. Use `Allow` on a request path — queueing
there converts a rate problem into a latency problem.

---

## HTTP

### Compose HTTP middleware in the right order

[`middleware`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware) provides
`RequestID`, `Logger`, `Recoverer` and `Cors` as ordinary decorators. **The order is load-bearing:**

```go
handler := middleware.Recoverer( // 1. outermost: a panic anywhere becomes a clean 500
    middleware.RequestID( // 2. the id must exist before anything logs it
        middleware.Logger(log)( // 3. outside the limiter, so a 429 is still logged
            limiter.Middleware()( // 4. shed before routing costs anything
                middleware.Cors(middleware.CorsConfig{ // 5. nearest the handler
                    AllowedOrigins: []string{"https://app.example.com"},
                    AllowedMethods: []string{http.MethodGet, http.MethodPost},
                    AllowedHeaders: []string{"Content-Type"},
                    MaxAge:         10 * time.Minute,
                })(app),
            ),
        ),
    ),
)
```

`Recoverer` never leaks the panic value or a stack trace to the client — it logs them server-side and
writes a bare `500`. CORS is deny-by-default: the zero `CorsConfig` allows no origin, and combining
credentials with `"*"` panics at construction rather than serving an open door.

---

## Observability

### Get structured logs correlated per request

[`logger`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/logger) builds `slog`
loggers and carries per-request fields through a `context.Context`.

```go
log := logger.NewStructured(logger.WithLevel(slog.LevelInfo))

// In a handler: attach the request id once, and every later line carries it.
ctx := logger.WithFields(r.Context(), logger.String("request_id", id))
logger.FromContext(ctx).Info("order accepted")
```

Pair it with `middleware.RequestID`, which generates the id and puts it on the request context, so
one request's lines are findable in an aggregator holding a hundred other services.

### Expose Prometheus metrics

[`metrics`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/metrics) instruments
handlers and serves the Prometheus text format **without depending on the Prometheus SDK**.

```go
rec := metrics.New()

mux.Handle("/", rec.Middleware()(app))
mux.Handle("/metrics", rec.Handler())
```

Construct the `Recorder` **once** at wiring time and share it. One per request would reset the
counters it exists to accumulate.

### Answer liveness and readiness probes

[`health`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/health) runs dependency
probes concurrently and reports the result.

```go
mux.Handle("/healthz", health.Handler()) // liveness: is the process up?
mux.Handle("/readyz", health.Handler(    // readiness: can it take work?
    health.Check{
        Name:  "worker-pool",
        Probe: func(ctx context.Context) error { return pool.Submit(ctx, func(context.Context) {}) },
    },
))
```

**Liveness and readiness are different questions.** Liveness with dependency checks makes an
orchestrator restart a healthy process because a database blipped. A readiness probe that returns
`nil` documents your wiring and verifies nothing — exercise the real admission path, as above.

The response names *which* check failed and never *why*, so an unauthenticated caller learns nothing
about your infrastructure.

### Add context to an error

[`errx`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/errx) adds a message and,
**only when you ask**, a captured stack — staying interoperable with `errors.Is`/`As`/`Unwrap`.

```go
if err != nil {
    return errx.Wrap(err, "loading user") // "loading user: not found"
}

err = errx.WithStack(err)          // capture once, at the boundary where it matters
frames := errx.Frames(err)         // resolved lazily, only if something reads them
```

`Wrap` never touches the runtime, so wrapping stays cheap on paths where errors are routine.
`Wrap(nil, "…")` returns `nil`, so `return errx.Wrap(f(), "…")` is correct on the success path too.

---

## Configuration

### Load configuration from a file

[`config`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/config) reads JSON or
YAML into your struct, expands `${VAR}` references, and can validate the result in one step.

```go
cfg, err := config.Load[Config]("config.yaml", config.WithStructValidation())
```

`${VAR}` expansion is how a secret reaches the process without being committed. Use
`config.WithoutEnvExpansion()` for a file that legitimately contains `$`.

### Read environment variables safely

[`env`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/env) returns the fallback
for a variable that is unset, empty **or malformed** — so a typo degrades one setting instead of
failing the process at startup.

```go
addr := env.GetDefault("SERVICE_ADDR", ":8080")
workers := env.GetInt("SERVICE_WORKERS", 4)
debug := env.GetBool("SERVICE_DEBUG", false)
grace := env.GetDuration("SERVICE_SHUTDOWN_GRACE", 15*time.Second)
```

### Validate a struct

[`validator`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/validator) checks
rules declared in struct tags and **aggregates every failure** rather than stopping at the first.

```go
type Signup struct {
    Email    string `validate:"required,email"`
    Password string `validate:"required,min=12"`
    Plan     string `validate:"oneof=free pro"`
}

if err := validator.Struct(input); err != nil {
    // a ValidationErrors holding one entry per failed field
}
```

---

## Data

### Cache expensive lookups

[`cache`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/cache) is a generic
in-memory cache with TTL expiry and a background sweeper. It is sharded internally, so a write does
not lock the whole keyspace.

```go
c := cache.New[string, Session](5 * time.Minute)
defer c.Close()

c.Set("tok-1", session)

if s, ok := c.Get("tok-1"); ok {
    // hit
}
```

`Get` returns comma-ok, so **a stored zero value is a hit, not a miss** — the reason to prefer it
over an error sentinel. Always `Close()`: the sweeper is a goroutine.

### Run a function in a SQL transaction

[`db`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/db) commits on success, rolls
back on error, and — the case a hand-written `defer` usually gets wrong — rolls back and re-panics on
panic.

```go
err := db.Transaction(ctx, sqlDB, func(tx *sql.Tx) error {
    if _, err := tx.ExecContext(ctx, "..."); err != nil {
        return err // rolled back
    }
    return nil // committed
})
```

### Reuse buffers on a hot path

[`syncpool`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/syncpool) pools
`*bytes.Buffer` values to relieve GC pressure on serialization and string building.

```go
pool := syncpool.NewBufferPool()

buf := pool.Get()
defer pool.Put(buf) // reset and returned; oversized buffers are dropped, not retained
```

---

## Security

### Hash and verify passwords

[`hash`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/hash) wraps bcrypt at a
work factor this module owns rather than inherits from a dependency.

```go
stored, err := hash.HashPassword(plaintext)   // default cost 12

if err := hash.CheckPassword(attempt, stored); err != nil {
    // wrong password, or a malformed hash
}
```

**Upgrade old hashes on login**, which is the only moment you legitimately hold the plaintext:

```go
if cost, err := hash.Cost(stored); err == nil && cost < 12 {
    if rehashed, err := hash.HashPassword(attempt); err == nil {
        _ = save(rehashed)
    }
}
```

Hashing is deliberately expensive — roughly 4 logins per second per core at the default cost. Put
`ratelimit` in front of your login endpoint.

---

## Lifecycle

### Shut down in the right order

[`lifecycle`](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2/pkg/lifecycle) runs
shutdown hooks in **reverse registration order** — stop accepting traffic first, close what it
depended on last — bounded by a deadline you choose.

```go
lifecycle.Register(pool.Close)    // runs second
lifecycle.Register(srv.Shutdown)  // runs first

lifecycle.WaitForSignals(15*time.Second, os.Interrupt, syscall.SIGTERM)
```

**The deadline is measured from the signal**, not from the call, so a process that ran for a day
still gets its full grace period. `0` means no deadline — the right choice under systemd or
Kubernetes, where a second number here just duplicates a grace period configured one level up and
the shorter one silently wins.

`lifecycle.Trigger()` starts the same sequence programmatically, from an admin endpoint or a
supervisor command.

---

## Where to go next

- **[pkg.go.dev](https://pkg.go.dev/github.com/danielPoloWork/egl-utils-go/v2)** — the full reference
  for every package, with runnable examples.
- **[`examples/service`](../../examples/service)** — these pieces assembled into one HTTP service,
  with the composition decisions explained where they are made.
- **[`docs/adr/`](../adr/)** — why each package behaves the way it does.
- **Missing something?** [Open a discussion](https://github.com/danielPoloWork/egl-utils-go/discussions)
  and say what you need, what you do instead today, and what that costs — see
  [`CONTRIBUTING.md` §7](../../CONTRIBUTING.md).
