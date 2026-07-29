# ADR-0050: metrics without the SDK — the exposition format written directly, and ring 3 down to one entry

- **Status:** Accepted
- **Date:** 2026-07-29
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** ADR-0027 (the metrics design — superseded on the public surface and the
  implementation; its cardinality decisions are preserved and re-enforced here), ADR-0004 (runtime
  dependency policy — ring 3 loses its `prometheus/client_golang` entry, and this ADR *partially
  adopts* the alternative that ADR was written to reject), ADR-0035 (the two enforcement layers,
  both edited here), ADR-0031 (`(*Limiter).Middleware()` — the precedent for the new shape),
  ADR-0038 (why the hot path is lock-free), ADR-0037 (benchmark methodology), ADR-0030 §2 ledger
  item 23, ADR-0045 (the `/v2` boundary); spec §2 feature 23, §3, §5; ROADMAP 13.6

## Context

Ledger item 23 is the only entry in Milestone 13 that changes the dependency graph. The gap
analysis states the target as **"No Prometheus SDK — emit exposition text format directly"**, and
spec v2 §3 is more explicit still: *"no package imports database drivers, redis clients, or
prometheus SDK — metrics emits exposition format directly."*

The ROADMAP's own shorthand for this item is narrower — "remove the Prometheus SDK and
`prometheus.Registerer` from the public API" — which would be satisfied by hiding the SDK behind an
unexported registry. The gap analysis wins, on the tie-breaker established in 13.3 and applied
again in 13.4 and 13.5: where the ROADMAP's shorthand and the gap analysis disagree, the gap
analysis is the contract. Its gap column reads **"SDK in API + dependency"** — two objections, and
hiding the type answers only the first.

Three facts were measured before deciding anything, because each of them could have changed the
answer:

**The dependency is not one module, it is nine.** `pkg/metrics` was the only importer of
`client_golang`, and `go mod why` confirms that every one of `beorn7/perks`, `cespare/xxhash/v2`,
`munnerz/goautoneg`, `prometheus/common`, `prometheus/procfs`, `golang.org/x/sys` and
`google.golang.org/protobuf` was in the graph solely because of it — plus `client_model`, a direct
test-only requirement. Nine of the module's eighteen modules, for two metric families.

**The SDK was also supplying 37 metric families nobody had decided to ship.**
`metrics.Handler()` returned `promhttp.Handler()` over the *default* registry, which
`client_golang` pre-populates: a scrape of a freshly started process returns 29 `go_*` runtime
families, 6 `process_*` families and 2 `promhttp_*` families — and none of ours, since ours appear
only once traffic has been recorded. This was verified, not assumed.

**The surface had a latent wart.** `Prometheus(reg)` registered on the caller's registry while
`Handler()` served the default one, so the documented pairing was `Prometheus(DefaultRegisterer)` +
`Handler()` and any other registry silently exposed the wrong metrics. ADR-0027 recorded this as a
consequence of the frozen signatures rather than as a design anyone wanted.

## Decision

### 1 · The exposition text is written directly; the SDK leaves the module entirely

`pkg/metrics` now formats Prometheus text exposition format version 0.0.4 itself. Nine modules
leave `go.mod` and `go.sum` drops from 50 lines to 24. **ADR-0004's ring 3 goes from two entries to
one** (`gopkg.in/yaml.v3`), and the `depguard` rule that used to confine `github.com/prometheus` to
one package now bans it outright, with no exception for `metrics` and none for tests.

What replaces the SDK is deliberately small — a counter and a histogram over the standard bucket
ladder, keyed by the two labels ADR-0027 allows, and a writer. It is emphatically **not** a metrics
library: there is no registry to register into, no gauge, no summary, no custom collector.

### 2 · `New() *Recorder`, with `Middleware()` and `Handler()` as methods

```go
rec := metrics.New()
mux.Handle("/metrics", rec.Handler())
h := rec.Middleware()(appHandler)
```

`Prometheus(reg prometheus.Registerer)` and the package-level `Handler()` are both gone.

**This is ADR-0031's shape, not a new one.** `(*Limiter).Middleware()` already established
"stateful object owning its counters, handing out a decorator" in this module, so `metrics` adopting
it means one idiom rather than two.

**The registry parameter becomes ownership, and that closes the wart.** A `Recorder`'s middleware
and its handler are the same object's two faces, so the endpoint cannot expose a different registry
than the one being written to — the failure mode is removed by construction rather than documented.
For the same reason **a double install is no longer an error to guard**: two `New()` calls are two
independent recorders, so ADR-0027's `MustRegister` panic has nothing left to protect and is gone.
The nil-registerer panic goes with the parameter; the nil-handler panic stays (ADR-0005).

Package-level functions over hidden state were rejected. They would have made the migration one
deleted argument, but ADR-0025 already paid for module-level singletons in `lifecycle`: they need
an internal swap seam to be testable, forbid parallel tests, and cannot serve two independent
servers in one process. `lifecycle` accepted that because the spec froze its signatures; here the
signature has to change anyway, so there is nothing to buy with it.

### 3 · The 37 free families are given up, and the composition path is documented instead

After this change `/metrics` serves the two families this package records and nothing else: no
`go_*`, no `process_*`, no `promhttp_*`, and no Accept negotiation — text format 0.0.4
unconditionally, with no protobuf or OpenMetrics variant and no exemplars.

This is the honest reading of "no SDK", and it moves a cost to where it belongs. The dependency was
being imposed on **every** consumer of `egl-utils-go` to serve the subset who scrape runtime
metrics. A consumer who wants them now imports a Prometheus client library themselves and mounts
its handler at a second path — Prometheus scrapes two paths as happily as one — which puts the nine
modules in the build that actually asked for them.

A `WithRuntimeMetrics()` option over the standard library's `runtime/metrics` would be **additive,
and therefore free to defer** (the rule 13.2 used to drop `Frame.String()`). Hand-rolling a subset
now would mean choosing which of 29 families matter, which is how a bounded HTTP instrumentation
package turns into the metrics library ADR-0004 exists to prevent.

### 4 · Conformance is proved against the reference encoder, once, and the evidence is committed

`testdata/exposition.golden` is **not** a record of what this code produces. It is the output of
`prometheus/client_golang` + `prometheus/common/expfmt` for the exact two families, captured while
that SDK was still a dependency, over observations chosen to exercise the first bucket, several
middle ones, exactly the top bound, the `+Inf` overflow, and the normalized `other` method. An
internal test drives the same observations through this package and requires byte equality.

So the assertion is "this matches what the reference implementation emits", and it survives the
implementation's removal. If it ever fails, the writer has drifted from the format — not from an
earlier version of itself. Four format properties were pinned this way rather than from memory:
families are emitted in sorted name order (so the histogram precedes the counter); label names sort
alphabetically with `le` appended last; bucket counts are cumulative; and floats render in the
shortest round-tripping form, which is what makes `1` render as `1` and not `1.0`.

**A family with no series is omitted entirely, HELP and TYPE included**, matching the reference
encoder — so a `Recorder` that has seen no traffic serves an empty body. That is a valid scrape,
where emitting zero-valued lines would report observations that never happened.

Keeping a Prometheus module as a test-only dependency to parse the output on every run was
considered and rejected: it would hold four or five modules in the graph to re-verify a frozen
format, and the golden *is* the contract.

### 5 · ADR-0027's cardinality guarantees survive, and are now bounded by our own arithmetic

Unchanged: two families, `(method, code)` labels only, **the request path is never a label**, the
method normalized to the nine known verbs plus `other`, `DefBuckets` reproduced verbatim, and the
`Unwrap`-aware status recorder.

Two things are sharper than before. The normalizer now maps each method to *this package's* own
constant instead of returning the caller's string, so a label value never references a request
buffer. And since the method label has ten values and `net/http` admits only status codes 100–999,
**a `Recorder` holds at most 9 000 series whatever traffic it is shown** — with the SDK, the bound
was an argument about label domains; now it is a bound on a map this module owns and can state.

### 6 · Recording is lock-free; a scrape is consistent enough, and says so

Each series is a set of atomics. The `RWMutex` is taken only to find or create a series, and
recording holds nothing.

**A per-series mutex was rejected on ADR-0038's evidence.** It would have been simpler and would
have given a scrape an exactly consistent snapshot, but in a real server most requests share one
label pair, so that mutex would serialise the whole hot path on a single lock — the mistake cache
already paid 7.5× to learn.

The accepted cost, stated rather than discovered: **a concurrent scrape can observe a histogram
mid-update.** `client_golang` avoids this with a hot/cold buffer swap; this package does not
reproduce that machinery. Instead the three updates are ordered so the count is written **last**,
which makes the skew one-directional and bounded by the number of in-flight requests, and preserves
the invariant a consumer is most likely to lean on: the `+Inf` bucket is never smaller than the
reported count. Because one atomic now backs both `http_requests_total` and the histogram's
`_count`, those two can no longer drift apart at all, where two independent collectors could.

## Alternatives Considered

- **Hide the SDK behind an unexported registry.** Satisfies the ROADMAP's wording and nothing else:
  the nine modules stay, spec v2 §3 stays violated, and ledger item 23 is left half-discharged
  inside the major that exists to empty it — the same trap 13.4 avoided by folding in `ErrClosed`.
- **Hand-roll a subset of the `go_*` families** from `runtime/metrics` to keep dashboards alive.
  Rejected: it requires picking winners among 29 families and grows the package's remit from
  "instrument HTTP handlers" to "provide metrics", which is exactly what ADR-0004 bounds. Additive
  later if anyone asks.
- **Keep package-level `Prometheus()` + `Handler()` over hidden state.** Rejected: see Decision 2 —
  ADR-0025's singleton costs, bought with nothing, since the signature changes regardless.
- **A test-only Prometheus dependency for ongoing format validation.** Rejected: see Decision 4.
- **Negotiate `Accept` and support OpenMetrics/protobuf.** Rejected: reimplementing content
  negotiation and a second encoding to serve scrapers that all accept text 0.0.4 anyway.
- **Expose a `WriteTo(io.Writer)` so a consumer can merge our output into their own endpoint.**
  Attractive, and the exposition format does concatenate cleanly across disjoint families.
  Rejected *for now* purely because it is additive: a second scrape path already solves the same
  problem, and surface added at a major cannot be withdrawn before the next one.

## Consequences

- **Breaking, and it is the largest migration in Milestone 13** because the call site changes
  shape rather than spelling:
  `metrics.Prometheus(prometheus.DefaultRegisterer)(h)` → `rec := metrics.New()` then
  `rec.Middleware()(h)`; `metrics.Handler()` → `rec.Handler()`. A consumer who was passing a custom
  registry no longer has one to pass, and a consumer relying on `go_*`/`process_*` metrics must
  mount a Prometheus handler themselves.
- **The dependency graph halves: 18 modules → 9**, direct requires 8 → 6, `go.sum` 50 → 24 lines.
  `golang.org/x/sys` leaves with `procfs`, which **retires ADR-0027's accepted-uncalled-advisory
  trade-off** — that section weighed keeping a Go floor against clearing GO-2026-5024, and the
  module carrying it is simply no longer here.
- **Faster and allocation-free on the request path, measured with both implementations in one
  binary and in the same run** (ADR-0037, and 13.2's method): recording **223.0 → 63.4 ns/op and
  1 alloc → 0**; a nine-series scrape **48.4 → 11.4 µs, 436 allocs → 6, 52.4 → 14.0 KB**. Two
  follow-up optimizations then took the scrape to **6.2 µs and 3 allocs** — sizing the render buffer
  from the series count, and `slices.SortFunc` in place of `sort.Slice`, whose reflection-based
  swapper allocates on every scrape. **The allocation counts carry the durable claim**; they are
  deterministic and reproduce exactly, where the ns figures move tens of percent on this box.
  The single v1 allocation was isolated to `strconv.Itoa(status)` — pre-rendering the code in v1
  removes it, and v1 is still ~3× slower without it — which is why keying the series on an `int` and
  rendering the string once per series is the whole trick. Report:
  `docs/benchmarks/2026-07-29-metrics-without-the-sdk.md`.
- **The advantage narrows under contention** (155.9 → 104.0 ns parallel on one shared label pair)
  because every goroutine touches the same three atomics. Recorded rather than tuned: 104 ns is far
  below any plausible per-request cost, and ADR-0038's rule is to measure before sharding.
- **Both enforcement layers were edited, as ADR-0035 requires**: `depguard` gains
  `no-prometheus-sdk` (which replaces `prometheus-belongs-to-metrics` and carries no exception), and
  `import_graph_lint.py` drops `client_golang` from `RUNTIME_DEPS` and `client_model` from
  `TEST_ONLY_DEPS`. The `$all`-without-`!$test` mechanism was verified by deliberate violation with
  a real (not blank) import, since ADR-0035 records that depguard is silent on blank imports.
- **100% statement coverage retained**, including the series-creation race branch — pinned
  deterministically by calling the creator twice rather than left to the scheduler.
- **Spec §5's metrics line and §3's dependency sentence are both amended**, the latter losing both
  Prometheus modules; `spec_api_lint.py` keeps the surface honest.

## References

- `pkg/metrics/metrics.go`, `pkg/metrics/exposition.go`, `pkg/metrics/testdata/exposition.golden`,
  and the three test files; `.golangci.yml`; `tools/import_graph_lint.py`.
- ADR-0027 — the design this replaces; ADR-0004 — the ring policy, with the intake alternative this
  partially adopts; ADR-0031 — the shape; ADR-0038 — why recording is lock-free.
- ADR-0030 §2 — the ledger, item 23 discharged; `docs/specs/02_spec_v2_gap_analysis.md` row 23.
