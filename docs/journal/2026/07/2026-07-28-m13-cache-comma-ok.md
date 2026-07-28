# 2026-07-28 — 13.3: the error channel that carried one bit

## What got done

**`cache.Get` returns `(V, bool)`, `NewInMemory` becomes `New`, `ErrNotFound` is deleted.** ADR-0047
supersedes *only* those parts of ADR-0021. Ledger item 17 discharged; Milestone 13 is 3 of 10.

The tell was that `ErrNotFound` was the **only** error `Get` could ever return. An error channel with
exactly one possible value is carrying one bit of information, and Go already spells that bit
comma-ok in map lookups, type assertions and channel receives.

- **Two decisions went to the maintainer rather than being guessed, and the ROADMAP was wrong on one
  of them.** It said "plus a `New` **alias**", but spec v2's own gap analysis (line 51) said
  `New(...)` — so "alias" was this repo's earlier paraphrase, not the requirement. Confirmed as a
  **rename**: two constructors for one type is what a major removes, not adds.
- **`ErrNotFound` is deleted, not kept as a migration landmark.** Keeping it looked friendlier and is
  worse: nothing would return it, so `errors.Is(err, cache.ErrNotFound)` would still compile and
  simply never be true — a compile-time break converted into silent runtime falsehood. A removed
  identifier fails loudly, which is what a major is for.
- **ADR-0021 is only *partially* superseded, and the surviving half is load-bearing.** Its
  lazy-expiry-on-`Get` model is what makes the boolean sound: expiry is judged against the entry's
  deadline at call time, never against the sweeper's schedule, so there is no window in which `Get`
  returns `true` for an entry the caller must not use. **Had expiry depended on the sweeper having
  run, the bool would have been a weaker promise than the error was.** The ADR status line says so
  precisely instead of marking the whole document superseded — the expiry model, sharding, the single
  sweeper and the usable-after-`Close` posture all stand.
- **Absence and expiry stay indistinguishable.** They already were, under one sentinel. Separating
  them would promise callers something about *when* eviction happens, which is exactly the freedom
  ADR-0038's per-shard, non-atomic sweep depends on.
- **The one genuinely new invariant got its own test: a stored zero value is not a miss.** Implicit
  under `(V, error)`, but it is the entire reason comma-ok exists — with the value alone there is no
  way to tell `Set(k, 0)` from "never set", and a caller caching zeros, empty strings or nil slices
  would silently re-fetch forever. `TestStoredZeroValueIsNotAMiss` pins it.
- **No performance claim and no benchmark report**, deliberately. Returning a bool instead of a
  pre-allocated sentinel cannot plausibly move a measurement; publishing a number here would be
  manufacturing a win. (13.2 earned its report by finding a real 276 ns; this item has no such
  finding, and saying so is the honest outcome.)

**A grep that under-reported, caught by the compiler.** The first sweep for call sites listed two
`Get` uses in `lifecycle_test.go`; there were four, because two more were spelled
`got, err := c.Get(...)` inside loops the pattern missed. `go vet` and the build named them
immediately, which is the useful lesson: for a signature change, the compiler is the authoritative
call-site index and a grep is only a preview.

## Where the project stands

Milestone 13 is **3 of 10** (13.1 layout, 13.2 errx, 13.3 cache). `version.go` stays 1.1.1 until
13.10. 100% coverage on `cache` retained; the package no longer imports `errors` at all.

## How the next session resumes

Confirm 13.3's PR merged, re-sync `master`, **create the branch before the first edit**. Then 13.4:
`workerpool.Stop` → `Close` (ledger item 1) — the smallest remaining rename, but check whether
`ErrPoolClosed`'s wording and the Stop-then-Submit contract still read correctly once the method is
called `Close`, and remember that `ErrPoolClosed` is the sentinel `spec_api_lint` caught in 12.3 as
the second member of a `var (…)` block.
