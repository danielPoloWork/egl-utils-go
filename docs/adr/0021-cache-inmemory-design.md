# ADR-0021: cache.InMemory design — lazy expiry on Get, one sweeper goroutine, deterministic Close

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §1 (zero goroutine leaks, allocation-conscious hot paths), §2 feature 17, §5
  (`NewInMemory[K comparable, V any](ttl time.Duration, opts ...Option) *Cache[K, V]`;
  Get/Set/Delete; Close; ErrNotFound); ROADMAP 7.1 (opens Milestone 7, rides the M2 concurrency
  tier); ADR-0005 (loud-by-default, functional options); ADR-0010 (deterministic-clock testing)

## Context

Feature 17 is "a map-backed local cache with TTL expiry via a periodic cleanup thread", the
module's first component since M2 to **own a goroutine** — which under the zero-leak philosophy
(spec §1) makes its lifecycle the design's center of gravity. The API is frozen at intake. The open
decisions: what expiry means when the sweeper hasn't run yet, how the sweeper's lifetime is bound,
what Close leaves behind, and how expiry is tested deterministically.

## Decision

**Expiry is enforced by `Get`, not by the sweeper.** Every entry stores its deadline
(`Set` time + ttl); `Get` compares against the deadline at call time and returns `ErrNotFound` the
instant it passes — the deadline itself is exclusive (an entry read exactly at its deadline is
already expired). The periodic sweeper is therefore **purely a memory reclaimer**: its interval
(default ttl, override `WithCleanupInterval`) tunes how long expired garbage may linger, never
whether stale data can be observed. This decouples correctness from scheduling — the classic TTL
cache bug (a stale read in the window between deadline and sweep) is impossible by construction.

**One sweeper goroutine, deterministically stopped.** `NewInMemory` starts exactly one goroutine:
a `time.Ticker` loop selecting on an internal `done` channel. `Close` closes `done` through a
`sync.Once` — idempotent, concurrency-safe (verified by test), and immediate even mid-interval; the
ticker is stopped on the way out. goleak asserts the whole contract, including a dedicated test
that `Close` mid-interval leaves no goroutine behind.

**Close does not brick the cache.** After `Close`, `Get`/`Set`/`Delete` keep working — `Get` still
refuses expired entries — but nothing reclaims expired memory in the background; the godoc says a
closed cache should be left to the GC. Rationale: unlike `workerpool.Stop` (where accepting work
after stop would silently drop it), a closed cache misbehaves in no observable way, so a loud panic
would criminalize a harmless shutdown ordering; the graceful contract matches `pubsub.Close`
(ADR-0006, additive shutdown).

**Loud validation** (ADR-0005): `NewInMemory` panics on a non-positive ttl (a cache in which nothing
can live is a wiring bug), and `WithCleanupInterval` panics on a non-positive interval — at option
construction, so the failure points at the bad call site.

**Locking and hot-path cost.** A single `sync.RWMutex` over one `map[K]entry[V]`: `Get` takes the
read lock (concurrent readers scale), `Set`/`Delete`/sweep take the write lock. Deadlines are
computed *before* the lock, keeping the critical sections minimal. Measured on the reference box:
Get-hit ~28 ns, Set ~51 ns, **0 allocs/op** across the surface (spec §1's allocation-conscious
requirement) — co-located benchmarks, house style (`ReportAllocs`, `b.N`/`RunParallel`).

**Deterministic expiry tests via an injectable clock** (ADR-0010 idiom): an unexported
`now func() time.Time` field, overridden in internal tests to hand-crank time — the expiry boundary
(live at deadline−1ns, expired at the deadline) is asserted exactly, with no sleeps. The sweeper's
background reclamation is tested separately with a real clock and a tight interval.

## Alternatives Considered

- **Sweeper-only expiry (Get trusts the map)** — cheaper Get by one time comparison, but stale reads
  become schedulable (visible until the next sweep), and correctness would then *depend* on the
  goroutine the zero-leak philosophy wants inessential. Rejected.
- **A timer per entry** (`time.AfterFunc`) — prompt reclamation, but one timer per key is unbounded
  timer pressure and every timer is a leak candidate; a single sweep amortizes reclamation. Rejected.
- **Panic on use-after-Close** — the workerpool posture; rejected here because a closed cache has no
  failure mode to make loud (reads stay correct), and shutdown ordering between a cache and its users
  should not be a crash hazard. Documented graceful degradation instead.
- **Lazy deletion inside Get** (delete the expired entry while reading) — saves memory sooner but
  forces `Get` to take the write lock on the expiry path, penalizing the read fast path to do the
  sweeper's job. Rejected; `Get` only reports, the sweeper reclaims.
- **Sharded map / `sync.Map`** — higher write concurrency, but the single-RWMutex map benches at
  tens of nanoseconds with zero allocations, and sharding adds complexity with no measured need.
  Rejected until a benchmark says otherwise (additive, invisible to the API).
- **Per-entry TTL (`Set(key, value, ttl)`)** — more flexible, but a different public surface than the
  frozen one (per-cache ttl in the constructor). Deferred; an additive `SetWithTTL` could arrive later.

## Consequences

- The module gains its first post-M2 goroutine owner with the same discipline: leak-proof by
  construction, goleak-gated, `-race`-verified in CI (the local box cannot run the race detector —
  no cgo toolchain — so the Linux race job is the gate, as for all of M2/M3).
- Stale reads are impossible regardless of sweep scheduling; memory reclamation latency is tunable
  independently of correctness.
- Hot paths are zero-allocation (~28 ns Get), honouring spec §1 without `sync.Pool` machinery.
- Milestone 7 opens; `db.Transaction` (7.2) completes it.
- Deferred, additive surface: `SetWithTTL`, `Len`, sharding — none would break the frozen API.

## References

- `docs/specs/01_spec_utils.md` §1, §2.17, §5.
- ADR-0005 (loud-by-default, options), ADR-0006 (graceful Close precedent), ADR-0010
  (deterministic clock).
- `cache/cache_bench_test.go` — co-located benchmarks (house style).
