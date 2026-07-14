# ADR-0010: circuitbreaker design — lazy timerless transitions, generation-guarded accounting

- **Status:** Accepted
- **Date:** 2026-07-14
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 6, §5 (circuitbreaker API), §6; ROADMAP 3.1; ADR-0005
  (loud-by-default idiom, functional options); ADR-0009 (surface discipline); patterns
  catalogue (Circuit Breaker)

## Context

Feature 6 commits to "circuit breaker guarding outbound HTTP calls (closed/open/half-open
states)", with the API fixed at intake: `New(opts ...Option) *Breaker;
(*Breaker).Do(ctx, func() error) error; ErrOpen`. The roadmap tags 3.1 the hardest
resilience piece: a state machine mutated concurrently by every guarded call, with the
half-open probe-admission problem on top. Within that frozen surface, five decisions are
not derivable from the spec: how failures are accounted in the closed state, how probes
are admitted in half-open, what drives the time-based open → half-open transition, what
happens to a call whose outcome arrives after the state that admitted it is gone, and
what counts as a failure at all.

## Decision

**Consecutive-failure accounting.** In the closed state the breaker counts consecutive
failures in completion order; a success resets the count, and reaching the failure
threshold (`WithFailureThreshold`, default 5) trips the breaker open. No sliding window,
no failure rate: consecutive counting is deterministic, allocation-free, and
property-testable, and richer accounting can arrive later as new options without touching
the frozen surface.

**Lazy, timerless transitions.** The breaker owns no goroutines and no timers. Tripping
open records the trip time; the open → half-open transition is evaluated lazily inside
the next admission attempt, by comparing the injected clock against the open timeout
(`WithOpenTimeout`, default 30s, admission at exactly the boundary). Every state change
therefore happens under `Do`, under one mutex — there is nothing to leak (the module's
zero-goroutine-leak NFR holds trivially) and nothing to stop.

**Probe budget = success threshold.** Half-open admits a call while
`successes + inFlight < successThreshold` (`WithSuccessThreshold`, default 1): never more
trial traffic than the successes still needed to close. One knob instead of two
(resilience4j-style `permittedNumberOfCalls` plus a success criterion), with a crisp
invariant — `successes + inFlight ≤ successThreshold`, so when the threshold is reached
the in-flight count is provably zero. Any probe failure reopens the breaker and restarts
the full cool-down; excess calls are rejected with the same `ErrOpen` as the open state
(spec §5 has one sentinel).

**Generation-guarded outcome accounting.** Every state transition bumps a generation
counter, and an outcome is recorded only against the generation that admitted the call.
A slow call that completes after its episode ended — a closed-state call finishing after
the breaker tripped and recovered, or an orphaned probe finishing after a sibling probe
reopened the breaker — is discarded entirely (its caller still receives its own result).
This is the concurrency-correctness core: without it, stale outcomes corrupt the counters
of a state they never ran under.

**Failure classification is nil/non-nil.** `Do` never inspects the error value: nil is a
success, non-nil a failure. Two carve-outs follow the module's contracts: a call whose
context is already done returns `ctx.Err()` uncounted without running (caller
cancellation says nothing about the dependency's health — spec §5's cancellation rule),
and a panicking call counts as a failure and propagates untouched (loud-by-default,
ADR-0005 lineage — containment is never silent).

The public surface is exactly spec §5 plus the three threshold options; the clock is an
unexported field, injected only by the package's own deterministic tests.

## Alternatives Considered

- **Sliding-window / failure-rate accounting** — smoother tripping under mixed traffic,
  but adds window-size and minimum-call knobs, per-call bookkeeping, and clock coupling on
  the hot path. Rejected for v0; additive options can supersede this without a breaking
  change.
- **A separate half-open probe cap (`WithMaxProbes`)** — a second knob whose safe value is
  almost always the success threshold; splitting them invites `maxProbes >
  successThreshold` misconfigurations that send excess trial traffic. Rejected.
- **Timer-driven transitions (`time.AfterFunc` open → half-open)** — moves a state change
  outside any call, and makes the breaker own a timer lifecycle that must be stopped and
  leak-checked. Rejected; lazy evaluation keeps the machine passive and inert when idle.
- **An error-classifier option (exempt e.g. `context.Canceled` returned by fn)** — a real
  need in services that cancel aggressively, but it expands the decision surface beyond
  the spec's contract. Deferred: add via a spec amendment when a consumer needs it.
- **Exporting `State()` / metrics hooks** — observability is useful but outside spec §5;
  same discipline as ADR-0009's rejected `TryAcquire` re-export.

## Consequences

- The breaker is a passive mutex-guarded value: no goroutines, no timers, safe for
  concurrent use, `goleak`-clean by construction; `go test -race` is the operative gate.
- "Consecutive" is defined in completion order — under concurrency the trip point depends
  on the order outcomes land, which is inherent, documented, and covered by the race test.
- After any transition, outcomes of straddling calls are discarded even when they are
  successes; each episode's counters stay self-consistent at the cost of ignoring stale
  evidence.
- Recovery traffic is bounded: at most `successThreshold` concurrent probes ever reach a
  struggling dependency.
- Verification: deterministic fake-clock tests pin the timeout boundary and both
  stale-outcome paths; a `rapid` property drives the breaker against a sequential
  reference model of this contract.
- Catalogued as **Circuit Breaker** (in-taxonomy, Cloud/Distributed), first resilience
  pattern in the module.

## References

- `docs/specs/01_spec_utils.md` §2.6, §5, §6.
- ADR-0005 (loud-by-default, functional options), ADR-0009 (surface discipline).
- `docs/patterns/design-patterns.md` — Circuit Breaker (Cloud/Distributed Systems).
