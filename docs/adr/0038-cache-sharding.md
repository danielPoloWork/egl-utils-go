# ADR-0038: shard the cache internally — 32 shards, hashed with maphash.Comparable

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0021 (the cache's design: Get enforces expiry, the sweeper only reclaims), ADR-0037 (the NFR suite that produced the evidence), ADR-0030 (spec v2 reconciliation: 10.11 adopted), ADR-0005 (loud-by-default), ADR-0047 (renamed the constructor and changed `Get`'s signature; this decision is untouched), spec v2.0 item 17 and NFR-06, roadmap 10.11

## Context

Roadmap 10.11's brief was deliberately conditional: *"1 000-cache create/close goleak test + NFR-06
1 M-entry p99 bench; **shard internally only if the bench demands**"*. Roadmap 10.10 built that benchmark
and answered the question:

| Measured in 10.10 (1 M entries, 8 goroutines) | Result |
|---|---|
| `Get` only | 78.9 ns/op |
| 90/10 read/write mix | **349.8 ns/op** |

A 4.4× penalty from adding 10% writes, against an NFR-06 target of p99 ≤ 200 ns that the *mean* already
missed. The cause is structural rather than incidental: `Cache` was one `map` behind one `sync.RWMutex`,
so every `Set` took the exclusive lock and serialised all eight readers. Reads are fast; contention was
the whole problem. **The bench demanded sharding.**

Two constraints shape the solution. The public API is frozen by the v1 commitment —
`NewInMemory`/`Get`/`Set`/`Delete`/`Close` (the v1 surface as it stood; `NewInMemory` is `New` and
`Get` returns `(V, bool)` since [ADR-0047](0047-cache-comma-ok.md), which changed the names and the
signature but not one word of this decision) — so this must be invisible from outside. And the cache's
goroutine contract is load-bearing: it owns *exactly one* goroutine, which the module's zero-leak
guarantee and this milestone's thousand-cache test both depend on.

## Decision

Split each cache across **32 independently locked shards**, selecting a shard with
`maphash.Comparable(seed, key) & (shardCount-1)` against a **per-cache seed**, and keep **one sweeper
goroutine** that visits the shards in turn, each under its own lock. No API change.

## Alternatives Considered

- **Keep one lock and accept the NFR miss.** Rejected: the requirement is in the spec, the fix is
  contained, and the evidence for it was gathered specifically to answer this question. Leaving a known
  4.4× contention penalty in a cache — the one component most likely to sit on a hot path — is not a
  defensible outcome for a milestone whose brief was to consider exactly this.
- **`sync.Map`.** Purpose-built for high read concurrency and would have removed the lock entirely.
  Rejected on fit: it is untyped (`any` keys and values, so the generic `Cache[K, V]` would erase its own
  type safety and allocate on every store), it offers no way to scan for expired entries without
  `Range`-ing the whole structure, and its read-mostly optimisation is defeated by exactly the 10% write
  mix NFR-06 specifies.
- **A shard count derived from `runtime.GOMAXPROCS`.** Adapts to the machine, which is superficially
  attractive. Rejected for determinism: the shard count would then vary between the developer's box, CI
  and production, so a benchmark result would not be comparable across them and a contention bug could
  reproduce on one machine only. A fixed 32 comfortably exceeds the core count of the machines this
  library targets, and 32 is cheap enough that adapting buys nothing.
- **Per-shard sweeper goroutines.** The obvious parallel to sharding the locks, and it would shorten the
  sweep. Rejected because it multiplies the cache's goroutine count by 32: a thousand caches would own
  **32 000** goroutines instead of 1 000, which trades a lock-contention problem for a scheduler one and
  breaks the "exactly one goroutine" contract. One sweeper visiting 32 shards in turn holds each lock
  briefly, which is strictly better than the single global lock it replaces — and
  `TestThousandCachesOwnOneGoroutineEach` now pins the property so this cannot regress silently.
- **Cache-line padding between shards** to avoid false sharing between adjacent mutexes. Standard
  practice for sharded structures, and deliberately *not* done: the win from sharding alone is 7.5×, the
  padding would cost real memory in every cache (and a process may hold thousands), and no measurement
  here shows false sharing is a live cost. Measure before adding it — the same discipline that produced
  this ADR in the first place.
- **A caller-supplied hash function** (`WithHasher`), avoiding `maphash` and letting a consumer optimise
  for their key type. Rejected as surface for a problem nobody has: it would push a correctness-critical
  choice onto callers, and `maphash.Comparable` already handles every type `K comparable` admits.
- **Hashing with `fmt.Sprint(key)` or reflection.** Rejected as far slower than the operation being
  protected, and allocating.

## Consequences

- **NFR-06 is now met at the mean, and the fix is worth 7.5×** on the workload the NFR describes.
  Measured before/after on the same machine (median of 5):

  > **Flagged 2026-08-06 (roadmap 14.8).** "Met at the mean" needs a correction that does **not** touch
  > this decision. `NFR06Mixed` is a parallel benchmark, so its ns/op is an *aggregate throughput*
  > figure — Go divides wall time by operations across all goroutines — and NFR-06's 200 ns is a
  > *latency* target. Caller-observed latency is the aggregate times the concurrency, so at eight
  > goroutines a mix measuring 46.6 ns/op has each `Get` occupying several hundred nanoseconds of its
  > caller's wall clock. **The 7.5× stands and so does everything below it**: more throughput on the
  > contended path is more throughput, and `GetOnly` no longer beating `Mixed` is still the evidence
  > that write serialisation stopped being the bottleneck. What does not stand is the phrase "meets
  > NFR-06". The measured tail and what to do about the target are in
  > [ADR-0037](0037-nfr-benchmark-methodology.md)'s 2026-08-06 amendments and the
  > [NFR report](../benchmarks/2026-07-26-nfr-suite.md).

  | Benchmark | Before | After | Change |
  |---|---|---|---|
  | `NFR06Mixed` (90/10, 8 goroutines, 1 M) | 349.8 ns | **46.6 ns** | **7.5× faster** |
  | `NFR06GetOnly` (8 goroutines, 1 M) | 78.9 ns | 57.7 ns | 1.4× faster |
  | `GetParallel` (4 threads) | 59.7 ns | 56.1 ns | 1.06× faster |
  | `GetHit` (uncontended) | 27.2 ns | 32.9 ns | **21% slower** |
  | `GetMiss` (uncontended) | 15.1 ns | 20.1 ns | **33% slower** |
  | `Set` (uncontended) | 52.0 ns | 60.5 ns | **16% slower** |

- **The uncontended paths pay for it, and that is the honest trade**: about 5 ns per operation, the cost
  of the `maphash.Comparable` call, which is a 16–33% relative regression on single-threaded use. A
  consumer using one cache from one goroutine now pays a small tax for concurrency protection they do not
  need. It is accepted because the absolute cost is ~5 ns while the benefit on the contended path is
  ~300 ns, and because a cache in this library exists to be shared — the concurrent case is the normal
  one. The numbers are recorded so the trade can be revisited rather than rediscovered.
- **The one-goroutine-per-cache contract survives**, now tested at scale: a thousand caches created,
  used and closed leave nothing behind, and the new-goroutine count stays within one per cache rather
  than one per shard.
- `removeExpired` is **no longer an atomic snapshot** across the keyspace, since shards are swept one at a
  time. This costs nothing given ADR-0021's central invariant — expiry is judged by `Get` against the
  deadline, never by the sweeper's schedule — so a shard swept a moment after its neighbour only means
  memory reclaimed a moment later. Stated explicitly because it is the kind of weakening that would
  otherwise be discovered by a confused reader.
- The **per-cache seed** means shard assignment differs between caches and cannot be predicted from
  outside, so a caller cannot craft keys that pile onto one cache's single shard. Not the reason for the
  seed, but a welcome property.
- Memory grows by 31 additional empty maps per cache — an empty Go map allocates its header and no
  buckets, so this is on the order of a kilobyte and a cache holding nothing stays cheap.
- `maphash.Comparable` panics only where using the value as a map key would panic anyway (an
  interface-typed `K` holding a non-comparable dynamic value), so no new failure mode is introduced.
- No API change, no dependency change (`hash/maphash` is stdlib; `Comparable` needs Go 1.24 and the
  module floor is 1.25). `cache` stays at 100% coverage.
- The 10.10 NFR report is **updated in place** with the post-sharding figures rather than superseded, so
  the before/after sits in one document.

## References

- Spec v2.0 item 17, NFR-06; roadmap 10.11 ("shard internally only if the bench demands").
- ADR-0021 (`Get` enforces expiry, the sweeper reclaims — what makes a non-atomic sweep harmless),
  ADR-0037 + [the NFR report](../benchmarks/2026-07-26-nfr-suite.md) (the evidence and the updated
  numbers).
- `hash/maphash.Comparable` (Go 1.24+); `cache/cache.go`, `cache/lifecycle_test.go`,
  `cache/nfr_bench_test.go`.
