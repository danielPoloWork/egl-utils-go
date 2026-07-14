# Benchmark Report: ratelimit hot paths (Allow / funded Wait)

- **Date:** 2026-07-14
- **Version / commit:** v0.1.0 + unreleased M2–M3 work (branch `feat/ratelimit`, PR #16;
  parent `master` @ `5c49670`)
- **Environment:** Intel Core i5-6600K @ 3.50GHz (4 cores), 32 GB RAM, Windows 10 Pro
  (10.0.19045), go1.26.5 windows/amd64, default (release) build. Developer workstation —
  numbers are informational, not a gating baseline.
- **Command:** `go test -run '^$' -bench . -benchmem -count 5 ./ratelimit/`

## Scenario

The admission decision is the limiter's hot path: it sits in front of every guarded call,
so its cost and allocation behavior bound the overhead a caller pays per request. Four
paths are measured: `Allow` admitting (bucket kept funded by a 1e12/s refill), `Allow`
denying (bucket drained, ~1e-9/s refill), `Allow` under 4-way contention
(`RunParallel`), and the funded `Wait` fast path (no sleep taken; burst 1024 absorbs
same-clock-tick call bursts — see Interpretation).

## Results

Median of 5 runs; spread is min–max of the 5.

| Metric | Value | Spread |
|--------|-------|--------|
| `Allow` admit | 25.06 ns/op, 0 B/op, 0 allocs/op | 24.76–25.91 ns/op |
| `Allow` deny | 25.42 ns/op, 0 B/op, 0 allocs/op | 25.31–26.06 ns/op |
| `Allow` parallel (4 threads) | 49.74 ns/op, 0 B/op, 0 allocs/op | 48.53–52.31 ns/op |
| `Wait` funded fast path | 50.33 ns/op, 0 B/op, 0 allocs/op | 43.95–63.05 ns/op |

## Interpretation

Admission is a mutex plus a few float64 operations: ~25ns single-threaded and
**zero-allocation on every no-sleep path**, admit and deny alike. Contention doubles the
cost (lock hand-off), still allocation-free.

One honest caveat surfaced while writing the benchmark: with `burst: 1`, back-to-back
`Wait` calls that land inside a single clock tick (Windows reports ~sub-microsecond
granularity) see zero elapsed time, zero refill, and take the timer path for a 1ns sleep
— measured at ~5.4µs/op with one 123 B timer allocation. That is the documented cost of
a *blocked* `Wait` (one timer per sleep), not of the fast path; the fast-path benchmark
uses `burst: 1024` so no iteration is pushed onto the timer. The ADR records this
boundary behavior.

## Reproduce

```bash
git checkout feat/ratelimit   # or master once PR #16 merges
go test -run '^$' -bench . -benchmem -count 5 ./ratelimit/
```
