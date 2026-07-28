# ADR-0046: errx — rename off the stdlib name, opt-in stacks, and `[]Frame` instead of program counters

- **Status:** Accepted
- **Date:** 2026-07-28
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** ADR-0029 (superseded), ADR-0030 §2 ledger item 25, ADR-0045 (`/v2` boundary),
  spec §2 feature 25 / §4 `core` layer / §5; ROADMAP 13.2

## Context

ADR-0029 shipped `errors.Wrap`/`Wrapf` with three properties that item 25 of the ADR-0030 ledger
put on the `/v2` list:

1. **The package is named `errors`**, shadowing the standard library. Its own doc comment conceded
   the wart: a file needing both must alias one, and the package aliases `stderrors` internally to
   compile at all.
2. **Wrap captures a call stack implicitly.** The first wrap of a stackless chain ran
   `runtime.Callers`; later wraps reused the captured stack by reference so the trace kept pointing
   at the origin. Nobody asked for the capture, and every caller paid for it.
3. **The stack is exposed as `[]uintptr`** through `StackTracer`. Program counters are inert
   without `runtime.CallersFrames`, so reading a trace meant importing `runtime` and writing a
   four-line iterator — ADR-0029 explicitly considered a `Frame` type and deferred it.

Milestone 13 opens the major that lets all three change. This ADR settles what replaces them.

## Decision

**The package is `errx`** at `pkg/errx`, importable as
`github.com/danielPoloWork/egl-utils-go/v2/pkg/errx`. It no longer collides with the standard
library, so no consumer — and no file inside the package — needs an alias to use both.

**Stack capture is opt-in.** `Wrap` and `Wrapf` attach a message and nothing else; they never reach
into the runtime. `WithStack(err)` captures, and is the one place a caller says the stack is worth
its cost:

```go
if err != nil {
        return errx.Wrap(errx.WithStack(err), "loading config")
}
```

`WithStack` is nil-transparent and **idempotent**: given a chain that already carries a stack it
returns that chain unchanged, so the recorded trace keeps pointing at the earliest capture.

**ADR-0029's origin-pointing property survives, and is now structural rather than maintained.** The
stack lives at exactly one node in the chain instead of being copied into every wrap, and `Frames`
finds it by unwrapping. Wrapping an error therefore cannot move its trace, because wrapping does
not touch the trace at all.

**A trace reads as `[]Frame`.** `Frame` carries `Function`, `File` and `Line` — no runtime types,
so reading a trace never requires importing `runtime`:

```go
for _, f := range errx.Frames(err) {
        log.Printf("%s %s:%d", f.Function, f.File, f.Line)
}
```

**Resolution is lazy and cached.** Capture records program counters; the symbolization that turns
them into frames happens on first read, under a `sync.Once`, so an error that is both logged and
inspected pays for it once and an error that is never printed never pays at all. This split is
measured, not assumed — see Consequences.

**`Frames(err)` is the read path; `StackTracer` remains the extension point.** `Frames` performs the
`errors.As` walk internally and returns nil when nothing was captured, which replaces ADR-0029's
four-line dance with a `range`. `StackTracer` (now `StackTrace() []Frame`) is what `Frames` searches
for, so an error type defined outside this package can still contribute a trace.

The full surface is six identifiers: `Wrap`, `Wrapf`, `WithStack`, `Frames`, `Frame`, `StackTracer`.

## Alternatives Considered

- **Keep implicit capture in `Wrap`, and make `WithStack` merely a no-message capture helper.**
  Rejected: it contradicts the ledger's own wording, and it keeps the cost on a function whose
  documented job is to add a message. The measurements below show that cost is not marginal.
- **Resolve frames eagerly at capture time.** Simpler — no `sync.Once`, no cached field — but it
  loads symbolization onto every `WithStack` whether or not anyone reads the trace. Rejected on the
  numbers: symbolization measures 6.5× the capture itself (4147 ns against 643 ns), so eager
  resolution would have made the
  opt-in cost six times larger for a result most errors never produce.
- **`StackTracer` returning `[]uintptr`, with `Frames` doing the conversion.** Keeps a raw-PC escape
  hatch for exporters that want counters (OTel, Sentry). Rejected *for now* rather than on the
  merits: it puts the program-counter concept back into the surface that item 25 set out to remove,
  and a `PCs()`-style accessor can be added later **additively**, needing no major. Choosing the
  narrow surface now is the reversible option.
- **A `Frame.String()` method.** Drafted, then dropped. `Frame`'s fields are exported, so callers
  can format it however they log; adding the method would commit v2 SemVer to a rendering nobody
  asked for. Also additive later.
- **Retaining the `errors` name.** The rename is the least interesting change here but the one with
  the widest blast radius, and skipping it would waste the only boundary at which it is free.

## Consequences

- **Breaking, three ways:** the import path and package name change; `Wrap` no longer produces a
  stack; `StackTracer.StackTrace()` returns `[]Frame`, not `[]uintptr`. The migration is mechanical
  for the first two (`errors` → `errx`; add `WithStack` where a trace is wanted) and a simplification
  for the third — code that ran `runtime.CallersFrames` now ranges over `Frames(err)`.
- **A trace is now something you asked for.** The cost of the change is that an error wrapped
  without `WithStack` has no stack to print, where v1 would have captured one unbidden. `%+v` on
  such an error prints the message alone. This is the intended trade: v1's behavior meant every
  caller funded a trace for the few that read one.
- **Measured on this box (i5-6600K, `-count 5`, medians), with v1 and v2 compiled into the *same*
  binary and measured in the same run so the comparison is not across two noisy sessions. Per
  ADR-0037, the allocation columns are the hardware-independent claim and the latency columns are
  reported, not gated:**

  | Operation | v1 | v2 | Change |
  | --- | --- | --- | --- |
  | `Wrap`, stackless chain | 550.1 ns · 336 B · 3 allocs | 37.1 ns · 32 B · 1 alloc | **14.8× faster, 2 allocs removed** |
  | `Wrap`, chain already carrying a stack | 276.0 ns · 80 B · 2 allocs | 36.0 ns · 32 B · 1 alloc | **7.7× faster, 1 alloc removed** |
  | `WithStack` (capture) | — | 642.7 ns · 360 B · 4 allocs | the opt-in cost |
  | `Frames`, first read (symbolize) | — | 4147 ns · 544 B · 5 allocs | **6.5× the capture** |
  | `Frames`, cached read | — | 229.8 ns · 16 B · 1 alloc | the `errors.As` walk |

  **The allocation figures are the durable half.** Within that controlled run the spreads were tight
  (`Wrap` v1 546.9–558.8, v2 35.5–42.0), but an independent in-repo run of the same `WithStack`
  benchmark ranged 1018–2258 ns — consistent with ADR-0037's finding that this machine moves
  microbenchmarks by tens of percent between identical runs. Treat the ratios as order-of-magnitude
  evidence, not as a regression budget; the alloc counts are exact and reproducible.

- **The second row is the finding this ADR would not have had without measuring.** v1's `Wrap` cost
  240 ns even when it captured nothing, because `originStack` ran `errors.As` over the whole chain on
  *every* wrap to discover the stack already there. Removing implicit capture removed that walk too:
  v2's `Wrap` never inspects the chain, so wrapping costs the same whether or not a stack exists.
  The win is therefore not only "no capture on the first wrap" — it is "no chain walk on any wrap".
- **`hasStack` deliberately does not go through `Frames`.** Answering "does this chain carry a
  stack?" must not trigger symbolization, or `WithStack`'s idempotence check would resolve an entire
  trace just to conclude it has nothing to do. The two helpers look redundant and are not.
- **100% statement coverage**, matching the package's v1 figure and clearing ADR-0036's 85% gate.
  Reached the same way ADR-0029 reached it: by deleting an unreachable defensive branch rather than
  leaving it uncovered. A length guard in `resolve` was dead, because `runtime.CallersFrames` over an
  empty slice already reports `more=false` on its first `Next`.
- **No dependency change and no new internal edge** — `errx` imports only the standard library, so
  ADR-0004's rings and ADR-0035's layer graph are untouched. `errx` has no in-module consumers, which
  is what made the rename isolated.
- **Deferred, both additive and therefore free to add in v2.x:** a raw-PC accessor for tracing
  exporters, and `Frame.String()`.

## References

- `pkg/errx/errx.go`, `pkg/errx/errx_test.go`, `pkg/errx/errx_bench_test.go`.
- `docs/benchmarks/2026-07-28-errx-opt-in-stacks.md` — the measurements above, with method.
- ADR-0029 (superseded) — the v1 design and its deferred `Frame` type.
- ADR-0030 §2 — the ledger this empties one more entry from; ADR-0045 — the `/v2` boundary.
- ADR-0037 — the methodology under which latency is reported rather than gated.
