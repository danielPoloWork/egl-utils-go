# ADR-0011: retry design — proportional jitter, hard cap, last error verbatim

- **Status:** Accepted
- **Date:** 2026-07-14
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 7, §5 (retry API), §6 (backoff bound invariants); ROADMAP
  3.2; ADR-0005 (loud-by-default idiom); ADR-0010 (deferred error classification);
  patterns catalogue (Retry with Backoff)

## Context

Feature 7 commits to "function execution with retry, exponential backoff, and random
jitter", with the API fixed at intake: `Backoff(ctx, policy Policy, fn func(ctx) error)
error — Policy{MaxAttempts, BaseDelay, MaxDelay, Jitter}`, and §6 demands property tests
for the backoff bound invariants under deterministic clocks. Inside that frozen surface
four decisions remain open: the jitter model (the spec names the `Jitter` field but not
its semantics), how `MaxDelay` interacts with jitter, what Backoff returns when the
attempt budget is spent, and how a package-level pure function gets deterministic tests
without growing the public surface.

## Decision

**Symmetric proportional jitter.** `Jitter ∈ [0, 1]` is a half-range fraction: a
pre-jitter delay `d` becomes uniform in `[d·(1−J), d·(1+J)]`. `Jitter: 0` means exactly
the deterministic exponential schedule — the field's zero value disables the feature
instead of silently choosing a different curve. The pre-jitter schedule starts at
`BaseDelay` and doubles per retry, overflow-safe.

**`MaxDelay` is a hard cap that survives jitter.** The doubling clamps at `MaxDelay` and
the jittered value is re-capped, so the documented invariant is unconditional: **no sleep
ever exceeds `MaxDelay`**, and every sleep lies in `[(1−J)·exp, min((1+J)·exp,
MaxDelay)]` for the capped exponential envelope `exp`. This is the §6 bound invariant the
rapid property pins.

**Exhaustion returns the last error verbatim.** Backoff never wraps and never inspects
errors: nil is success, non-nil schedules a retry, and when `MaxAttempts` calls (total,
counting the first — `1` means no retry) have all failed, the final error comes back
unchanged, keeping `errors.Is/As` chains pointing at the real cause. Context ends surface
`ctx.Err()` (spec §5's cancellation rule): before the first call without running fn, and
during any between-attempt sleep. fn receives ctx unchanged and owns its own deadline
handling; a caller that must not retry permanent errors wraps fn (the classifier option
stays deferred, as in ADR-0010).

**Validation is loud; test seams are unexported.** An impossible policy
(`MaxAttempts ≤ 0`, negative `BaseDelay`, `MaxDelay < BaseDelay`, `Jitter` outside
`[0, 1]`) or a nil fn panics — ADR-0005's programming-error idiom. `Policy` carries two
unexported seams (`sleep`, `rand`) reachable only by in-package tests: the bound-invariant
property and the schedule tests run with recorded sleeps and chosen jitter draws, no wall
clock and no flakiness, while the public surface stays exactly the spec's function and
four fields. Jitter randomness is `math/rand/v2` with a targeted, reasoned `//nolint`
for gosec G404 — jitter spreads retry storms, not secrets, and `crypto/rand` would be
cost without benefit.

## Alternatives Considered

- **Full jitter (AWS style: `uniform(0, exp)`)** — great storm-spreading, but it ignores
  a tunable `Jitter` field (the spec has one) and halves the mean delay, surprising anyone
  reasoning from `BaseDelay`. Rejected: the spec's field demands a proportional knob.
- **Equal jitter (`exp/2 + uniform(0, exp/2)`)** — a fixed special case of the
  proportional model (J=0.5 shape); strictly less expressive than the field it would
  ignore. Rejected.
- **Letting jitter exceed `MaxDelay`** — a simpler formula, but it forfeits the one
  invariant an operator most wants from a cap. Rejected; the cap is re-applied post-jitter.
- **A wrapped exhaustion error (`retry: 3 attempts failed: %w`) or an `ErrExhausted`
  sentinel** — adds failure taxonomy the spec does not name and pushes callers through an
  unwrap hop for the common case. Rejected; the last error verbatim composes best.
- **Package-level clock/rand hooks for tests** — mutable globals break parallel tests and
  leak test plumbing into production state. Rejected in favor of unexported per-call
  seams on `Policy`.

## Consequences

- Backoff is stateless and goroutine-free (one stopped timer per sleep), trivially
  `goleak`-clean and safe for concurrent use.
- The delay envelope is fully characterized and property-tested: callers can budget worst
  cases as `min((1+J)·BaseDelay·2^(i−1), MaxDelay)` per gap.
- `BaseDelay: 0` legally means immediate retries (spin-retry is the caller's explicit
  choice, not an accident — `MaxAttempts` still bounds it).
- Error classification (e.g. stop on `context.Canceled` returned by fn) remains a spec
  amendment away, consistent with ADR-0010's deferral.
- Catalogued as **Retry with Backoff** (in-taxonomy, Cloud/Distributed), second
  resilience pattern in the module.

## References

- `docs/specs/01_spec_utils.md` §2.7, §5, §6.
- ADR-0005 (loud-by-default), ADR-0010 (deferred classifier precedent).
- `docs/patterns/design-patterns.md` — Retry with Backoff (Cloud/Distributed Systems).
- Marc Brooker, "Exponential Backoff And Jitter" (AWS Architecture Blog) — the jitter
  taxonomy weighed above.
