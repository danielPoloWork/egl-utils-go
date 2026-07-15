# ADR-0026: health.Handler design — concurrent probes, 200/503, status-only body (no error leak)

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role
- **Related:** spec §2 feature 22, §5 (`Handler(checks ...Check) http.Handler`, `Check{Name, Probe
  func(ctx) error}`); ROADMAP 9.2; ADR-0005 (loud-by-default); ADR-0013 (HTTP edge); ADR-0014/0016
  (path-only / no-detail-to-client information-disclosure posture); threat model (public HTTP edge)

## Context

Feature 22 is a preconfigured health-check endpoint that "analyses the state of active connections
(DB, Redis)", frozen at intake as `Handler(checks ...Check) http.Handler` with `Check{Name string,
Probe func(ctx) error}`. It is a new HTTP surface, typically unauthenticated and often internet- or
cluster-reachable (load balancers, Kubernetes liveness/readiness), so its **response content is a
security decision** under the enterprise posture. The frozen `Check` has no timeout or config field,
which constrains how "per-check timeout" (the ROADMAP note) can be expressed.

## Decision

**Run every probe concurrently on each request, with the request context.** Probes are independent
I/O checks; running them in parallel makes endpoint latency the slowest probe, not their sum. Each
receives `r.Context()`, so cancellation/deadline propagates; `run` waits for all probes (a
`WaitGroup`), owning **no goroutine past the response** (spec §1) — the price is that a probe which
ignores its context can stall the handler, which the godoc names as the probe's responsibility.

**200 when all pass, 503 when any fails.** The standard readiness contract: `200 OK` /
`503 Service Unavailable`. No checks at all is healthy (200) — a bare liveness endpoint.

**Status-only JSON body — the probe's error is never written to the response.** The body is
`{"status": "ok"|"unavailable", "checks": {name: "ok"|"fail"}}`. The failing check's **name** is
enough for an operator to locate the problem; the **error text is deliberately omitted**, because a
probe error routinely carries internal detail (connection strings, hostnames, backend versions) and
this endpoint is commonly unauthenticated — echoing it is an information-disclosure leak into
whoever can reach `/healthz`. This is the same "no internal detail to the client" posture as Logger
(path-only, ADR-0014) and Recoverer (no stack, ADR-0016). A consumer wanting the error logs it
*inside* the probe, where it has a logger and controls the sink.

**Loud on wiring errors** (ADR-0005): an empty check name, a nil `Probe`, or a duplicate name panics
at `Handler` construction — a misconfigured health endpoint should fail at startup, not silently
serve a wrong picture. The checks slice is copied so later caller mutation cannot affect the handler.

**Deterministic output.** `encoding/json` marshals map keys sorted, so the `checks` object is stable
across requests (friendly to diffing and tests).

## Alternatives Considered

- **Include the probe error in the body** — best debuggability, but leaks internals to an
  unauthenticated endpoint; rejected for status-only, with in-probe logging as the escape hatch. An
  opt-in "verbose" mode is a possible additive later, off by default.
- **Sequential probes** — simpler, but endpoint latency becomes the sum of all probes; rejected for
  concurrent execution.
- **A per-check timeout field / `Handler` options** — the ROADMAP's "per-check timeouts", but the
  spec froze `Check{Name, Probe}` and `Handler(checks ...Check)` with nowhere to put one. Per-check
  bounds are therefore the probe's own concern (honor `ctx`), and an overall bound composes via
  `http.TimeoutHandler` or the server's timeouts. `Check.Timeout` is a clean additive extension,
  deferred.
- **Restrict to GET/HEAD (405 otherwise)** — stricter, but health probers only GET/HEAD and method
  policing adds no safety here; rejected for simplicity (any method is answered).
- **Tolerate duplicate names (last-wins) / skip nil probes** — quiet, and quietly wrong (a check
  silently missing from the report). Rejected for the loud panic.

## Consequences

- A ready-to-mount readiness endpoint: `mux.Handle("/healthz", health.Handler(db, redis))`, 200/503,
  concurrent probes, deterministic JSON.
- The response cannot leak probe error detail; the public HTTP edge gains health-specific
  Information-disclosure and DoS rows in the threat model (the DoS surface — unauthenticated,
  probe-triggering — is bounded by concurrency and delegated to the consumer's auth/rate limiting).
- No new dependency; no new pattern. Milestone 9 continues (metrics, syncpool, errors remain).
- Deferred, additive: a verbose/error-exposing mode, `Check.Timeout`, method restriction.

## References

- `docs/specs/01_spec_utils.md` §2.22, §5.
- ADR-0005 (loud-by-default), ADR-0013 (HTTP edge), ADR-0014/0016 (no-detail-to-client posture).
- `docs/security/threat-model.md` — public HTTP edge, health rows.
- `net/http`, `encoding/json` (sorted map keys), `http.TimeoutHandler` (composed overall bound).
