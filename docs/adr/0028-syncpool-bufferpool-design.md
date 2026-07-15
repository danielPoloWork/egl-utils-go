# ADR-0028: syncpool.BufferPool design — sync.Pool of bytes.Buffer, reset on return, discard oversized

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §1 (allocation-conscious hot paths via sync.Pool object reuse), §2 feature 24,
  §5 (`NewBufferPool() *BufferPool`, `Get() *bytes.Buffer`, `Put(*bytes.Buffer)`); ROADMAP 9.4;
  patterns catalogue (Object Pool); ADR-0005 (loud-by-default)

## Context

Feature 24 is "a `bytes.Buffer` pool managed by `sync.Pool` to optimize temporary string/buffer
allocations" — spec §1 names `sync.Pool` object reuse as one of the module's three pillars, and this
is its concrete realization. The API is frozen at intake. The design decisions are the two ways a
naive `sync.Pool` wrapper goes wrong: returning a **dirty** buffer, and **retaining an outsized**
buffer that pins memory forever. This is also the adoption of the **Object Pool** pattern (catalogue).

## Decision

**A `sync.Pool` with a `New` that makes empty `*bytes.Buffer`.** `Get` returns a ready-to-write
empty buffer (allocating only when the pool is drained); `Put` returns one for reuse. In steady
state the cycle allocates nothing — verified two ways: a `testing.AllocsPerRun` assertion of **0
allocations** for a Get/write/Put cycle, and a co-located benchmark (~17 ns/op, 0 allocs/op; ~8 ns
under RunParallel) in the house style (`ReportAllocs`, `b.N`/`RunParallel`).

**`Put` resets before pooling.** A returned buffer is `Reset()` so the next `Get` hands out an empty
one; a pool that returned dirty buffers would be a silent data-corruption/leak hazard (one caller's
bytes visible to the next). `Get` therefore guarantees `Len() == 0`.

**`Put` discards a buffer grown past a cap** (`maxRetainedCap`, 64 KiB) instead of pooling it. This
is the decision that separates a correct pool from a memory leak: `sync.Pool` retains whatever you
put back, so a single request that grows a buffer to many megabytes would pin that capacity in the
pool indefinitely, and under bursty large payloads the pool's steady-state memory ratchets only
upward. Capping retention bounds steady-state memory to roughly `(cap) × (pooled count)`; the
occasional large buffer is served, used, and dropped to the GC. 64 KiB is generous for the
string/serialization work this targets while still bounding pathological retention.

**A nil `Put` is ignored** (a harmless caller slip, not worth a panic), and the zero `BufferPool` is
documented as unusable — `NewBufferPool` is the constructor. There is no other misuse surface to
guard loudly.

## Alternatives Considered

- **Pool without a retention cap** — simplest, and the classic `sync.Pool` memory leak: outsized
  buffers are parked forever and steady-state memory ratchets up under bursty load. Rejected; the cap
  is the whole point of a *correct* buffer pool.
- **Not resetting in `Put` (reset in `Get` instead)** — equivalent in effect, but resetting on return
  keeps a dirty buffer from lingering in the pool and makes `Get`'s "always empty" contract obvious.
  Either works; reset-on-`Put` chosen for the cleaner invariant.
- **A generic `Pool[T]`** — more reusable, but the spec froze a `bytes.Buffer` pool, and a buffer's
  reset/retention policy is type-specific (it hinges on `Cap`/`Reset`). A generic pool is a possible
  additive later; this stays concrete.
- **A configurable retention cap** (`NewBufferPool(maxCap int)`) — flexible, but not the frozen
  no-arg constructor; deferred as an additive option. 64 KiB is a sensible fixed default.
- **Panic on a nil `Put`** — consistent with loud-by-default elsewhere, but a nil buffer to `Put` is a
  trivial no-op with no downstream hazard (unlike a nil handler/dependency); silently ignoring it is
  the proportionate choice and keeps the hot path branch-cheap.

## Consequences

- A drop-in buffer pool with zero steady-state allocations and **bounded** memory: correct under
  both typical and bursty-large workloads.
- The **Object Pool** pattern moves from *Planned* to *Implemented* in the catalogue (row 10), with
  this ADR and `syncpool/` as its code location — realizing spec §1's `sync.Pool` pillar.
- Milestone 9 has one item left: `errors.Wrap` (9.5), after which the library is feature-complete.
- Deferred, additive: a configurable cap, a generic `Pool[T]`, pools for other reusable types.

## References

- `docs/specs/01_spec_utils.md` §1, §2.24, §5.
- `docs/patterns/design-patterns.md` — Object Pool (Creational).
- ADR-0005 (loud-by-default). `sync.Pool`, `bytes.Buffer`, `testing.AllocsPerRun`.
- `syncpool/syncpool_bench_test.go` — co-located zero-alloc benchmarks.
