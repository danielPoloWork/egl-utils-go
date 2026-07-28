# 2026-07-28 — 13.2: errx, and the 276 nanoseconds v1 spent looking for a stack it already had

## What got done

**A red required check, not a missing click.** The session opened intending to start 13.2 and found
13.1's PR #71 still unmerged. It was not waiting on the maintainer: `fuzz / 10-minute budget` was
failing in six seconds with `stat …/config: directory not found`. The `pkg/` move had left the only
two places in CI where a package path is **written out by hand** pointing at the old root —
`ci.yml`'s two fuzz targets, and `nfr-nightly.yml`'s six enumerated benchmark packages. Every other
job resolves packages through `./...`, which follows a move automatically.

- **The nightly was the more dangerous of the two.** It is not a required status check, so it would
  have failed that night with nobody watching, rather than on the PR that broke it.
- Fixed on 13.1's own branch (`c4f212a`) as finishing 13.1 rather than as new scope; no CHANGELOG
  entry, since CI paths are not user-visible (the 10.1 precedent). Verified against the real
  toolchain instead of by inspection: both fuzz targets reach their corpora and fuzz, all six
  nightly benchmark paths resolve. #71 then went **13/13 green** and merged as `9dc2d18`.
- **The lesson generalises past this PR:** a layout change is only ever as risky as its hand-written
  paths, and **none of the four policy tools can see them.** `consistency_lint.py` asserting that
  every explicit `./pkg/…` path in a workflow exists would have caught both. Offered; not built,
  because it is not 13.2's business.

**Then 13.2 itself: `errors` → `errx`, and stacks become something you ask for.** The three design
questions the ROADMAP had flagged as open were settled with the maintainer before any code was
written — capture opt-in, `Frame` resolved lazily, `Frames(err)` alongside a retained `StackTracer`
— and all three came back as recommended. ADR-0046 supersedes ADR-0029.

- **ADR-0029's origin-pointing property survives, but stopped being maintained and became
  structural.** v1 copied the captured stack by reference into every later wrap to keep the trace on
  the origin. v2 keeps the stack at exactly one node and has `Frames` find it by unwrapping, so
  wrapping *cannot* move a trace — because wrapping no longer touches it.
- **The benchmark changed how the ADR argues.** The expected result was there (`Wrap` 550.1 → 37.1
  ns, 3 → 1 allocs, 14.8×, once `runtime.Callers` and its 256-byte array stop being mandatory). The
  unexpected one: **v1's `Wrap` cost 276 ns even on the path where it captured nothing**, because
  `originStack` ran an `errors.As` walk over the whole chain on *every* wrap to discover the stack
  that was already there. v2's `Wrap` never inspects the chain, so it costs 36.0 ns against 37.1 —
  **its cost is now independent of what the chain contains**, which v1's never was. Removing implicit
  capture removed a second, quieter cost that "opt-in" had not predicted.
- **Lazy resolution was vindicated, not assumed:** symbolization measures **6.5× the capture** (4147
  ns against 643). Resolving eagerly would have made every `WithStack` seven times dearer to produce
  `[]Frame` values that an error logged as `%v` or matched with `errors.Is` never needs.
- **`hasStack` deliberately does not go through `Frames`.** They look redundant; they are not.
  Answering "does this chain already carry a stack?" must not trigger symbolization, or
  `WithStack`'s idempotence check would resolve an entire trace just to conclude it had nothing to
  do. Writing the obvious version first is how the trap surfaced.
- **Two identifiers were drafted and dropped** — `Frame.String()` and a raw-PC accessor for tracing
  exporters. Both are **additive**, so they cost nothing to omit now and would cost a major to
  remove later. The surface is six identifiers.
- **100% coverage, reached ADR-0029's way:** by deleting a dead defensive branch rather than leaving
  it uncovered. A length guard in `resolve` was unreachable, because `runtime.CallersFrames` over an
  empty slice already reports `more=false` on its first `Next`.

**Measurement honesty.** A first in-repo run put `WithStack` between 1018 and 2258 ns; the controlled
run put it at 626–669. That spread is the machine, not the code, and ADR-0037 already records it. So
v1 and v2 were compiled into **one binary and measured in a single run**, the ratios are published as
order-of-magnitude evidence, and the allocation columns — deterministic, exactly reproducible — carry
the durable claim.

Also folded in at the maintainer's request: `local-build.md` still told contributors "≥ 80% line"
while the same file's tool list said 85% per package. A stale fact from before ADR-0036, amended in
place.

## Where the project stands

Milestone 13 is **2 of 10**. `version.go` stays 1.1.1 until 13.10; the ADR-0030 §2 ledger has one
more entry discharged. `errx` has no in-module consumers, which is what made the rename isolated —
the remaining items are not all so lucky: 13.5 (pubsub reshape) and 13.6 (metrics dropping the
Prometheus SDK) are redesigns, and 13.6 is the only item that touches the dependency graph.

## How the next session resumes

Confirm 13.2's PR merged, re-sync `master`, and **create the branch before the first edit** (the 12.3
slip). Then 13.3: `cache.Get` → `(V, bool)` with a `New` alias. The live question there is what
ADR-0021's Get-enforced expiry means for the boolean — a value that is present but expired must read
as absent, or the comma-ok idiom will quietly hand back stale data.
