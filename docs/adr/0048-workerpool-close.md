# ADR-0048: workerpool.Stop becomes Close — one shutdown verb for the module

- **Status:** Accepted
- **Date:** 2026-07-29
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** ADR-0005 (`Stop`/`ErrPoolClosed` names superseded; every semantic it decided is
  preserved), ADR-0025 (no hidden shutdown timeout — the reason `ctx` stays), ADR-0030 §2 ledger
  item 1, ADR-0045 (the `/v2` boundary), spec §2 feature 1 / §5; ROADMAP 13.4

## Context

Three types in the module own a goroutine and must be released. Two spell that `Close`
(`cache.Close`, `pubsub.Close`); `workerpool` spelled it `Stop`. Nothing distinguished the odd one
out — ADR-0005 simply picked a verb before there was a second lifecycle to be consistent with, and
by the time there were three the name was frozen by the v1 compatibility contract (ADR-0042).

A consumer wiring all three writes three shutdowns and has to remember that one of them is not
`Close`. `io.Closer` is the shape Go readers expect for "release this thing", and a pool is exactly
that thing.

Ledger item 1 also recorded the sentinel divergence: the v2 target is `ErrClosed`, shipped as
`ErrPoolClosed`.

## Decision

**`Stop(ctx) error` becomes `Close(ctx) error`.** A pure rename: not one line of the shutdown body
changed.

**`ErrPoolClosed` becomes `ErrClosed`.** The package name already qualifies it —
`workerpool.ErrClosed` says "pool closed" once instead of twice. The ROADMAP line for 13.4 named only
the method, but the ledger names both, and an item left half-discharged is one 13.10 would have to
find still open (maintainer confirmed).

**`Close` keeps its context, and the module's shutdown vocabulary is deliberately not uniform in
*signature*.** The gap analysis writes the target as `Close() error`, which is the table's shorthand:
its own gap column flags only the method and sentinel names, never the parameter. Dropping `ctx` would
be a semantic change wearing a rename's clothes, and it is rejected on ADR-0025's grounds — a
`Close()` that waits for caller-supplied tasks either waits forever or invents a hidden timeout, and
ADR-0025 refused hidden timeouts as a documented, deliberate deviation.

The asymmetry with `cache.Close()` and `pubsub.Close()` is therefore principled rather than
tolerated: **the pool is the only shutdown in the module that waits on work the caller wrote.** A
cache sweeper and a broker's fan-out loop are the module's own goroutines, and their termination is
bounded by construction. Uniformity is worth having on the verb, where it removes a thing to remember;
it is not worth having on the parameter list, where it would remove a control the caller needs.

## Alternatives Considered

- **`Shutdown(ctx) error`**, after `http.Server`. Fitting, and it is *the* precedent for
  "context-bounded graceful drain". Rejected only because the module already committed to `Close` on
  two of three lifecycles — a third verb to unify two would be a net loss.
- **Keep `Stop`.** The v2 boundary is the only place this can change; not spending it here means
  spending v3 on it, or never.
- **`Close() error` per the literal ledger, plus a separate `CloseContext(ctx) error`.** Satisfies
  `io.Closer`. Rejected: two shutdown methods on one type is the ambiguity `Close` was meant to remove,
  and the no-arg form still has to choose between an unbounded wait and a hidden timeout.
- **Keep `ErrPoolClosed`.** Rejected: see Decision — it would leave ledger item 1 partly discharged
  inside the very major that exists to empty it.

## Consequences

- **Breaking, two ways, both mechanical:** `p.Stop(ctx)` → `p.Close(ctx)`;
  `workerpool.ErrPoolClosed` → `workerpool.ErrClosed`. Both fail at compile time, which is the point
  of doing them in a major.
- **`Pool` still does not implement `io.Closer`**, because `Close` takes a context. That is the
  accepted cost of the decision above, and it is worth stating plainly rather than discovering: code
  written against `io.Closer` cannot accept a `*Pool`.
- **Every ADR-0005 semantic survives untouched:** blocking-first admission, the `RWMutex`
  discipline that makes `close(queue)` provably race-free, idempotence, the drain guarantee, the
  execution context canceled only on deadline, and the loud-panic policy. ADR-0005 is superseded on
  two names and nothing else.
- **No behavioural change and no benchmark report.** A rename cannot move a measurement; the
  co-located NFR suite was updated to the new name and still passes.
- **100% statement coverage retained.** No new test was added: the rename introduced no new invariant,
  and the existing `TestSubmitAfterCloseReturnsErrClosed`,
  `TestCloseIsIdempotentAndConcurrent` and `TestCloseDeadlineCancelsExecutionContext` already pin the
  behaviour under its new spelling. Adding a test here to look thorough would be adding a duplicate.

## References

- `pkg/workerpool/workerpool.go`, `pkg/workerpool/workerpool_test.go`,
  `pkg/workerpool/nfr_bench_test.go`.
- ADR-0005 — the design this renames; ADR-0025 — the no-hidden-timeout ruling that keeps `ctx`.
- ADR-0030 §2 — the ledger, item 1 discharged; ADR-0045 — the `/v2` boundary.
