# Design Patterns Catalogue

Living index of every design pattern **adopted**, **planned**, **considered and rejected**,
or **under evaluation** for `egl-utils-go`. Mandatory reading whenever a PR introduces
or removes a pattern, and updated in the same PR.

- **Rules** — [`AGENTS.md`](../../AGENTS.md) §8.
- **Canonical taxonomy** — [`design-patterns.md`](design-patterns.md). All pattern names
  used here, in ADRs, and in commit messages must match its spelling and categorisation.

## Architecture style

_No single architectural style committed at intake (typical for a library, which exposes an API
rather than an application architecture). Record one in an ADR here if that changes._


## How to use this catalogue

- **Adding a pattern** — when a PR lands one, add a row to *Implemented / Planned* as
  `Implemented`, with the ADR link and the code location (a real feature-package path at
  the repo root, per ADR-0003); a pattern decided in an ADR but not yet in code is added as `Planned`.
- **Refining** — update the row and link the new ADR.
- **Rejecting** — add it to *Rejected* with the reason; do not silently drop it.
- **Removing** — move the row to *Superseded*, link the superseding ADR, keep the history.

Status vocabulary: `Planned` (decided in an ADR, not yet landed) · `Implemented` (present
in the code, ADR `Accepted`) · `Considered` · `Rejected` · `Superseded`.

## Implemented / Planned

_Patterns named in the spec at intake are seeded below as **Planned**; each becomes
**Implemented** with its ADR and a real code location in the PR that introduces it._

| # | Pattern | Status | Problem it addresses | Code location | ADR / PR |
|---|---------|--------|----------------------|---------------|----------|
| 1 | Thread Pool | Implemented | bounded concurrency with backpressure (a.k.a. worker pool — workerpool.Pool) | [workerpool/](../../workerpool/) | [ADR-0005](../adr/0005-workerpool-design.md) |
| — | Publish-Subscribe | Planned | decoupled in-memory eventing over channels (pubsub.Broker) | _TBD_ | _spec (intake)_ |
| — | Fan-In / Fan-Out | Planned | canonical channel merge/split building blocks (fanin, fanout) | _TBD_ | _spec (intake)_ |
| — | Circuit Breaker | Planned | fail-fast protection for outbound calls (circuitbreaker.Breaker) | _TBD_ | _spec (intake)_ |
| — | Retry with Backoff + Jitter | Planned | transient-failure recovery without thundering herds (retry.Backoff) | _TBD_ | _spec (intake)_ |
| — | Token Bucket | Planned | smooth rate limiting with bursts (ratelimit.Limiter) | _TBD_ | _spec (intake)_ |
| — | Decorator | Planned | composable func(http.Handler) http.Handler middleware chain | _TBD_ | _spec (intake)_ |
| — | Object Pool | Planned | sync.Pool reuse to relieve GC pressure (syncpool.BufferPool) | _TBD_ | _spec (intake)_ |
| 2 | Functional Options | Implemented | idiomatic, forward-compatible construction for configurable components (first use: workerpool; taxonomy deviation recorded in ADR-0005) | [workerpool/options.go](../../workerpool/options.go) | [ADR-0005](../adr/0005-workerpool-design.md) |


## Rejected

_No rejections recorded yet._

| # | Pattern | Considered for | Rejected because | ADR / PR |
|---|---------|----------------|------------------|----------|
| — | —       | —              | —                | —        |

## Superseded

_No superseded patterns yet._

| # | Pattern | Superseded by | When | ADR / PR |
|---|---------|---------------|------|----------|
| — | —       | —             | —    | —        |

## Candidate patterns to consider

The taxonomy in [`design-patterns.md`](design-patterns.md) lists every pattern in scope. As
the architecture takes shape, narrow that universe to the patterns plausibly applicable to
*this* artifact and list them here by category, each with a one-line "possible application".
A candidate remains a candidate until adopted (own ADR) or explicitly rejected.

## Out-of-scope categories

Record here any taxonomy category pre-classified as not applicable to this artifact (with a
one-line reason), so the policy of explicit rejection is honoured without filling the
*Rejected* table with N/A noise.
