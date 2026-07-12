# ADR-0009: semaphore design — thin adapter over x/sync, first runtime dependency

- **Status:** Accepted
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 5, §5 (semaphore API), §6; ROADMAP 2.5; ADR-0004 (runtime
  dependency policy — `golang.org/x/*` ring); patterns catalogue (Guarded Suspension)

## Context

Feature 5 commits to "weighted task admission control", with the API fixed at intake:
`NewWeighted(capacity int64) *Weighted; Acquire(ctx, weight) error; Release(weight)`, and
the spec and ADR-0004 both name `golang.org/x/sync/semaphore` as the intended engine.
Two decisions follow. First, *what to build*: a hand-rolled weighted semaphore, or a thin
wrapper over the vetted `x/sync` implementation. Second, *the supply-chain step*: this is
the module's **first runtime dependency**, so it also establishes `go.mod`'s `require`
graph and the committed `go.sum`.

## Decision

**Wrap, do not reimplement.** `Weighted` embeds an `x/sync/semaphore.Weighted` and
delegates the blocking, fairness (FIFO waiter queue), and weight accounting to it. The
wrapper adds only the module's house contract on top: `NewWeighted` panics on a
non-positive capacity, and `Acquire`/`Release` panic on a non-positive weight — the
loud-by-default idiom of ADR-0005/0007/0008, catching programming errors at the call site
instead of deadlocking or miscounting silently. The public surface is exactly the spec's
three methods; `x/sync`'s `TryAcquire` is deliberately not re-exported (not in spec §5, and
adding surface would require a spec change, not an ADR).

`golang.org/x/sync` enters under ADR-0004's ring 2 (`golang.org/x/*` treated as extended
stdlib) — no superseding ADR needed, this is the budgeted case. Pinned at **v0.16.0**,
**not** the latest (v0.22.0). The driver is the module's documented **1.24 `go` floor**
(AGENTS.md §1, spec §3, CI matrix): a dependency's own `go` directive must be satisfied by
the main module's directive, and `go build` enforces it in `-mod=readonly`. x/sync
v0.20.0+ declares `go 1.25.0` (would push the floor to 1.25); v0.17.0–v0.19.0 declare
`go 1.24.0`, which `go build` does not accept from a short-form `go 1.24` main directive —
it demands the patch-precise `go 1.24.0` and fails otherwise. v0.16.0 is the newest release
still on `go 1.23.0`, which a `go 1.24` main module satisfies unambiguously, so the
documented floor string stays exactly `go 1.24` with no dependency-driven bump. The
semaphore API (`NewWeighted`/`Acquire`/`Release`) is identical across all these releases, so
the pin costs nothing functional. A future 1.25 floor bump can adopt a newer x/sync.
`go.sum` holds the two canonical checksums for v0.16.0 retrieved from the Go checksum
database (`sum.golang.org`) and is committed (the module had no `go.sum` before this item).
`govulncheck` (CI blocking gate, compliance control C-1) now has a real dependency edge to
scan and verifies these checksums on every run.

## Alternatives Considered

- **Hand-rolled semaphore (channel- or `sync.Cond`-based)** — zero runtime dependencies,
  but re-implements a subtle, already-vetted primitive (weighted fairness, waiter wakeups)
  for no gain. Contradicts the spec's explicit `x/sync` choice and ADR-0004's "stdlib and
  `x/*` first" posture. Rejected.
- **Type alias / re-export of `x/sync`'s type** — leaks the dependency into the public
  surface (consumers would import `x/sync` types) and forfeits the house misuse contract.
  Rejected; the wrapper keeps the boundary and the panics.
- **Re-export `TryAcquire`** — useful, but outside spec §5. Rejected here; add via a spec
  amendment if a caller needs non-blocking admission.

## Consequences

- The first `require` edge lands; `go.sum` is now part of the tree and Dependabot's
  `gomod` watch (ADR-0004) has something to bump. Future runtime deps follow this same
  tidy-and-commit path.
- Behavior inherits `x/sync`'s guarantees verbatim: FIFO fairness, and a `weight >
  capacity` `Acquire` that blocks until its context is done. The tests pin the wrapper's
  own contract (capacity/weight panics, admission bound, cancellation) rather than
  re-testing `x/sync` internals.
- Catalogued as **Guarded Suspension** (in-taxonomy: block until a precondition — enough
  free weight — holds, then proceed), first use in the module.

## References

- `docs/specs/01_spec_utils.md` §2.5, §5, §6.
- ADR-0004 (runtime dependency policy, `x/*` ring), ADR-0005 (loud-by-default idiom).
- `golang.org/x/sync/semaphore` (v0.16.0 — newest on a `go 1.23.0` directive; see Decision).
