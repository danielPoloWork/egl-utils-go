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
| —    | —        | —       | —               | —      |

_No benchmarks recorded yet._
