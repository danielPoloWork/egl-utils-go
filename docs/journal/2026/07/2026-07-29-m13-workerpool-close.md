# 2026-07-29 — 13.4: one shutdown verb, and the parameter that should not follow it

**`workerpool.Stop` becomes `Close`, `ErrPoolClosed` becomes `ErrClosed`.** ADR-0048 supersedes those
two names in ADR-0005 and nothing else. Ledger item 1 discharged; PR #74.

## What the vocabulary sweep actually found

The ROADMAP asked to check every package's shutdown name "so v2 ships one vocabulary". Run against
the v2 tree, the module has exactly three goroutine-owning shutdowns:

| Package | Before | After |
| --- | --- | --- |
| `cache` | `Close()` | unchanged |
| `pubsub` | `Close()` | unchanged |
| `workerpool` | `Stop(ctx) error` | `Close(ctx) error` |

One outlier, and the item was scoped correctly. Worth having run it anyway: the alternative was
asserting the sweep's conclusion from memory.

## Two things the brief and the ledger disagreed about

**The sentinel.** The ROADMAP line named only the method; gap-analysis item 1 names `ErrClosed` too.
Folded in — half-discharging a ledger item inside the very major that exists to empty it is how a v3
gets scheduled. Same call as 13.3's `New`-is-a-rename: when the ROADMAP's shorthand and the gap
analysis disagree, the gap analysis is the contract.

**The context, where the same rule pointed the other way.** The ledger writes the target as
`Close() error`. Taking that literally would have been a semantic change wearing a rename's clothes,
so the tie-breaker was the table's own gap column: it flags the method name and the sentinel name and
**never the parameter**. `Close() error` is the table being terse, not the table deciding. Kept
`Close(ctx) error`.

The reasoning underneath matters more than the verdict. A `Close()` with no context that waits for
caller-supplied tasks has two options, and ADR-0025 already rejected the second: wait forever, or
invent a hidden timeout. So **v2's shutdown vocabulary is uniform on the verb and deliberately not on
the signature** — and the asymmetry with `cache.Close()`/`pubsub.Close()` is principled rather than
tolerated: the pool is the only shutdown in the module that waits on work *the caller wrote*. A cache
sweeper and a broker fan-out are the module's own goroutines, bounded by construction. Uniformity is
worth having on the verb, where it removes something to remember; it is not worth having on the
parameter list, where it would remove a control the caller needs.

**The cost is stated, not discovered:** `*Pool` does not satisfy `io.Closer`. Someone will eventually
try. ADR-0048 and the CHANGELOG both say so out loud.

## The test I did not write

Coverage stayed at 100% without a new test, and that is the correct outcome rather than a gap. The
rename introduced **no new invariant** — unlike 13.3, where comma-ok created one (a stored zero is not
a miss) that genuinely needed pinning. Here `TestSubmitAfterCloseReturnsErrClosed`,
`TestCloseIsIdempotentAndConcurrent` and `TestCloseDeadlineCancelsExecutionContext` already pin every
behaviour, now under the new spelling. Adding a fourth would have been a duplicate that looked like
diligence.

`b.StopTimer()` is the one thing a careless `Stop`→`Close` sweep breaks. A `\bStop\b` boundary skips
it, which is why the sed was written that way — but the four `TestStop…` function names have no
boundary either, so they survived the first pass and needed a second, explicit one. **A word-boundary
rename has two blind spots that are the same shape: identifiers that embed the word.** The compiler
caught nothing here because both blind spots were still self-consistent — the test names compiled fine
while reading wrong. That is the sharper version of 13.3's lesson: the compiler is the authoritative
index for *call sites*, and no index at all for *names*.

## Where the project stands

Milestone 13 is **4 of 10** (13.1 layout, 13.2 errx, 13.3 cache, 13.4 workerpool). `version.go` stays
1.1.1 until 13.10. `workerpool` at 100% coverage; all four policy tools green.

Also cleaned up in 13.3's PR at the maintainer's request: ADR-0038 and ADR-0032 both cited
`cache.NewInMemory`, and the two got **different** treatments on purpose — annotated in place where
the old identifier was part of the premise (ADR-0038's frozen v1 surface), renamed where it was
incidental to the argument (ADR-0032's list of panicking constructors).

## How the next session resumes

Confirm 13.4's PR merged, re-sync `master`, **create the branch before the first edit**. Then 13.5:
the pubsub reshape — ctx-scoped subscription, `Publish(ctx) error`, `ErrSlowSubscriber` (ledger item
2). Rated `max` for a reason: it is a **redesign, not a rename**, and the first question is not code
but semantics. ADR-0006's "Publish never blocks" is the invariant everything downstream was built on,
and `Publish(ctx) error` says it can now fail or wait. Decide what that `error` *means* — and what it
does to ADR-0039's drop policies and delivered-or-reported accounting — before writing a line.
