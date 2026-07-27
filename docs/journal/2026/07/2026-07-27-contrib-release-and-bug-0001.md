# 2026-07-27 — contrib v0.1.0 released; BUG-0001, a red master nobody could see

## What got done

- **`contrib/redishealth/v0.1.0` and `contrib/pgxhealth/v0.1.0` released.** Annotated tags on
  master `4ab2d6e`, pushed, and **verified end to end rather than assumed**: both are indexed on
  `proxy.golang.org` with the right `Subdir` and `Hash`, and both are in `sum.golang.org`. They are
  installable now. Three choices worth recording:
  - **v0.1.0, not v1.0.0.** Pre-1.0 says the API may still change. ADR-0042's stability commitment
    is the core's alone; ADR-0040 has contrib version independently, and a first release should not
    open by promising more than it has to.
  - **`require egl-utils-go v1.0.0` left as it was.** It is a *minimum*, not a pin: `health.Check`
    has not changed since v1.0.0, so it is the widest floor that works. Bumping it to v1.1.0 would
    narrow the modules for no gain, and editing a `go.mod` needs a PR, not a tag.
  - **No GitHub Release.** `release.yml` fires on `tags: ["v*.*.*"]`, which does not match
    `contrib/…/v0.1.0` — checked before tagging, precisely so a misfire could not draft a bogus
    core release. For contrib the tag *is* the release; no changelog convention exists for them,
    and inventing one risks `consistency_lint.py`'s version-lockstep gate, which ties
    `CHANGELOG`/`docs/releases` to `version.go`.
  - The first `go get` of each returned `500` from `sum.golang.org` while it indexed the new
    module — the transient this repo has hit before. Retried; never reached for `GOSUMDB=off`.

- **Then found that `master` had been red for a day — [BUG-0001](../../../bugs/2026/07/BUG-0001-race-detector-breaks-allocation-and-pool-identity-assertions.md).**
  Two jobs, `race` and `quality`, have failed on every push since 2026-07-26 17:05, **including the
  `v1.1.0` release commit `6907706`**. Every other job is green: all four `build` cells,
  `consistency`, `coverage`, `imports`, `fuzz`, `benchmark`, and both `contrib` modules.
- **No data race was ever involved.** Four assertions measure things `-race` deliberately perturbs:
  - **allocation counts** — the detector allocates on its own account, so `Logger` reports 2 against
    a budget of 1 and `ChainWithLogger` 13 against 11. Those numbers describe an instrumented
    binary, not the one consumers run.
  - **`sync.Pool` identity** — `require.Same(returned, next Get)` holds for one goroutine with no GC
    because of the per-P private slot, and instrumentation changes P pinning. **Which** syncpool
    test tripped varied between runs (`TestPutRetainsBufferAtCap` first, `TestPutResetsAndReuses`
    now), and that non-determinism was the clue: same assertion, scheduling decides which one loses.
    The sibling `TestPutDiscardsOversizedBuffer` asserts `NotSame` and never failed, which fits.
- **Fixed by exclusion, not by loosening.** Raising the budgets to fit the detector was rejected
  outright — it would mask the real regressions the ratchet exists to catch. `//go:build !race` on
  dedicated files, so the exclusion shows up in a directory listing instead of hiding in a
  conditional: `middleware/nfr_alloc_test.go` tagged in place (the whole file is allocation
  assertions), plus new `syncpool/syncpool_norace_test.go` and
  `syncpool/syncpool_internal_norace_test.go`. **Every assertion still gates** — they run in the
  ordinary `go test ./...` on all four cells; only the two `-race` jobs skip them. Nothing that can
  usefully run under the detector was taken away from it: `TestConcurrentGetPut` still runs there.
- **`TestGetPutIsZeroAllocInSteadyState` was moved too, though it had not yet been seen failing.**
  Same class, and leaving a latent flake in a job nobody watches is what produced this record.

## Where the project stands

Core still at **v1.1.0** — this is a test-only fix, no version bump. Contrib modules released at
v0.1.0. No open milestone; M1–M12 all closed. The fix is on `fix/race-incompatible-alloc-assertions`
awaiting merge, and **CI on that branch is the first real verification of the race path** (see
below).

## How the next session resumes

**The honest limit of this fix's local verification.** `-race` requires cgo and this workstation has
no C compiler — `go build -race` refuses outright, and `-tags race` cannot substitute because
setting the tag by hand makes the runtime demand a tsan runtime that is not linked. So what was
verified locally is:

- `go list -tags race` shows the guard **selecting the right files** — the three `norace` files and
  `nfr_alloc_test.go` drop out, and `syncpool_test.go` / `syncpool_internal_test.go` stay;
- `go test ./...`, `go vet ./...` and `gofumpt` are clean with the moved tests still running and
  passing on the non-race path;
- all four policy tools pass.

**The `-race` execution itself is verified only by CI.** That is the same gap that let BUG-0001
live for a day, so it is worth saying plainly rather than implying the fix was proved locally: the
green `race` job on this PR is the proof, and it should be checked before merging rather than after.

Worth considering separately: **nothing alerts on a red `master`.** Nine merges landed on top of a
failing build without anyone noticing, because the failure was in jobs that only CI can run. A
branch-protection rule requiring the `race` and `quality` checks, or a notification on a failed
`master` run, would have turned a day of red into one blocked merge.

Unchanged carry-overs: the EADOS bundle's `go.yaml` profile (gitignored here, fix upstream);
`egl-util-cpp`'s `it/d4np/util` against ADR-0041's `utils`; the ADR-0030 `/v2` ledger; the NFR-01
spec amendment; and the `middleware.HeaderName` canonicalisation decision.
