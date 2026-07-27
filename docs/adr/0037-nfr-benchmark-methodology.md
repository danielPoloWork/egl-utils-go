# ADR-0037: NFR benchmark methodology — gate what is hardware-independent, report the rest

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0030 (spec v2 reconciliation: 10.10 adopted), ADR-0036 (coverage gate — same "a gate that cannot fail is not a gate" reasoning), ADR-0012 (ratelimit's injected clock, which makes NFR-04 gateable), ADR-0006 (pubsub drop policy, which shapes NFR-03's measurement), ADR-0013/0014/0016/0017 (the middleware NFR-01 measures), spec v2.0 §5 and §7, roadmap 10.10, roadmap 10.11 (consumes NFR-06's numbers)

## Context

Spec v2 §5 states six numeric NFRs and a methodology: "`go test -bench` with `B.ReportAllocs`, ≥ 10 runs
compared via `benchstat` (p < 0.05), reference machine Ryzen 7 5800X, latest stable Go, pinned in
`bench/README`. Nightly CI tracks results; **> 10% regression fails**." Roadmap 10.10 asks for the
benches, the methodology, and a nightly workflow, and tags the item "perf methodology,
flaky-resistant gating".

Three facts constrain any honest implementation.

**The NFRs are not the same kind of claim.** Some are properties of the code — an allocation count, the
arithmetic of a token bucket — and hold identically on every machine. Others are throughput and latency
figures that depend on cores, clock speed, scheduler and neighbours.

**The reference machine does not exist here.** The available workstation has 4 cores against the
reference's 8, and CI runs on shared GitHub-hosted runners which are unpinned and vary far more than 10%
between runs for CPU-bound microbenchmarks. `docs/benchmarks/README.md` already records the workstation
as "informational, not a gating baseline".

**Percentiles need a clock the platform may not have.** NFR-02 and NFR-06 state p99 targets of 2 µs and
200 ns, and Go's framework reports means, so percentiles require per-operation sampling.

## Decision

Split the suite by the kind of claim each NFR makes. **Hardware-independent properties are hard
assertions in the ordinary test suite** — NFR-01's allocation counts, NFR-04's ±1% admission accuracy
(exact, via ratelimit's injected clock), NFR-05's zero-allocation steady state — so they fail the build
on every CI cell. **Hardware-dependent throughput and latency are measured and tracked, not gated**: a
nightly workflow runs the suite at `-count 10`, compares against the previous nightly with `benchstat`,
flags movements over 10% as warnings in the job summary, and **does not fail**. Where a tail cannot be
measured soundly, the benchmark reports `tail-unmeasurable` rather than a number.

NFR-01's "0 allocs/op" is unachievable and is replaced, in practice, by an **enforced budget at the
measured floor**.

## Alternatives Considered

- **Failing the nightly job on a >10% regression, as §5 specifies.** Rejected as a documented deviation,
  and the reasoning is the same one ADR-0036 used for coverage: a gate that fires on noise is worse than
  no gate, because it gets switched off and takes the honest signal with it. A hard threshold needs a
  stable baseline on pinned hardware; shared runners routinely move microbenchmarks by tens of percent
  between identical runs, and the baseline here is a cached artifact from a *different* runner instance.
  Note that the roadmap itself already reads "(>10% flags)" rather than "fails", so this deviation is
  consistent with the milestone as planned. It is recorded, per ADR-0030's practice of maintaining
  deviations with their ADRs.
- **A same-runner A/B comparison instead** — check out base and HEAD, benchmark both in one job, and gate
  on the delta. This is the statistically defensible way to *fail* on a regression, because machine noise
  is common-mode and largely cancels. Genuinely attractive, and deliberately deferred rather than
  rejected: it doubles CI time on every PR and belongs to a PR-triggered workflow, not a nightly tracker.
  Recorded here as the way to get a real gate when one is wanted.
- **Self-hosted or pinned runners** to make §5's gate viable — out of scope for a library repo, and it
  trades a noise problem for an infrastructure one.
- **Asserting the throughput NFRs anyway** ("≥ 1 M tasks/s" as a test). Rejected: it would pass with 4×
  margin on any developer machine and fail spuriously on a loaded CI runner, testing the runner rather
  than the code. The measurements have 4×–13× headroom, which is exactly the situation where an
  assertion adds flakiness and no information.
- **Carrying NFR-01's 0-allocs/op target unchanged.** Rejected on evidence: `context.WithValue` (1),
  `r.WithContext` (1), and each `Header.Set`'s stored `[]string` (1) are structural — any middleware that
  propagates a value through the context and writes a response header allocates. A target that can only
  ever fail enforces nothing, so the measured floor is pinned as a ratchet budget instead
  (`TestNFR01AllocationBudget`); raising an entry requires a stated reason in the PR. The spec target
  needs amendment, which is noted for the /v2 ledger rather than silently ignored. **(Amendment made
  2026-07-27 — and the /v2 ledger was the wrong bucket. Nothing about this module's API blocks the
  target and no major release would make zero reachable, so ledgering it would have filed it
  somewhere it could never be resolved. It is recorded instead as a maintained deviation in
  [ADR-0030](0030-spec-v2-reconciliation.md) §3, which is the register for "v2 asks X, we do Y, here
  is why". The target itself is not edited: it lives in `docs/specs/v2/`, a verbatim import marked
  unmodifiable, and the frozen v1 contract never made the claim.)**
- **Fixing the non-canonical `middleware.HeaderName` in this PR.** Measuring showed
  `"X-Request-ID"` is not Go's canonical `"X-Request-Id"`, so `textproto.CanonicalMIMEHeaderKey`
  allocates a fresh string on every `Get` and every `Set` — 2 of `RequestID`'s 6 allocations, for
  nothing, with no effect on the wire format (net/http canonicalises when writing). Rejected *for this
  PR*: 10.10 is the measurement item, the constant's value is API-visible under the v1 stability
  commitment, and measure-first discipline says the optimisation is a separate, evidenced change. It is
  recorded as a follow-up with its numbers. **(Correct conclusion, faulty middle step — see the
  Consequences note above. The fix never required touching the exported constant, so it landed as a
  PATCH in v1.1.1 rather than waiting on a major: [ADR-0044](0044-canonical-header-key-for-map-access.md).)**
- **Per-operation percentile sampling** for NFR-02 and NFR-06. Rejected as unsound on the available
  platform, having been tried: `time.Now()` on Windows is a coarse cached counter — **100% of adjacent
  `Now`/`Since` pairs read exactly 0 ns** — so the first Submit benchmark reported a p99 of exactly 0 ns.
  Batched sampling (1000 operations, per-operation mean) is used instead, and its weakness is stated
  rather than hidden: averaging inside a batch dilutes the tail, making the figure a regression detector
  and a lower bound rather than a percentile.
- **Trusting a probed clock resolution to decide whether a tail is measurable.** Rejected after it let a
  p99 of "4988 ns/op" through — a clean multiple of the tick. So did counting distinct sample values
  (hundreds of batches on *different* multiples of one tick give hundreds of distinct values), and so did
  taking the smallest gap between samples (one lucky 100 ns pair suffices). The accepted test is the
  **median positive gap** between adjacent sorted samples, which describes the granularity that actually
  governed the data; when it is not ≥ 20× finer than the median sample, no percentile is published.

## Consequences

- **Verdicts, not just benchmarks.** Every NFR now has a measured number and a stated verdict
  ([report](../benchmarks/2026-07-26-nfr-suite.md)): NFR-03 met by 12.7×, NFR-02 throughput met by 4.4×,
  NFR-04 met exactly (0.0000% deviation) and gated, NFR-05 met and already gated, NFR-01 latency met with
  ~6% headroom but its allocation target unachievable, NFR-06 **not met** and NFR-02/NFR-06 tails
  unverified on the available platform. A suite that only proved benchmark functions exist would be
  worth much less than one that says which requirements the library actually meets.
- **NFR-06's failure is architectural and actionable.** Reads average 79 ns; adding 10% writes takes the
  mean to 350 ns, because a single `sync.RWMutex` means every `Set` serialises all readers. **This is the
  evidence roadmap 10.11 was told to gather before deciding whether to shard — the bench demands
  sharding**, and the Get-only/mixed gap is the number to re-measure afterwards.
- Three hard gates join the test suite and run on every CI cell: the allocation budget, NFR-04's accuracy
  and its never-exceeds-budget companion. They cost milliseconds.
- A `nfr-nightly` workflow runs at 03:00 UTC and on demand, publishing a benchstat comparison to the job
  summary, warning on >10% movements, and uploading raw results. Its baseline is a prefix-matched cache
  entry from the previous nightly.
- **Measurement traps are documented in the code that hit them**, because each cost real time to find and
  would otherwise be rediscovered: the pubsub drop path making an undrained fan-out benchmark measure
  nothing; dead-code elimination reporting 0 allocations for a discarded result; a full token bucket
  wasting an idle window's refill against the cap; and the clock behaviour above.
- The local workstation cannot verify two of the six NFRs. That is a real limitation of this report, and
  CI is the authority for them — the same posture the repo already takes for `-race` and `-fuzz`.
- Deferred: the same-runner A/B PR gate; a spec amendment for NFR-01's allocation target; and
  re-measuring NFR-06 after 10.11.
- **Resolved 2026-07-27 (v1.1.1):** the `middleware.HeaderName` canonicalisation, worth the 2
  allocs/op measured here. This ADR deferred it as needing an API-visible change; it did not —
  see [ADR-0044](0044-canonical-header-key-for-map-access.md). The alternative rejected below was
  the right call *for that PR* (a measurement PR should not carry a behaviour change) but its
  stated reason, that the fix requires touching the exported constant, was wrong.

## References

- Spec v2.0 §5 (the six NFRs and the methodology), §7 (nightly benchmarks with a regression gate).
- [`docs/benchmarks/2026-07-26-nfr-suite.md`](../benchmarks/2026-07-26-nfr-suite.md) — the measurements
  and per-NFR verdicts; `docs/benchmarks/README.md` — the reporting convention.
- ADR-0036 (a gate that cannot fail is not a gate), ADR-0012 (injected clock → NFR-04 is exact),
  ADR-0006 (pubsub drops → NFR-03 must count deliveries).
- `middleware/nfr_bench_test.go`, `middleware/nfr_alloc_test.go`, `workerpool/nfr_bench_test.go`,
  `pubsub/nfr_bench_test.go`, `cache/nfr_bench_test.go`, `ratelimit/nfr_test.go`,
  `.github/workflows/nfr-nightly.yml`.
