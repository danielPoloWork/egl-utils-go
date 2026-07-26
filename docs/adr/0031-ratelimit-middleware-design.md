# ADR-0031: `ratelimit` HTTP middleware — shed on deny, sentinel for the Allow path

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0012 (token-bucket engine), ADR-0030 (spec v2.0 reconciliation), ADR-0013/0014/0016/0017 (the middleware decorator family), ADR-0004 (dependency & import-graph budget), spec v2.0 items 8 / §3 / §5, roadmap 10.4

## Context

Spec v2.0 asks the rate limiter for "middleware ergonomics" (item 8) and pins the surface in its
§5 table as `(l) Wait(ctx)` / `Allow()` / `Middleware()`, with `ErrLimited` on the `Allow`
middleware path. ADR-0030 placed this in the additive bucket: roadmap 10.4, shipping in v1.1.0
without touching the engine ADR-0012 froze.

Three forces shape the design.

**Placement.** Spec §3 layers the module: `ratelimit` sits at L2 (services), `middleware.*` at L3
(HTTP), and arrows point downward only, so `middleware` importing `ratelimit` is legal while the
reverse is not. That argues for `middleware.RateLimit(l)`. But spec §5 spells the API as a method
*on the Limiter*, and the L2 rule constrains **intra-module** imports — L1 is the layer forbidden
anything "above stdlib/x", and `net/http` is stdlib. 10.8 will enforce this graph with depguard, so
the choice has to be one depguard can express as a rule rather than an exception.

**Deny semantics.** A limiter offers two admission modes, and an HTTP front end can be built on
either: refuse immediately (`Allow`) or park the request until a token funds (`Wait`).

**The sentinel's job.** An `http.Handler` cannot return an error, so `ErrLimited` cannot be what the
middleware "returns". Its role has to be defined, or it becomes decoration.

## Decision

`Middleware()` is a method on `*Limiter`, defined in `ratelimit/middleware.go`, returning the
house-standard decorator `func(http.Handler) http.Handler`; it admits via **`Allow`, never `Wait`**,
answering `429 Too Many Requests` with a `Retry-After` of `ceil(1/rate)` whole seconds and a body of
the generic status text; and `ErrLimited` is exported as the **canonical sentinel for the fail-fast
admission condition** — the error a consumer gating its own work on `Allow` returns so its callers
can `errors.Is` it — with the middleware expressing that same condition in the only vocabulary HTTP
has, a 429. The middleware logs nothing and owns no goroutines.

## Alternatives Considered

- **`middleware.RateLimit(l *ratelimit.Limiter)` in the `middleware` package** — the layering-purist
  reading of spec §3, and it keeps `net/http` out of an L2 package. Rejected because spec §5 names
  the method on the Limiter, and because it would make `middleware` the only package that imports a
  sibling feature package, turning a clean "L3 imports nothing internal" invariant into a graph
  10.8's depguard must carve an exception for. `net/http` is stdlib, so the L2 package takes on no
  dependency budget (ADR-0004) by importing it.
- **Blocking admission via `Wait`** — smoother client experience: a burst is delayed rather than
  refused, and no 429 is ever seen. Rejected as a denial-of-service foot-gun. Each parked request
  holds a server goroutine and its connection for the wait, so an over-budget burst converts into an
  unbounded queue and the failure mode becomes resource exhaustion instead of a clean refusal.
  Shedding load is what a limiter is *for*. A consumer that genuinely wants queueing can call `Wait`
  with a deadline context in its own handler.
- **Logging each denial** on `slog.Default`, as `Recoverer` does for panics — rejected twice over.
  A denial is a normal operating condition, not a fault (`Recoverer` logs a bug; this logs a budget
  working), and a client-triggerable log line is a log-flooding amplifier: the very traffic being
  limited would drive unbounded log volume. Denials are already observable as 429s to
  `middleware.Logger`, which is the composition this family is built for.
- **A configurable deny handler / functional options** (custom body, custom status, `OnLimited`
  hook) — rejected for now as surface beyond what the spec asks. `Middleware()` takes no arguments
  in §5; a consumer needing a custom refusal writes four lines against `Allow` and `ErrLimited`.
  Deferred and additive.
- **Per-client (per-IP, per-key) limiting inside the middleware** — the policy most consumers
  eventually want. Rejected because a fair per-key limiter needs a key-extraction policy (which
  header? trusting `X-Forwarded-For` requires knowing the proxy topology) and an eviction policy for
  idle keys, both of which are consumer decisions this package must not presume — and getting either
  wrong silently converts the limiter into a memory leak or an IP-spoofing bypass. Documented as the
  consumer's job, with the global-budget consequence stated plainly.
- **A `Retry-After` computed from the live bucket level** — `(1 - tokens)/rate` is tighter than
  `ceil(1/rate)`. Rejected: it needs the lock on the deny path, it leaks the limiter's instantaneous
  state to an untrusted client, and RFC 9110 requires whole `delta-seconds` anyway, which rounds most
  of the precision away. The constant is precomputed once per decorator instead, keeping the deny
  path lock-free beyond `Allow` itself.

## Consequences

- Additive and non-breaking: `NewLimiter`, `Allow`, and `Wait` are untouched; v1.0.0's stability
  commitment holds. `ratelimit` gains one stdlib import (`net/http`) and no module dependency.
- The admit path costs **~30 ns/op and 0 allocs/op** (`BenchmarkMiddlewareAdmit`), so the decorator
  is compatible with NFR-01's zero-allocation posture for the non-logging chain; a test asserts the
  zero-allocation property directly via `testing.AllocsPerRun` rather than trusting the benchmark.
  The deny path costs ~386 ns/op and 4 allocs (`http.Error`'s header write and body format), paid
  only by refused requests.
- One limiter bounds **total** throughput across every request the decorator admits, so a single
  heavy client can consume the whole budget. This is a documented limitation, not a defect, and it
  is recorded as a threat-model row alongside the reasoning above.
- Nothing about the limiter's configuration or level is disclosed to a client beyond a conservative
  `Retry-After` — the same no-detail-to-client discipline as `Recoverer` (ADR-0016) and
  `health.Handler` (ADR-0026).
- The Decorator catalogue entry (patterns row 9) now spans two packages: the middleware family plus
  this one. Deferred, additive: a deny hook, per-key limiting, and a configurable status/body.

## References

- Spec v2.0 item 8, §3 (layered import graph), §5 (public interface table), NFR-01 / NFR-04.
- ADR-0012 (token-bucket engine: lazy refill, reservation fairness), ADR-0030 (adoption bucket).
- ADR-0016 (`Recoverer`: no detail to the client, `slog.Default` for genuine faults), ADR-0026
  (`health.Handler`: status-only body).
- RFC 9110 §10.2.3 (`Retry-After` as `delta-seconds`), §15.5.29 (`429 Too Many Requests`).
- `BenchmarkMiddlewareAdmit` / `BenchmarkMiddlewareRefuse`, `ratelimit/middleware_test.go`.
