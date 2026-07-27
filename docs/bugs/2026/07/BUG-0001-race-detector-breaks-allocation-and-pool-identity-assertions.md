---
id: BUG-0001
title: The race detector breaks allocation-count and sync.Pool identity assertions, holding master red
status: fixed
severity: high
reporter: internal
discovered: 2026-07-27
affected-versions: ">=1.1.0"
fixed-in: v1.1.1
---

# BUG-0001: The race detector breaks allocation-count and sync.Pool identity assertions, holding master red

## Summary

Two CI jobs — `race / data-race detector` and `quality / lint + race + vuln` — have failed on
every push to `master` since 2026-07-26, **including the `v1.1.0` release commit `6907706` and
every commit after it**. No data race is involved. Four test assertions measure things the race
detector deliberately perturbs — allocation counts and `sync.Pool` object identity — so under
`-race` they report an instrumented binary's behaviour rather than the one consumers run, and fail
by construction.

## Environment

- **Affected versions:** `>=1.1.0` (the defect predates the tag; v1.1.0 was cut from a red master)
- **Toolchain / platform:** Go 1.25 and 1.26 on `ubuntu-24.04`; both failing jobs run `-race`
- **Configuration:** only `-race` builds. Every other CI job is green — all four `build` cells,
  `consistency`, `coverage`, `imports`, `fuzz`, `benchmark`, and both `contrib` modules.

## Reproduction

```bash
go test -race ./middleware/ ./syncpool/
```

Observed on `4ab2d6e` (CI run 30276714755):

```text
--- FAIL: TestNFR01AllocationBudget/Logger
    nfr_alloc_test.go:101: Logger: 2 allocs/op (budget 1)
--- FAIL: TestNFR01AllocationBudget/ChainWithLogger
    nfr_alloc_test.go:101: ChainWithLogger: 13 allocs/op (budget 11)
--- FAIL: TestNFR01GeneratedIDCostsOneMoreAlloc
    nfr_alloc_test.go:130: RequestID: 6 allocs adopting an inbound ID, 8 minting one
FAIL  github.com/danielPoloWork/egl-utils-go/middleware
--- FAIL: TestPutResetsAndReuses
    syncpool_test.go:28: (require.Same)
FAIL  github.com/danielPoloWork/egl-utils-go/syncpool
```

**The `syncpool` failure alternates between runs** — `TestPutRetainsBufferAtCap` on the first red
run (30211835507, 2026-07-26 17:06) and `TestPutResetsAndReuses` now. That non-determinism is
itself evidence: the two are the same assertion (`require.Same`) about pool identity, and which one
trips depends on scheduling.

## Expected vs. actual

- **Expected:** `-race` reports data races. It has never reported one here.
- **Actual:** four assertions fail on numbers the detector changed. NFR-01's budgets are exceeded by
  exactly the detector's own overhead (1 extra allocation on `Logger`, 2 on `ChainWithLogger`), and
  `sync.Pool` hands back a different buffer than the one returned.

## Root cause

Both classes of assertion read runtime internals that `-race` is documented to change.

- **Allocation counts.** `testing.AllocsPerRun` counts allocations in the binary it is running in.
  Race instrumentation allocates on its own account, so the count under `-race` is not the count a
  consumer's build produces. NFR-01's ratchet budgets (10.10, ADR-0037) and `syncpool`'s
  zero-allocation assertion (9.4, ADR-0028) both pin exact numbers.
- **`sync.Pool` identity.** `TestPutResetsAndReuses` and `TestPutRetainsBufferAtCap` assert
  `require.Same` — the very buffer returned by `Put` comes back from the next `Get`. That holds for
  a single goroutine with no GC because of the per-P private slot; the race detector changes P
  pinning and the slot's behaviour, so `Get` may legitimately allocate a fresh buffer. The sibling
  `TestPutDiscardsOversizedBuffer` asserts `require.NotSame` and is unaffected, which is why it
  never appeared in the failures.

**Why it went unnoticed for a day and through a release.** `-race` requires cgo, and the
maintainer's Windows workstation has no C compiler (`CGO_ENABLED=0`, `go build -race` refuses
outright). Every "full local gauntlet green" in the sessions that followed — including the v1.1.0
release cut — therefore never executed a single race-detector test. CI was the only gate on this
class, and CI was the thing failing. A gate whose failures nobody reads is a gate nobody has.

## Impact

**Severity high, for process reasons rather than runtime ones.** No consumer is affected: the
defect is in the tests, the library's behaviour is correct, and `v1.1.0` is sound. What is affected
is every subsequent change:

- `master` was red for a day, so **a genuine regression in the race jobs would have been invisible**
  — the signal was already noise. That includes real data races, the exact thing those jobs exist
  to catch, in a library whose first design principle is leak- and race-freedom.
- `v1.1.0` was tagged and released from a red `master`, contrary to the release protocol.
- The two `contrib` modules were tagged `v0.1.0` from the same red master (both `contrib` jobs were
  green, and the published modules contain only `contrib/` code, so the releases stand).

## Fix / workaround

The assertions are **structurally incompatible with `-race`**, so they are excluded from race builds
rather than loosened. Raising the budgets to accommodate the detector was rejected outright: it
would mask the real regressions the ratchet exists to catch, which is the opposite of the gate's
purpose.

Excluded with `//go:build !race` on dedicated files, so the exclusion is visible in a directory
listing rather than buried in a conditional:

| File | Holds |
|---|---|
| `middleware/nfr_alloc_test.go` (tagged in place) | all three NFR-01 allocation tests — the whole file is allocation assertions |
| `syncpool/syncpool_norace_test.go` (new) | `TestPutResetsAndReuses`, `TestGetPutIsZeroAllocInSteadyState` |
| `syncpool/syncpool_internal_norace_test.go` (new) | `TestPutRetainsBufferAtCap` |

**Every one of these assertions still gates.** They run in the ordinary `go test ./...` on all four
CI cells (Linux/Windows/macOS × Go 1.25/1.26); only the two `-race` jobs skip them. Nothing that can
usefully run under the detector was removed from it — `TestConcurrentGetPut`, the syncpool test the
detector is actually for, still runs there, as does `TestPutDiscardsOversizedBuffer`.

`TestGetPutIsZeroAllocInSteadyState` had not yet been observed failing, but it is the same class and
was moved with the others: leaving a latent flake in a job nobody watches is what produced this
record.

## References

- Fixing PR: this one
- `CHANGELOG` entry: `[Unreleased]` → Fixed
- CI evidence: run 30276714755 (`4ab2d6e`, current), run 30211835507 (`d14166d`, first red)
- Related: [ADR-0037](../../../adr/0037-nfr-benchmark-methodology.md) (NFR-01's ratchet budgets),
  [ADR-0028](../../../adr/0028-syncpool-bufferpool-design.md) (the zero-allocation contract),
  AGENTS.md §10 (`go test -race` as the canonical concurrency gate)
