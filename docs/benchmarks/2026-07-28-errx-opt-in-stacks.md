# Benchmark Report: errx — the cost of implicit stack capture, and of removing it

- **Date:** 2026-07-28
- **Version / commit:** v1.1.1 + unreleased `/v2` work (branch `feat/v2-errx`, roadmap 13.2,
  ADR-0046; parent `master` @ `9dc2d18`)
- **Environment:** Intel Core i5-6600K @ 3.50GHz (4 cores), 32 GB RAM, Windows 10 Pro
  (10.0.19045), go1.26.5 windows/amd64, default (release) build. Developer workstation —
  numbers are informational, not a gating baseline (ADR-0037).
- **Command:** `go test -run '^$' -bench 'Wrap|WithStack|Frames' -benchmem -count 5 ./pkg/errx/`,
  with the v1 `errors` package compiled into the **same test binary** as `errx` so both are
  measured in one run under identical conditions.

## Scenario

ADR-0029's `Wrap` captured a call stack implicitly: the first wrap of a stackless chain ran
`runtime.Callers`, and later wraps reused that stack by reference. ADR-0046 makes capture opt-in via
`WithStack` and exposes traces as `[]Frame`, resolved lazily. Three questions decided the design and
each is measured here:

1. **What did implicit capture actually cost `Wrap`?** — whether "opt-in" buys anything real.
2. **What does `Wrap` cost when the chain already carries a stack?** — v1 skipped capture on this
   path, so it should have been cheap. It was not, and the reason is the finding below.
3. **Is symbolization worth deferring?** — i.e. resolve `[]Frame` at capture, or on first read.

Measuring the same operation across two separate sessions on this box is not sound (see
Interpretation), so v1 and v2 are linked into one binary and benchmarked in a single run.

## Results

Median of 5 runs; spread is min–max of the 5. `B/op` and `allocs/op` were identical across all runs.

| Benchmark | Median ns/op | Spread | B/op | allocs/op |
|-----------|--------------|--------|------|-----------|
| `WrapV1` (stackless chain) | 550.1 | 546.9 – 558.8 | 336 | 3 |
| `WrapV2` (stackless chain) | **37.1** | 35.5 – 42.0 | **32** | **1** |
| `WrapOverExistingStackV1` | 276.0 | 272.6 – 374.5 | 80 | 2 |
| `WrapOverExistingStackV2` | **36.0** | 35.2 – 36.9 | **32** | **1** |
| `WithStack` (opt-in capture) | 642.7 | 626.3 – 669.1 | 360 | 4 |
| `FramesFirstRead` (symbolize) | 4147 | 4073 – 4236 | 544 | 5 |
| `FramesCachedRead` | 229.8 | 228.9 – 240.2 | 16 | 1 |

## Interpretation

**1. Implicit capture dominated `Wrap`.** 550.1 → 37.1 ns, a 14.8× reduction, with allocations
falling 3 → 1 and bytes 336 → 32. The 336 B is the giveaway: 256 of them are the
`make([]uintptr, 32)` backing every captured stack. Every caller of `Wrap` funded that array
whether or not anything ever read the trace.

**2. The unexpected result, and the one that changed how ADR-0046 is argued: v1's `Wrap` cost
276 ns even when it captured nothing.** On this path the stack already existed, so `runtime.Callers`
never ran — yet the call was still 7.7× v2's. The cost was `originStack`'s `errors.As` walk over the
whole chain, executed on *every* wrap to discover the stack that was already there. Because v2 keeps
the stack at a single node and only looks for it when a trace is read, `Wrap` no longer inspects the
chain at all: 36.0 ns against 37.1 ns for the stackless case — statistically the same, i.e. **v2's
`Wrap` cost is now independent of what the chain contains**, which v1's never was. So removing
implicit capture removed a second, quieter cost that the "opt-in" framing had not predicted.

**3. Deferring symbolization is worth ~6.5× the capture.** `FramesFirstRead` at 4147 ns against
`WithStack`'s 642.7 ns means resolving eagerly would have made every `WithStack` roughly seven times
more expensive, to produce `[]Frame` values that most errors — logged as `%v`, matched with
`errors.Is`, never printed with `%+v` — never need. `FramesCachedRead` at 229.8 ns confirms the
`sync.Once` cache works: a trace that is both logged and inspected symbolizes once. The residual
229.8 ns and 1 alloc are the `errors.As` walk plus the interface boxing of the `StackTracer` target,
inherent to finding the stack by unwrapping rather than copying it into every node.

**Why one binary rather than two sessions.** An earlier in-repo run of the *same* `WithStack`
benchmark ranged 1018 – 2258 ns, against 626 – 669 ns in the controlled run above. That is this
machine, not the code: ADR-0037 already records that microbenchmarks here move tens of percent
between identical runs, and 10.10's NFR work found the Windows clock unusable below ~1 ms. Hence the
same-binary method, and hence the ratios are presented as order-of-magnitude evidence. **The
allocation and byte columns carry the durable claim** — they are deterministic, reproduce exactly,
and are what ADR-0037 classifies as a hardware-independent result.

## Reproduce

```
go test -run '^$' -bench . -benchmem -count 5 ./pkg/errx/
```

The committed suite measures v2 only (`BenchmarkWrap`, `BenchmarkWrapf`,
`BenchmarkWrapOverExistingStack`, `BenchmarkWithStack`, `BenchmarkWithStackIdempotent`,
`BenchmarkFramesFirstRead`, `BenchmarkFramesCachedRead`). The v1 columns above came from a
throwaway module that imported both packages at once; v1's `errors` package no longer exists in the
tree after 13.2, so that comparison is reproducible only from a checkout at or before `9dc2d18`.
