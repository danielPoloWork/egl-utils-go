# 2026-07-29 — 13.7: overturning a deviation by reading what it actually said

**`lifecycle.WaitForSignals` now takes a shutdown timeout as its first argument.** ADR-0051
supersedes exactly two points of ADR-0025 — the "no hidden timeout" decision and the rejected
default-timeout alternative — and nothing else. Ledger item 21 discharged.

This is the only Milestone 13 item that overturns something the module decided **deliberately and
documented as a deviation**, which is why the ROADMAP's instruction was to overturn it *explicitly,
not silently*.

## The whole item was a reading-comprehension problem

ADR-0025's title contains the words "no hidden timeout", and its Alternatives Considered rejects "a
default shutdown timeout in `WaitForSignals`". Read as a headline, 13.7 contradicts it and someone has
to lose.

Read as written, it does not. The objections are:

> a library-invented deadline would **silently truncate** shutdowns that the operator configured the
> platform to allow

> **A default shutdown timeout** … **invents a deadline the operator didn't set** and the platform
> already enforces one level up.

*Hidden. Invented. Default. Silent.* Every adjective describes a deadline **the caller did not
choose**. A mandatory first parameter — the first thing in the call, impossible to acquire by
accident, with no default to inherit — is none of those things.

So the conclusion reverses and **the reasoning is preserved**. Better than preserved: it is what
shaped the replacement, and it decided the one open question.

## The question that reasoning decided

`timeout == 0` imposes **no deadline** — hooks get a background context, exactly v1's behaviour.

That was the item's only real decision, and it was the maintainer's: does v2 keep a way to say "no
deadline", or must every caller pass a number? Forcing a number is simpler — one code path, and no
way to wedge shutdown indefinitely. It also **overrides the operator in precisely the case ADR-0025
got right.** Under systemd or Kubernetes the grace period is already configured one level up; a second
number in the application duplicates it, the two are free to drift apart, and the shorter silently
wins.

So `0` is not a compatibility shim. It is the recommended posture for that deployment, documented as
such in the godoc, which means ADR-0025's substantive claim stays alive as an option rather than being
deleted along with its conclusion.

Two alternatives fell out of earlier items: requiring a positive timeout (rejected above), and adding
a separate `WaitForSignalsTimeout` — rejected on **13.4's** grounds, since two entry points for one
lifecycle is exactly the ambiguity these renames exist to remove.

## The tie-breaker ran the other way, for the first time

Worth recording, because four consecutive items have now turned on the same table and this one turned
the opposite way.

In 13.4 the gap analysis wrote `Close() error` and I kept `ctx`; in 13.5 it wrote
`Subscribe(ctx, filter)` and I kept `topic`. Both times the licence was that **the gap column named
only the signature** — so the terse target was shorthand, not a decision.

Row 21's gap column reads **"🔴 signature · 🟡 `Trigger()` · 🟠 bounded-deadline philosophy"**. It names
the philosophy as a gap in its own right. The same rule that protected the last two decisions points,
here, at making the change — which is the test of whether it was a rule or a rationalisation.

## The detail with real consequences

**The deadline is derived after the wait returns, not before it.**

`WaitForSignals` blocks for the entire life of the process. Deriving the context before the `select`
would start the budget at wiring time, so a service up for a day would reach shutdown with the whole
grace period already spent — and it would look correct in every fast test, because in a test the
signal arrives immediately.

Pinned by `TestDeadlineStartsWhenTheSignalArrives`, which waits inside the select for a quarter of the
budget before `Trigger()` wakes it and then asserts the hook still sees most of it. **Verified by
deliberate violation:** moving the derivation above the `select` fails it with "1.4993391s is not
greater than 1.6s" — the 500 ms wait had eaten the budget, exactly as predicted.

## What the timeout does not do

The bound is **cooperative**, and saying so plainly is more useful than implying a guarantee:

- A hook that honours its context winds up early.
- A hook that ignores it **runs to completion anyway.** That is the hook's choice, per ADR-0025.
- An expired deadline **does not skip the remaining hooks.** ADR-0025 decided every hook runs and
  errors are joined; that is untouched, so a slow first hook cannot strand the resources behind it.

The timeout bounds what *can* be bounded. Only the platform can be a kill switch, and force-exit on
expiry was rejected for the reason ADR-0025 already gave about force-exit on a second signal: `os.Exit`
from library code bypasses every remaining hook.

The timeout is also one budget for the **sequence**, not per hook — one context passed to `Shutdown`.
Per-hook deadlines were already deferred as additive by ADR-0025 and stay deferred.

## Small notes

`WaitForSignals` still returns nothing. A timed-out shutdown surfaces the way every other shutdown
error does, logged at Error on `slog.Default`. Adding a return value later is **source-compatible for
a statement call**, so it is additive in practice and free to defer — the rule 13.2 used for
`Frame.String()` and 13.6 for `WriteTo`.

No benchmark report: one `context.WithTimeout` on a path that runs once per process is not a finding,
and publishing a number for it would be manufacturing one (the 13.3/13.4 rule).

Surface stays at **141 identifiers** — the signature changed, the identifier did not, which is the
first item this milestone where `spec_api_lint` could not have caught a mistake. §5's text carries the
semantics instead.

Coverage stayed 100% with five new tests: the deadline reaches the hooks; `0` leaves them unbounded; a
negative timeout panics; an expired deadline releases a cooperative hook *and* the later hooks still
run; and the budget starts at the signal.

## State

Milestone 13 is **7 of 10**. Next is 13.8 — bcrypt's default cost 10 → 12 — which is
**security-relevant**, so it needs its own ADR with an auditor sign-off line, a compliance-control
update and threat-model rows (§7), and it changes a default that 10.5 measured at roughly 4× login
latency. Then 13.9 (contrib → `/v2`, **only after the `v2.0.0` tag**, ADR-0040) and 13.10 (the
release).

**Still carried, still not fixed:** `orchestrator/project.yaml` describes the v1 API — and now
`WaitForSignals(sig ...os.Signal)` is one more line of that drift. Unchanged reasoning: it wants one
scoped sweep, not five partial ones.
