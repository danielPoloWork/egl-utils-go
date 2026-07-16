# ADR-002: Wrap `golang.org/x/sync` and `golang.org/x/time/rate` — don't reimplement

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-07-14 |
| **Related spec** | [d4np-go.md](../d4np-go.md) (§2 items 5, 8) |

## Context
v1 contained two inaccuracies that forced this decision into the open: item 5 attributed the weighted semaphore to a nonexistent stdlib `sync/semaphore` (it lives in `golang.org/x/sync/semaphore`), and item 8 described a token bucket built "on Go timers", implying a reimplementation of what `golang.org/x/time/rate` already provides. The `golang.org/x` modules are maintained by the Go team, sit inside the ADR-003 dependency budget (stdlib + `x/` only), and their concurrency code is among the most battle-tested in the ecosystem.

## Options considered

**A. Wrap the `x/` implementations** *(chosen)*
- ✅ Correctness inheritance: `x/time/rate`'s bucket math (reservations, burst handling, monotonic-clock edge cases) and `x/sync/semaphore`'s FIFO fairness have years of production hardening; reimplementations of exactly these two primitives are a classic source of subtle bugs (timer drift, lost wakeups).
- ✅ The wrappers add what the library actually contributes: middleware ergonomics (`ratelimit.Middleware()`), typed errors (`ErrLimited`), and contract documentation — thin, testable value.
- ✅ Within the ADR-003 dependency policy by definition.
- ❌ Public types partially expose `x/` semantics; version bumps of `x/` modules are ours to track. Accepted: they are low-churn modules.

**B. Reimplement (v1's implied path for item 8)**
- ✅ Zero external types in the API; freedom to tune.
- ❌ Re-derives well-known hard code for no functional gain; every fairness/accuracy bug becomes ours; NFR-04 (±1% burst accuracy) would gate *our* math instead of verified math.

**C. Expose the `x/` packages directly with no wrapper**
- ✅ Thinnest possible layer.
- ❌ Loses the middleware integration, typed error taxonomy, and the single-import ergonomics that justify the library's existence; consumers can already import `x/` themselves.

## Decision
**Option A.** `semaphore.Weighted` re-exports `x/sync/semaphore` behind the library's option pattern; `ratelimit.Limiter` wraps `x/time/rate.Limiter`, adding `Middleware()`, `ErrLimited`, and the §4 contract. Documentation states the underlying implementation explicitly — no originality theater.

## Consequences
- NFR-04's accuracy benchmark validates the *wrapper overhead*, not reinvented bucket math.
- `errgroup`, also from `x/sync`, becomes the sanctioned internal composition primitive (used by `fanout`/`lifecycle`) rather than hand-rolled WaitGroup+error plumbing.
- If `x/time/rate` semantics ever diverge from the library's documented contract, the wrapper absorbs the difference or this ADR is superseded — consumers' contracts stay stable.
