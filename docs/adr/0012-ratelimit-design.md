# ADR-0012: ratelimit design — hand-rolled lazy token bucket, reservation-model Wait

- **Status:** Accepted
- **Date:** 2026-07-14
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 8, §5 (ratelimit API), §6 (deterministic clocks, benchmark
  set); ROADMAP 3.3; ADR-0004 (dependency policy); ADR-0009 (wrap-vs-build precedent);
  ADR-0010/0011 (lazy timerless lineage, test seams); patterns catalogue (Rate Limiting /
  Throttling)

## Context

Feature 8 commits to a "token-bucket rate limiter built on Go timers", API frozen at
intake: `NewLimiter(rate float64, burst int) *Limiter; (*Limiter).Allow() bool;
(*Limiter).Wait(ctx) error`. §6 demands deterministic-clock tests and puts ratelimit in
the benchmark set. The open decisions: build the bucket or wrap `golang.org/x/time/rate`
(ADR-0009 wrapped `x/sync` for the semaphore); how refill is driven; how Wait queues; and
what a canceled Wait costs.

## Decision

**Build, do not wrap — because here the spec names the construction, not a wrapper.**
ADR-0009's "wrap, don't reimplement" rested on spec §2 explicitly prescribing the
`x/sync/semaphore` engine. Feature 8's text prescribes the opposite kind of thing: *"a
token-bucket rate limiter built on Go timers"* — a construction. The hand-rolled bucket
is ~60 lines of well-understood float arithmetic, keeps the resilience trio (3.1–3.3)
dependency-free, avoids another `go`-directive/floor pin dance, and gives the
deterministic-clock mandate first-class internal seams (`x/time/rate`'s `Wait` reads the
wall clock internally and cannot be pinned).

**Lazy refill, no background anything.** Tokens are a `float64` brought current on each
call from elapsed monotonic time (`tokens = min(burst, tokens + elapsed·rate)`). The
limiter owns no goroutines and no standing timers — ADR-0010's lazy lineage — so the
zero-leak NFR holds by construction; the only timer lives inside a blocked `Wait` and is
stopped when it returns. The bucket starts full: a fresh limiter admits its burst
immediately (the conventional contract; a cold-start-empty bucket punishes exactly the
first, most latency-visible requests).

**Reservation-model Wait.** A waiter reserves its token on arrival — `tokens` goes
negative as debt — and sleeps exactly `shortfall/rate` (rounded up to the nanosecond).
Lock-acquisition order is service order: each concurrent waiter's debt is one deeper, so
each sleeps exactly until its own token is funded. No wake-and-recheck loop, no
thundering herd, no starvation. A canceled `Wait` repays its reservation
(`tokens = min(burst, tokens+1)`) — a caller that gave up never costs a token; refill
accrues from wall time independently of debt, so the repayment is exact.

**Loud validation.** `rate` must be positive and finite (`+Inf`/`NaN` rejected), `burst
≥ 1` — a zero-capacity bucket could never admit anything and every `Wait` would block
forever, a trap not worth representing. Panics per the ADR-0005 idiom.

The public surface is exactly spec §5 — two methods and a constructor; the clock and
sleep seams are unexported fields, reachable only by in-package tests (ADR-0011's
pattern).

## Alternatives Considered

- **Wrap `golang.org/x/time/rate`** — vetted and API-congruent, and legal under
  ADR-0004's ring 2. Rejected on three grounds: the spec text names a construction rather
  than an engine (the inverse of the 2.5 situation); its `Wait` cannot run under an
  injected clock, weakening the §6 deterministic-clock mandate to wall-clock sleeps; and
  it re-opens the `go`-directive floor-pin problem (ADR-0009) for a package this small.
- **Channel-of-tokens with a refill goroutine** — the folk implementation; rejected
  because the standing goroutine and ticker are a leak surface and a shutdown contract
  the API has no room for (`Limiter` has no `Close`), and refill granularity becomes the
  ticker period instead of continuous.
- **Wake-and-recheck Wait (sleep, then retry `Allow`)** — simpler, but concurrent waiters
  herd on every funding instant and fairness is accidental. Rejected for the reservation
  model.
- **Integer nanosecond token accounting** — avoids float, but the spec's `rate` is
  `float64` (fractional rates are first-class), and the float path is exact for the
  test-relevant values (power-of-two rates and millisecond steps) while a rapid property
  guards the law in general. Rejected.

## Consequences

- Admission cost is a mutex plus a few float operations: measured ~25ns/op and
  zero-allocation for `Allow` (admit and deny), ~50ns/op zero-allocation for the funded
  `Wait` fast path (first report under `docs/benchmarks/`). A blocked `Wait` pays one
  timer allocation per sleep.
- Burst absorption and sustained rate are decoupled and exact: admissions over any window
  obey `admitted ≤ burst + rate·elapsed` (the token-bucket law, pinned by a rapid
  property).
- Waiters are served in arrival order; there is no fairness knob and none is needed.
- A canceled `Wait` is free; the repayment is observable (tests pin bucket-level
  restoration to the token).
- Sub-nanosecond funding intervals (rates near 1e9/s with an empty bucket) still take the
  timer path for at least 1ns — benign, but visible in benchmarks that drain a burst-1
  bucket faster than the clock ticks (documented in the benchmark report).
- Catalogued as **Rate Limiting / Throttling** (in-taxonomy, Cloud/Distributed; the
  intake's working name "Token Bucket" is the mechanism, not the catalogued pattern) —
  third and final resilience pattern, completing Milestone 3.

## References

- `docs/specs/01_spec_utils.md` §2.8, §5, §6.
- ADR-0004 (dependency rings), ADR-0009 (wrap-vs-build), ADR-0010/0011 (lazy lineage,
  unexported seams).
- `docs/benchmarks/2026-07-14-ratelimit-hot-paths.md` — the measured numbers above.
- `docs/patterns/design-patterns.md` — Rate Limiting / Throttling (Cloud/Distributed).
