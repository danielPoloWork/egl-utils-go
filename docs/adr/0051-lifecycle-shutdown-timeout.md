# ADR-0051: a shutdown deadline in `WaitForSignals` — explicit, not invented, and 0 still means none

- **Status:** Accepted
- **Date:** 2026-07-29
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** [ADR-0025](0025-lifecycle-shutdown-design.md) (the lifecycle design — superseded on
  "no hidden timeout" and on the rejected default-timeout alternative; every other decision, and the
  *reasoning* behind this one, is preserved), ADR-0005 (loud-at-wiring panics), ADR-0030 §2 ledger
  item 21, ADR-0045 (the `/v2` boundary), ADR-0048 (the sibling question of whether shutdown takes a
  bound at all); spec §2 feature 21, §5; ROADMAP 13.7

## Context

Ledger item 21 changes `WaitForSignals(sigs ...os.Signal)` to
`WaitForSignals(timeout time.Duration, sigs ...os.Signal)`. Spec v2 is specific about what the
parameter is for — *"hooks receive ctx with the bounded timeout"* — and its own example passes
`10*time.Second`.

This is the one Milestone 13 item that overturns a deviation the module took **deliberately and
documented as such**. ADR-0025 decided "no hidden timeout" and separately rejected "a default
shutdown timeout in `WaitForSignals`". So the ROADMAP's instruction for this item was to overturn
that reasoning *explicitly, not silently*.

Reading what ADR-0025 actually argued is what makes this tractable, and its objections are narrower
than the heading suggests. It refused a deadline that was **hidden**, **library-invented** and would
**silently truncate** what the operator configured:

> a library-invented deadline would silently truncate shutdowns that the operator configured the
> platform to allow

and, in the rejected alternatives:

> **A default shutdown timeout** … invents a deadline the operator didn't set and the platform
> already enforces one level up.

Every one of those adjectives — hidden, invented, default, silent — is about a deadline the *caller
did not choose*. A mandatory first parameter is the opposite of all of them.

Note also how the gap analysis frames this row, because it is the reverse of the last three items.
In 13.4 and 13.5 the gap column named only the signature, which is what licensed keeping `ctx` on
`Close` and `topic` on `Subscribe`. Here it explicitly lists **"🟠 bounded-deadline philosophy"** as a
gap in its own right. The same tie-breaker that protected those decisions points, here, at making the
change.

## Decision

**`WaitForSignals(timeout time.Duration, sigs ...os.Signal)`.** On waking, it derives a deadline
context and passes it to `Shutdown`, so the timeout reaches the hooks as their context's deadline.

**ADR-0025's reasoning survives; only its conclusion changes.** A library must not invent a deadline
the operator did not set — that was right and still is. What changes is that the deadline is no
longer invented: it is a required parameter, the first thing in the call, impossible to acquire by
accident. There is no default to inherit and nothing hidden to discover.

**`timeout == 0` imposes no deadline.** Hooks get a background context and the platform's kill
escalation (systemd's `TimeoutStopSec`, Kubernetes' grace period, then SIGKILL) is the only bound —
byte-for-byte v1's behaviour, now stated at the call site instead of being the only option.

This is the half that keeps ADR-0025's *substantive* claim alive rather than merely acknowledging it.
Under an orchestrator that already enforces a grace period, a second number in the application would
be a duplicate free to drift out of step with the first, and the shorter of the two silently wins.
Forcing every caller to invent one would have overridden the operator in exactly the case ADR-0025
identified correctly. So the escape hatch is not a compatibility shim — it is the recommended posture
for that deployment, and the godoc says so.

**A negative timeout panics** (ADR-0005). It cannot mean anything, and the alternative — clamping it
to zero — would present a call that reads as bounded and behaves as unbounded.

**The deadline is measured from the signal, not from the call.** `WaitForSignals` blocks for the
process's entire lifetime, so deriving the context before the wait would spend the whole budget
before shutdown began: a service up for a day would get no grace period at all. The context is
derived after the `select` returns. This is pinned by a test, and the test was verified by
deliberately moving the derivation earlier.

**The timeout bounds the sequence, not each hook.** It is one context passed to `Shutdown`, so hooks
share the budget. Per-hook deadlines were already deferred as additive by ADR-0025 and stay deferred.

**Nothing about hook semantics changes**, and this is where the bound's honesty matters: the deadline
is **cooperative**. A hook that honours its context winds up early; a hook that ignores it runs to
completion regardless. An expired deadline does **not** skip the remaining hooks — ADR-0025 decided
that every hook runs and errors are joined, and that is untouched, so a slow first hook cannot
strand the resources behind it. What the timeout buys is a bound on hooks that *can* be bounded; it
is not a kill switch, and only the platform can be one.

**`WaitForSignals` still returns nothing.** A timed-out shutdown surfaces the way any other shutdown
error does — logged at Error on `slog.Default`. Adding a return value later is source-compatible for
a statement call, so it is additive in practice and free to defer (the rule 13.2 used for
`Frame.String()` and 13.6 for `WriteTo`).

## Alternatives Considered

- **Require a positive timeout; make 0 or negative a panic.** Simplest to reason about — one code
  path, and no way to wedge shutdown indefinitely. Rejected: it overrides the operator in the case
  ADR-0025 got right, and forces a number that duplicates a platform grace period configured one
  level up.
- **Keep `WaitForSignals(sigs...)` and add `WaitForSignalsTimeout(timeout, sigs...)`.** Most explicit
  of all, with no magic value. Rejected on 13.4's grounds: two entry points for one lifecycle is the
  ambiguity these renames exist to remove, and the spec names one function.
- **Keep the v1 signature and leave bounding to `Shutdown` with a deadline context.** The status quo,
  and still available. Rejected because it makes the common case — bound the shutdown a signal
  starts — reachable only by not using the signal helper at all, which is the ergonomic gap the
  ledger recorded.
- **Have an expired deadline abandon the remaining hooks.** Rejected: it would convert a
  slow-hook problem into a resource-leak problem, and it contradicts ADR-0025's run-every-hook
  decision, which this ADR does not reopen.
- **Force-exit on deadline expiry** (`os.Exit` once the budget is gone). Rejected for the reason
  ADR-0025 already gave about force-exit on a second signal: `os.Exit` from library code bypasses
  every remaining hook. The platform owns process termination.

## Consequences

- **Breaking, and mechanical:** `lifecycle.WaitForSignals(os.Interrupt, syscall.SIGTERM)` →
  `lifecycle.WaitForSignals(10*time.Second, os.Interrupt, syscall.SIGTERM)`, or **`0` for exactly the
  old behaviour**. It fails at compile time, and `0` makes a mechanical migration available to
  anyone who does not want to make a decision while upgrading.
- **ADR-0025 is superseded on two points and nothing else** — the "no hidden timeout" decision and
  the rejected default-timeout alternative. LIFO ordering, sequential hooks, run-every-hook with
  `errors.Join`, exactly-once convergent `Shutdown`, the loud panics, the zero-owned-goroutines
  property, and the singleton-with-seams testability pattern all stand.
- **`Trigger()`-initiated shutdown is bounded too**, since the budget applies to whatever wakes the
  wait. That follows from the design rather than being a separate feature.
- **No new dependency, no new pattern, no measurable cost.** The only added work per shutdown is one
  `context.WithTimeout`, on a path that runs once per process, so **no benchmark report** — a measured
  claim needs a finding, not an opportunity (the 13.3/13.4 rule).
- **100% statement coverage retained**, with five new tests: the deadline reaches the hooks, `0`
  leaves them unbounded, a negative timeout panics, an expired deadline releases a cooperative hook
  *and* the remaining hooks still run, and the budget starts at the signal rather than at the call.
  The last was verified by deliberate violation — moving the derivation before the `select` fails it.
- **Surface unchanged at 141 identifiers**: the signature changed, the identifier did not.

## References

- `pkg/lifecycle/lifecycle.go`, `pkg/lifecycle/lifecycle_test.go`.
- ADR-0025 — the design this amends, and the source of the reasoning it preserves.
- ADR-0030 §2 — the ledger, item 21 discharged; `docs/specs/02_spec_v2_gap_analysis.md` row 21.
