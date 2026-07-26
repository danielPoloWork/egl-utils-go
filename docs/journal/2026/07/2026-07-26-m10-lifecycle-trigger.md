# 2026-07-26 — Milestone 10.3: lifecycle.Trigger()

## What got done

- **Roadmap 10.3 `lifecycle.Trigger()`** (branch `feat/lifecycle-trigger`): programmatic shutdown —
  spec v2 item 21 and its §6 example — adopted additively per ADR-0030. `Register`, `Shutdown`, and
  `WaitForSignals` keep their exact signatures; only a new package-level function appears.
- **Coordinator-scoped latch, not a process-wide flag.** The internal `coordinator` gains a
  `triggered chan struct{}` (built in `newCoordinator`) plus a `triggerOnce sync.Once`. `Trigger()`
  closes the channel through the Once, which makes it idempotent and safe under concurrency — a
  second `close` of a channel panics — and needs no interaction with `mu`, which guards only the hook
  slice and the `Shutdown` state machine. Because the channel is per-coordinator, the test `swapStd`
  helper resets it for free; no extra bookkeeping was needed there.
- **`WaitForSignals` now blocks in a `select`** over the signal channel and `std.triggered`. The
  design point: a *closed* channel — rather than a one-shot send — means **a `Trigger` that arrives
  before `WaitForSignals` is entered latches instead of being lost**. That is the difference between
  a correct wake and a startup race, and it is pinned by its own test. The package still owns no
  goroutines: this is a blocking wait in the caller's goroutine, so the module's zero-leak invariant
  is untouched (ADR-0025 amended with a note to that effect).
- `Trigger()` deliberately does **not** run the hooks itself — it only wakes the waiter, which then
  runs `Shutdown` on the single existing path. A process that never calls `WaitForSignals` therefore
  sees no effect from `Trigger` and should call `Shutdown`; the godoc says so plainly.
- Tests (100% coverage, goleak on every case): `Trigger` unblocks a *pending* `WaitForSignals`
  (driven through a new `swapSilentSignals` seam that subscribes but never delivers, so only the
  trigger can wake it); trigger-before-wait is not a lost wakeup; 8 concurrent `Trigger`s do not
  panic and still yield exactly one shutdown, with a post-shutdown `Trigger` still a no-op; and the
  latch is per-coordinator (a fresh coordinator does not inherit an earlier `Trigger`).

## Incidental fix — master did not build

`go build ./...` failed on a clean module cache with *missing go.sum entry for go.mod file*: the
Dependabot `prometheus/client_golang` 1.24.1 bump (#44) updated the direct dependency but left the
transitive `prometheus/common`, `prometheus/procfs`, and `protobuf` pins at their pre-bump versions,
with no matching `go.sum` entries. Repaired here with `go mod tidy` (also drops the now-unneeded
`go.yaml.in/yaml/v2`) because it blocked the gauntlet; noted under `Fixed` in the changelog.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (3 of 13)**: 10.1 (#37) and 10.2 (#38) merged; 10.3
drafted on `feat/lifecycle-trigger`, awaiting the maintainer to open and merge. M10 releases as
v1.1.0. Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, 100% lifecycle
coverage, gofumpt clean, golangci-lint v2 0 issues, `consistency_lint.py` OK. No new ADR — ADR-0030
records the adoption, ADR-0025 gets the amendment notes. `-race` is CI-only locally.

## How the next session resumes

Wait for the 10.3 PR to merge. Then **10.4 `ratelimit.Middleware()` + `ErrLimited`** — a
429-on-deny HTTP middleware layered over the existing rate-limit engine (v2 item 8), additive: a
sentinel `ErrLimited` plus a `net/http` middleware in the shape the `middleware` package already
establishes, reusing the limiter rather than reimplementing admission. Standard footprint per PR
(tests + goleak + 100% coverage, CHANGELOG `[Unreleased]`, ROADMAP checkbox, journal, lint). Portable
Go under `%TEMP%\go-portable`; golangci-lint needs the `/v2` module path; `-race` is CI-only.
