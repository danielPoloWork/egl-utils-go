# 2026-07-16 — Milestone 10.2: circuitbreaker.State()

## What got done

- **Roadmap 10.2 `circuitbreaker.State()`** (branch `feat/circuitbreaker-state`): the first feature
  item of the spec-v2 reconciliation (Milestone 10) — breaker observability (v2 item 6), lifting the
  `State()` deferral ADR-0010 recorded. Additive and non-breaking, per ADR-0030.
- **Exported `State` type** (promoted from the internal `state uint8`): `StateClosed` (zero value,
  a fresh breaker), `StateOpen`, `StateHalfOpen`, with a `String()` → `"closed"`/`"open"`/
  `"half-open"`/`"unknown"`. The internal machine now uses this one canonical type throughout (the
  internal test's constant references were renamed accordingly); no parallel type system.
- **`(*Breaker).State()` is a pure read-only observer — the design crux.** The breaker's time
  transitions are lazy (evaluated on admission, no timers — ADR-0010), so an open breaker whose
  cool-down has elapsed has not *yet* flipped its stored field. `State()` takes the lock and reports
  the **effective** state (open-past-cooldown → `StateHalfOpen`, the state the next call would be
  admitted under) **without performing the transition**: it never mutates `b.state`, advances the
  generation, or admits a probe. So a metrics scraper polling `State()` cannot perturb the machine —
  the alternative (evaluating the transition *and mutating*) would have let mere observation orphan
  in-flight calls via a generation bump. Rejected; observer stays inert.
- Tests: an **internal** test on the fake clock pins the whole contract — Closed fresh → Open on
  trip → Open one tick short of the timeout → HalfOpen exactly at it — and asserts the pure-observer
  property directly (after `State()` returns HalfOpen, the raw stored state is still Open, the
  generation is unchanged, inFlight is 0). External tests cover `String()` (incl. the unknown/zero
  cases) and basic closed→open observability through real calls. 100% coverage.
- Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, 100% circuitbreaker
  coverage, gofumpt clean, golangci-lint v2 0 issues, `consistency_lint.py` OK. No new ADR
  (ADR-0030 records the adoption; ADR-0010 gets a forward-reference note). `-race` is CI-only locally.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (2 of 13)**: 10.1 (#37) merged; 10.2 drafted on
`feat/circuitbreaker-state`, awaiting the maintainer to open and merge. M10 releases as v1.1.0.

## How the next session resumes

Wait for the 10.2 PR to merge. Then **10.3 `lifecycle.Trigger()`** — programmatic shutdown that
unblocks a pending `WaitForSignals` (v2 item 21, §6 example): add a `triggered` channel to the
internal `coordinator`, have `WaitForSignals` `select` over both the signal channel and the trigger
channel, make `Trigger()` idempotent (close-once, like `Close`), and reset it in the test `swapStd`
helper. Additive; no signature change to the existing three functions. Standard footprint per PR
(tests+goleak+100% cov, CHANGELOG [Unreleased], ROADMAP checkbox, journal, lint). Portable Go under
`%TEMP%\go-portable`; `/v2` golangci-lint path; `-race` CI-only.
