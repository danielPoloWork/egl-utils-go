# Benchmark Report: metrics — what the Prometheus SDK cost per request, and per scrape

- **Date:** 2026-07-29
- **Version / commit:** v1.1.1 + unreleased `/v2` work (branch `feat/v2-metrics-no-vendor-api`,
  roadmap 13.6, ADR-0050; parent `master` @ `205b044`)
- **Environment:** Intel Core i5-6600K @ 3.50GHz (4 cores), 32 GB RAM, Windows 10 Pro
  (10.0.19045), go1.26.5 windows/amd64, default (release) build. Developer workstation —
  numbers are informational, not a gating baseline (ADR-0037).
- **Command:** `go test -run '^$' -bench 'ZZ' -benchmem -count 3 ./pkg/metrics/`, with v1's
  `client_golang`-backed recording path compiled into the **same test binary** as v2's
  hand-written one, so both are measured in one run under identical conditions.

## Scenario

ADR-0050 replaces `prometheus/client_golang` with an in-house counter, histogram and text-exposition
writer. The dependency question was settled by the module graph (nine modules for two metric
families) rather than by performance, so the measurement's job was **not** to justify the change but
to establish what it costs — a hand-rolled implementation being *slower* than the library was the
plausible outcome worth knowing about before shipping.

Two paths matter, and they have very different shapes:

1. **Recording** — runs on every request, so allocations and nanoseconds both count.
2. **Scraping** — runs once per Prometheus poll (typically every 15–60 s), where the interesting
   figure is allocation churn rather than latency.

The v1 recording path is reproduced verbatim from the v1 middleware body:
`code := strconv.Itoa(status)`, then `WithLabelValues(method, code).Inc()` and
`.Observe(seconds)`.

**Reproducibility caveat, stated up front:** these comparisons cannot be re-run from the repository,
because the implementation they compare against was removed in the same PR. The v2 half is
reproducible via `pkg/metrics/metrics_bench_test.go`; the v1 half exists only in this report and in
git history.

## Results

Median of 3 runs; spread is min–max. `B/op` and `allocs/op` were identical across all runs of a
given benchmark.

### Recording (per request) — same binary, same run

| Benchmark | ns/op (median) | spread | B/op | allocs/op |
|---|---|---|---|---|
| v1 `client_golang` | 223.0 | 221.6 – 240.6 | 3 | **1** |
| v2 hand-written | **63.4** | 58.3 – 102.8 | 0 | **0** |
| v1, status string pre-rendered | 190.7 | 185.8 – 239.2 | 0 | 0 |
| v2, parallel (one shared label pair) | **104.0** | 95.5 – 105.1 | 0 | 0 |
| v1, parallel (one shared label pair) | 155.9 | 155.9 – 192.8 | 3 | 1 |

**v2 is ~3.5× faster serially and allocation-free.** In later isolated runs the v2 figure settled at
51.0 – 53.9 ns, so the ratio above is the conservative one.

### Scraping (nine series: three methods × three status codes)

Nine series render as 108 bucket lines plus nine `_sum`, nine `_count` and nine counter lines.

| Stage | ns/op (median) | spread | B/op | allocs/op |
|---|---|---|---|---|
| v1 `Gather()` + `expfmt` encode | 48 361 | 46 093 – 49 183 | 52 368 | **436** |
| v2, first cut — same run as the row above | **11 414** | 8 811 – 11 872 | 13 992 | **6** |
| v2, after buffer pre-sizing + `slices.SortFunc` | **6 171** | 6 096 – 6 271 | 13 776 | **3** |

The first two rows are one run of one binary and are the sound comparison: **4.2× faster, 73× fewer
allocations.** The third row was measured after two follow-up changes, in the same binary but a
later run, and is included because it is what actually ships — against the v1 figure above it is
7.8×, a ratio that spans runs and should be read as order-of-magnitude only.

### The shipped implementation

From `pkg/metrics/metrics_bench_test.go`, reproducible today (`-benchtime=1s -count=1`):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `Observe` | 51.1 | 0 | 0 |
| `ObserveParallel` | 92.1 | 0 | 0 |
| `ObserveDistinctSeries` | 51.1 | 0 | 0 |
| `Exposition` (buffer reused) | 3 904 | 1 488 | 2 |
| `Middleware` (end to end, discarding writer) | 108.7 | 32 | 1 |

`Exposition` is cheaper than the table above because the caller reuses a 16 KB buffer, so the render
never grows one — which is what `Handler` cannot assume and why its figure is the higher one.
`Middleware`'s single allocation is the `&statusWriter{}` escaping to the heap when it is passed as
an `http.ResponseWriter`; v1 paid the same one, plus the status string.

## Interpretation

**The one v1 allocation was `strconv.Itoa`, and this was isolated rather than inferred.** Feeding v1
a pre-rendered status string drops it to 0 allocs/op, which proves `WithLabelValues` itself does not
allocate — client_golang has a fast path that hashes label values without building a slice. So the
3 B/op was the three-character `"200"`, minted fresh on every request to be used as a label value.

v2 avoids it by construction: the series map is keyed on `struct{method string; code int}`, so the
hot path never formats anything, and the string form is rendered **once per series**, when the series
is created. That is the entire mechanism behind 1 → 0 allocations, and it is the kind of saving that
only appears when you own the storage — with the SDK, the label value *had* to be a string.

**v2 is faster even with that allocation removed** (190.7 → 63.4 ns), which is unsurprising once the
work is enumerated: v1 walks a label-hashing path, a `sync.Map`-ish metric lookup, and two separate
collectors (counter and histogram, each with its own lookup), where v2 does one map lookup and three
atomic adds, and backs both families with a single count.

**The parallel case is where the advantage narrows — 1.5× rather than 3.5× — and it is worth being
precise about why.** Every goroutine records the same label pair, so all of them contend on the same
three atomics in the same cache line, and v2 goes from 63 ns serial to 104 ns parallel. v1 goes the
other way (223 → 156 ns), which suggests its extra per-call work overlaps better across cores than a
tight contended increment does.

This was **not** tuned. Cache-line padding or per-P sharding would be the next step, and ADR-0038's
rule — measure before sharding — says the trigger has not been met: 104 ns is two orders of magnitude
below the cost of the HTTP request it is measuring, so the contention is invisible in any realistic
workload. Recorded here so a future reader has the number rather than a hunch.

**Scrape allocation churn was the largest single ratio in the report** (436 → 3, and 52 KB → 14 KB).
The v1 path builds a full protobuf `MetricFamily` tree in `Gather()`, then re-walks it in the text
encoder; v2 appends bytes into one buffer. The two follow-up optimizations were both found *by*
measuring rather than guessed: the 9-allocation figure was a doubling ladder (fixed by sizing the
buffer from the series count), and the residual 3 included `sort.Slice`'s reflection-based swapper
(fixed by `slices.SortFunc`, which the 1.25 floor makes available).

**Allocation counts carry the durable claims here.** They are deterministic and reproduced exactly
across every run; the ns figures moved by up to 40% between identical runs on this box, which is why
ratios are reported and the environment is recorded (ADR-0037).

## Method notes

- **Both implementations in one binary** is the only sound comparison on this hardware — ADR-0037
  established that, and 13.2 confirmed it when an in-repo run put `WithStack` at 1018–2258 ns
  against 626–669 in a controlled same-binary run.
- **`sink` is a package-level variable** so the compiler cannot delete the exposition render: a
  discarded result benchmarks as zero allocations, which nearly published a false zero once already.
- **`httptest.NewRecorder` is not used per iteration** in `BenchmarkMiddleware`; its allocations
  would be attributed to the code under test. A discarding `http.ResponseWriter` is used instead.
- The v1 comparison harness (`zz_compare_bench_test.go`) was deliberately **not** committed: keeping
  it would have kept `client_golang` in `go.mod`, which is the thing 13.6 removes.
