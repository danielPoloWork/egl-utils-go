# 2026-08-08 — 14.10: the ledger that would have proposed work already shipped

Milestone 14 reaches **10/12**. This is the item that decides what Milestone 15 is, and the reason
the whole milestone adds no exported identifier: the ~20 deferred capabilities scattered through the
ADRs were never a to-do list, and turning them into one would have contradicted every argument that
deferred them.

## What shipped

- **[ADR-0057](../../../adr/0057-additive-capability-ledger.md)** — the additive-capability ledger.
  §A: 49 open public-surface capabilities from 26 ADRs. §B: 7 internal/process deferrals. §C: 11
  discharged. §D: 2 reclassified as breaking.
- **A trigger on every entry** — the evidence that would schedule it, over six kinds.
- **`consistency_lint.py` check 11, `ledger-coverage`** — both directions between the canonical
  `Deferred, additive:` marker and the ledger.
- **The coupled documents**, in the same PR because 14.9 made that a rule: `CONTRIBUTING.md` §7,
  the ADR template, `AGENTS.md` §7, and ADR-0056 §(d) amended from ten checks to eleven.

## The roadmap line was wrong twice, and the second one mattered

It said: *"Sixteen ADRs each end with a 'Deferred, additive' line — roughly twenty capabilities."*

**Sixteen and twenty are both low.** Counting distinct capabilities rather than lines, and following
restatements across superseding ADRs, §A holds **49 capabilities from 26 ADRs**, plus one entry with
no ADR at all. Thirteen ADRs use the greppable marker; thirteen more say the same thing in prose no
pattern finds — `additive later`, `free to defer`, `a possible additive later`, `Deferred: add via a
spec amendment when a consumer needs it`.

That undercount is harmless. The second finding is not.

**Eleven of these deferrals had already been built, and not one of the deferring ADRs had been
updated to say so.** ADR-0021 deferred `cache` sharding; 10.11 built it under ADR-0038. ADR-0024
deferred a configurable cost and a cost-upgrade helper; 10.5 shipped both as `HashPasswordCost` and
`Cost`. ADR-0029 deferred "a bespoke frame type"; `errx.Frame` has existed since 13.2.

So the obvious way to build this ledger — grep for `deferred`, transcribe what comes back — **would
have published, as future work, capability the module already exports.** A register of what to
consider building, listing eleven things already built, on its first day.

What prevented it was checking each entry against the **source**, not against the ADR that deferred
it. `SetWithTTL` is genuinely absent from `pkg/cache`; `HashPasswordCost` is genuinely present in
`pkg/hash`. The deferring ADR is evidence about the past and says nothing about now.

The same pass produced §D. Two entries everyone had been calling additive are not: making `*Pool`
satisfy `io.Closer` needs a `Close` without a context, which ADR-0048 already recorded as an accepted
cost, and making argon2id the *default* is the bcrypt-cost decision again with a different constant.
Both belong to a future major.

## The trigger is the item

ADR-0030 §2 proved the table shape works — it made a major schedulable instead of arguable, and it is
empty today because it was discharged, not abandoned. But its entries were *already decided*:
breaking changes the spec had asked for, waiting on a boundary. Every entry here is the opposite, a
capability argued against on the merits by its own ADR.

Copy §2's shape without that difference and you get a backlog — twenty-odd things "to do eventually",
which invites precisely the speculative surface growth each of those ADRs refused.

So a trigger is defined as **falsifiable evidence**: what must be observed, by whom, before the entry
is scheduled. Never "when it becomes important". The six kinds are recorded because the kind names
who is expected to report it, and one consequence is worth stating plainly:

**Exactly one trigger in §A has fired.** `env` has `GetDefault`, `GetInt`, `GetBool` and
`GetDuration` and no float getter, which 14.5 found by *writing* `examples/service` — the example
converts an `int` at the call site to reach `ratelimit.NewLimiter(float64, int)`. Composing the
module surfaced a gap that reading its surface had not.

A milestone built from §A today would therefore be one entry long. That is not a disappointment; it
is what "wait for a consumer" looks like in a library whose consumers are still arriving, and it is
the ledger doing its job rather than failing at it.

That entry is deliberately **not** implemented in this PR. Discharging an entry in the pull request
that creates the register would make the register look like a to-do list on day one.

## Two tables, and the rule that keeps entries honest

§A and §B are separate because a merged trigger column would mean two incompatible things: demand
from outside for a surface capability, and a repository event for an internal one. The ambiguity runs
the wrong way — one table would license firing a §A trigger by wanting to.

And Decision 4: an entry must name the shape that makes it additive, or it is not an entry.
"argon2id" is not one; "an additive `HashPasswordArgon2` alongside bcrypt" is; "argon2id becomes what
`HashPassword` uses" is a §2 entry. That test is what produced §D rather than a wrong §A row.

## Gating prose, and saying what the gate cannot see

A ledger that is only prose drifts exactly like the eleven §C entries did. So `consistency_lint.py`
gained `ledger-coverage`, asserting both directions between the marker and ADR-0057.

Verified two ways, because 14.7 taught that a green run is not evidence. Deliberate violation in each
direction — an ADR gaining the marker with no ledger row, and a ledger row citing ADR-0099 — and
**printing what the check sees**: 13 marked ADRs against 36 cited, neither set empty. That second
step is the one that matters. `workflow-permissions` shipped green in 14.7 while matching nothing at
all, and an empty input set passes every assertion you can write over it.

The blind spot is written into the ADR rather than left to be found. The marker covers 13 of the 26
ADRs; the rest cannot be matched without false positives, because `a deferred recover()`, "Go's
`defer` intuition" and "deferring a nil-pointer dereference" all appear in these documents and none
of them is a deferral. Retrofitting the marker into thirteen accepted records to satisfy a regex was
rejected — that edits history to please a checker, and §A is complete by census regardless. The
marker's job is to catch the **next** deferral, where it costs nothing.

## Where this leaves the project

Milestone 14 is **10/12**. Next is **14.11** — repository metadata, the doc-site decision, and the
`v0.1.0` Release backfill — then **14.12**, the v2.0.1 release that puts the examples on pkg.go.dev.

And the question this item existed to answer, answered honestly: **Milestone 15 is not yet
determinable from the ledger.** One fired trigger is not a milestone. The ledger's value today is
that this is now a fact rather than an impression, and that the next consumer report has somewhere to
land.
