# Benchmarks

Reproducible performance measurements for `egl-utils-go`. Any performance claim in the
spec, README, or a PR must be backed by a benchmark report here and by co-located
`Benchmark*` functions in the relevant feature package (`go test -bench`, ADR-0003).
Numbers without a reproducible method are not evidence.

## Methodology

- **Harness:** `go build (go modules)` builds the bench target; run with `go test -bench=. -benchmem ./...`.
- **Environment:** record the machine (CPU, RAM, OS), the toolchain version, and the build
  configuration (release/optimized) with every result — a number without its environment is
  not comparable.
- **Discipline:** warm up, run multiple iterations, report a central tendency **and** spread
  (e.g. median + p99), and pin the commit SHA the run was taken at.
- **Regression gate:** the CI `benchmark` job runs the suite; a result is a regression only
  against a recorded baseline on comparable hardware (note when CI hardware is too noisy to
  gate and the run is informational).

## Results

One report per measured scenario, from [`template.md`](template.md). Keep the index newest-first.

| Date | Scenario | Version | Headline result | Report |
|------|----------|---------|-----------------|--------|
| 2026-07-28 | errx — the cost of implicit stack capture, and of removing it | v1.1.1+dev (roadmap 13.2, ADR-0046) | `Wrap` 550.1 → 37.1 ns and 3 → 1 allocs; v1 paid **276 ns even when it captured nothing** (an `errors.As` walk per wrap); symbolization is 6.5× the capture, which is why `[]Frame` resolves lazily | [report](2026-07-28-errx-opt-in-stacks.md) |
| 2026-07-26 | the NFR suite (NFR-01 … NFR-06) | v1.0.0+dev (roadmap 10.10, NFR-06 updated by 10.11) | NFR-03 met 12.7x, NFR-02 throughput 4.4x, NFR-04 exact; NFR-06 **fixed by sharding: 349.8 → 46.6 ns (7.5x)**; NFR-01 allocations **not met** (target unachievable), NFR-02/06 tails unmeasurable on Windows | [report](2026-07-26-nfr-suite.md) |
| 2026-07-26 | bcrypt cost sizing (hash / verify per work factor) | v1.0.0+dev (roadmap 10.5) | exact doubling per cost step: 55 ms at cost 10 → 887 ms at 14; verify costs the same as hash, so cost is a per-login CPU multiplier | [report](2026-07-26-hash-bcrypt-cost-sizing.md) |
| 2026-07-14 | ratelimit hot paths (Allow / funded Wait) | v0.1.0+dev (PR #16) | Allow ~25 ns/op, 0 allocs; funded Wait ~50 ns/op, 0 allocs | [report](2026-07-14-ratelimit-hot-paths.md) |
