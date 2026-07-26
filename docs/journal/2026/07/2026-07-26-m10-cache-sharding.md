# 2026-07-26 — Milestone 10.11: cache sharding (the bench demanded it)

## What got done

- **Roadmap 10.11** (branch `feat/cache-sharding`): the thousand-cache lifecycle test and — because
  10.10's measurements said so — internal sharding. **New
  [ADR-0038](../../../adr/0038-cache-sharding.md)**; the NFR report was **updated in place** rather than
  superseded, so the before/after sits in one document.
- **The conditional in the brief was answered by evidence, not preference.** 10.11 was written as
  "shard internally *only if the bench demands*". 10.10 measured the 90/10 mix at **349.8 ns** against
  **78.9 ns** read-only, all of it from one `sync.RWMutex` serialising all eight readers behind every
  `Set`. The bench demanded it.
- **32 independently locked shards**, selected with `maphash.Comparable(seed, key) & (shardCount-1)`.
  `maphash.Comparable` (Go 1.24+, floor is 1.25) is the right tool because it hashes exactly the
  constraint `K comparable` already carries, and it panics only where using the value as a map key would
  panic anyway — no new failure mode. The **seed is per cache**, so shard assignment cannot be predicted
  from outside and a caller cannot craft keys that pile onto one shard.
- **Result: NFR-06 met at the mean, 7.5× better on the workload it describes.**

  | Benchmark | Before | After | Change |
  |---|---|---|---|
  | `NFR06Mixed` (90/10, 8 goroutines, 1 M) | 349.8 ns | **46.6 ns** | **7.5× faster** |
  | `NFR06GetOnly` | 78.9 ns | 57.7 ns | 1.4× faster |
  | `GetParallel` (4 threads) | 59.7 ns | 56.1 ns | 1.06× faster |
  | `GetHit` (uncontended) | 27.2 ns | 32.9 ns | 21% slower |
  | `GetMiss` (uncontended) | 15.1 ns | 20.1 ns | 33% slower |
  | `Set` (uncontended) | 52.0 ns | 60.5 ns | 16% slower |

  The mixed path is now *faster than the read-only path*, which is itself the evidence that write
  contention has stopped being the bottleneck.
- **The uncontended regression is real and recorded rather than glossed.** About 5 ns per operation — the
  `maphash.Comparable` call — which is 16–33% in relative terms. A consumer using one cache from one
  goroutine now pays a small tax for protection they do not need. Accepted because ~5 ns absolute buys
  ~300 ns on the contended path and a shared cache is the normal case, but the numbers are in the ADR so
  the trade can be revisited instead of rediscovered. I measured master's baseline directly rather than
  trusting the figures I had on file.
- **One sweeper per cache, preserved and now pinned.** The tempting parallel to sharding the locks is a
  sweeper per shard; that would make a thousand caches own **32 000** goroutines instead of 1 000, trading
  lock contention for a scheduler problem and breaking the "exactly one goroutine" contract the module's
  zero-leak guarantee rests on. One sweeper visits the 32 shards in turn, each under its own lock — still
  strictly better than the single global lock it replaces, since no pause spans the keyspace.
  `TestThousandCachesOwnOneGoroutineEach` asserts the new-goroutine count stays within one per cache, so
  this cannot regress silently.
- **`removeExpired` is no longer an atomic snapshot** across the keyspace. Harmless, and only because of
  ADR-0021's central invariant: expiry is judged by `Get` against the deadline, never by the sweeper's
  schedule, so a shard swept a moment after its neighbour just means memory reclaimed a moment later.
  Stated explicitly in the code and the ADR, because it is exactly the kind of quiet weakening a later
  reader would otherwise trip over.
- **The thousand-cache lifecycle suite** (spec item 17): create/use/close a thousand caches; the
  one-goroutine-per-cache assertion; concurrent create-and-close across 16 workers; three concurrent
  `Close` calls per cache over a thousand caches (a second close of the `done` channel would panic — the
  `sync.Once` holds); and a 10 000-key round-trip through `Set`/`Get`/`Delete` that would catch a bad mask
  or a per-call seed. Every one goleak-clean, with `Close` called **before** the deferred
  `goleak.VerifyNone` — the LIFO trap recorded since 7.1, which would otherwise have shown a thousand
  live sweepers.
- Also rejected, with reasons in the ADR: `sync.Map` (untyped, so the generic cache would erase its own
  type safety, and no way to scan for expiry — plus its read-mostly optimisation is defeated by the exact
  10% write mix NFR-06 specifies); a `GOMAXPROCS`-derived shard count (results stop being comparable
  between the developer's box, CI and production); **cache-line padding** between shards (standard
  practice, but the win from sharding alone is 7.5× and padding costs memory in every cache for a
  false-sharing effect nothing here has demonstrated — measure first); and a caller-supplied hasher.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...`, gofumpt clean,
  golangci-lint v2 0 issues, govulncheck 0 affecting, `consistency_lint.py`, `import_graph_lint.py`,
  `coverage_gate.py` all OK. `cache` stays at **100% coverage**. No API change, no dependency change
  (`hash/maphash` is stdlib).

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (11 of 13)**: 10.1–10.10 merged (#37, #38, #46–#53); 10.11
drafted on `feat/cache-sharding`, awaiting the maintainer to open and merge. M10 releases as v1.1.0.

## How the next session resumes

Wait for the 10.11 PR to merge. Then **10.12 `pubsub.WithDropOldest`** — an additive slow-subscriber
policy option, with the default staying drop-newest plus the drop handler (v2 item 2). Points to settle:

- The current policy is a **non-blocking send**: a subscription whose buffer is full simply misses that
  message, and the drop handler observes it (ADR-0006). `WithDropOldest` inverts that — on a full buffer,
  discard the *oldest* buffered message to make room for the new one. Over a Go channel that means a
  non-blocking receive followed by a non-blocking send, and the race to think through is that a
  *subscriber* may consume between the two, so the receive can succeed while the send still finds the
  buffer full (or vice versa). Decide and document what happens then rather than looping indefinitely.
- **Which message the drop handler reports** changes meaning under the new policy: today it reports the
  message that could not be delivered; under drop-oldest the message that *was evicted* is the one lost.
  Get that right and say so in the godoc — a consumer counting drops needs to know which it is counting.
- The option is per broker (like `WithSubscriberBuffer`), so it applies to every subscription; per-
  subscription policy would need a `Subscribe` signature change, which the v1 commitment forbids.
- `pubsub` sits at **96.4%** coverage, the third-lowest in the module — worth closing the gap while in
  there, and `BenchmarkNFR03FanOut` already exists to check the delivery path has not regressed
  (it asserts `delivered + dropped == 10 × publishes`, which the new policy must keep true).

Standard footprint per PR (tests + coverage ≥ 85% per package, CHANGELOG `[Unreleased]`, ROADMAP
checkbox, journal, lint, and the three policy tools). Portable Go under `%TEMP%\go-portable` — in the Bash
tool add it as the *unix* path `/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs
the `/v2` module path; `-race` is CI-only, and `-fuzz` needs the restored `pkg/include` +
`src/runtime/cgo` headers.
