# 2026-07-29 — 13.6: nine modules for two metrics, and the 37 families nobody chose

**`pkg/metrics` no longer depends on the Prometheus SDK.** It writes the text exposition format
directly, and `Prometheus(reg prometheus.Registerer)` becomes `metrics.New()` with `Middleware()`
and `Handler()` methods. ADR-0050 supersedes ADR-0027's surface, implementation and dependency pin,
and ring 3's membership in ADR-0004. Ledger item 23 discharged — the only Milestone 13 item that
changes the dependency graph.

## The ROADMAP's own wording was the narrower reading, and lost

This item's ROADMAP line says "remove the Prometheus SDK and `prometheus.Registerer` from the public
API", which an unexported registry would satisfy. The gap analysis says **"No Prometheus SDK — emit
exposition text format directly"**, and spec v2 §3 is blunter still: *"no package imports database
drivers, redis clients, or prometheus SDK."*

The gap analysis wins, on the tie-breaker that has now fired in four consecutive items: where the
ROADMAP's shorthand and the gap analysis disagree, the gap analysis is the contract (13.3's `New`,
13.4's `ErrClosed`), and where the gap analysis's *signature* and its own *gap column* disagree, the
gap column wins (13.4's `ctx`, 13.5's `topic`). Here the gap column reads **"SDK in API +
dependency"** — two objections, and hiding the type answers one.

## Three measurements taken before deciding anything

Each could have changed the answer, so none was assumed.

**The dependency was nine modules, not one.** `go mod why` showed that `beorn7/perks`,
`cespare/xxhash/v2`, `munnerz/goautoneg`, `prometheus/common`, `prometheus/procfs`,
`golang.org/x/sys` and `google.golang.org/protobuf` were each in the graph *solely* because
`pkg/metrics` imported `client_golang` — plus `client_model` as a direct test-only require. Nine of
the module's eighteen modules, for two metric families. `go.sum` went 50 lines → 24.

A bonus that fell out: `x/sys` left with `procfs`, which **retires ADR-0027's accepted
uncalled-advisory trade-off**. That section weighed keeping the Go floor against clearing
GO-2026-5024, and the module carrying it is simply no longer here. A trade-off resolved by deleting
its subject rather than by revisiting it.

**`promhttp.Handler()` was serving 37 metric families nobody had chosen.** This was the decision-relevant
one, and I nearly wrote the ADR without checking it. A scrape of a fresh process returns 29 `go_*`
runtime families, 6 `process_*` and 2 `promhttp_*` — and *none* of ours, since ours appear only once
traffic has been recorded. So "drop the SDK" was not a like-for-like swap; it silently deletes the
runtime instrumentation a consumer's dashboards may be built on. That is a maintainer decision, not
an implementation detail, and it went back as a question with the number attached.

The ruling was to accept the loss and document the composition path. The argument that settled it:
**the dependency was being imposed on 100% of consumers to serve the subset who scrape runtime
metrics.** After this, a consumer who wants them imports a Prometheus client themselves and mounts
its handler at a second path — Prometheus scrapes two paths as happily as one — which puts nine
modules in the builds that asked for them.

**The old surface had a wart worth closing.** `Prometheus(reg)` registered on the caller's registry
while `Handler()` served the *default* one, so `Prometheus(myReg)` + `Handler()` silently exposed the
wrong metrics. ADR-0027 recorded this as a consequence of frozen signatures rather than as something
anyone wanted.

## The new shape was already in the module

`metrics.New() *Recorder` with `Middleware()` and `Handler()` methods is **ADR-0031's shape**:
`(*ratelimit.Limiter).Middleware()` established "stateful object owning its counters, handing out a
decorator" in 10.4. Adopting it means the module has one idiom for this rather than two.

Turning the registry parameter into ownership closes the wart by construction — a Recorder's
middleware and endpoint are two faces of one object, so they cannot disagree about which registry is
being exposed. For the same reason **double install stops being an error**: two `New()` calls are two
independent recorders, so ADR-0027's `MustRegister` panic has nothing left to protect. That let a
test be deleted and replaced by one asserting independence, which is a better test of a better
property.

Package-level functions over hidden state would have made the migration one deleted argument. Rejected
on ADR-0025's evidence: module-level singletons in `lifecycle` needed an internal swap seam to be
testable and forbid parallel tests. `lifecycle` accepted that because the spec froze its signatures;
here the signature changes regardless, so the cost buys nothing.

## Proving the format without the library that defines it

This was the interesting engineering problem. Claiming "Prometheus text exposition format" is easy;
being *right* about label ordering, cumulative buckets and float spelling is not, and the obvious
oracle was about to be deleted.

So the oracle was run first. `testdata/exposition.golden` is `prometheus/common/expfmt`'s own output
for exactly these two families, captured while the SDK was still a dependency, over observations
chosen to hit the first bucket, several middles, exactly the top bound, the `+Inf` overflow, and the
normalized `other` method. An internal test drives the same observations through the new writer and
requires byte equality — **so the assertion is "this matches the reference implementation", and it
outlives the reference implementation.** If it fails, the writer drifted from the format, not from an
earlier version of itself.

Four properties came from that capture rather than from memory, and three of them I would have got
wrong:

- Families are emitted in **sorted name order**, so the histogram precedes the counter.
- Label names sort **alphabetically** — `code` before `method`, regardless of declaration order —
  with `le` appended **last**, outside the sort.
- Bucket counts are **cumulative**.
- Floats use the **shortest round-tripping** form, which is why `le="1"` and not `le="1.0"`, and why
  a sum prints as `29.999100000000002`.

And one behaviour that would have been a plausible mistake: **a family with no series is omitted
entirely, HELP and TYPE included.** A fresh Recorder therefore serves an empty body — a valid scrape
— where emitting zero-valued lines would report observations that never happened.

The golden passed on the first run, which I did not expect.

**Then CI failed it on exactly one of thirteen jobs**, and the failure was worth more than the pass.
`windows-2022` reported the byte comparison failing while Linux, macOS and the `-race` cell were all
green. The cause is not in the writer: the golden had `eol` unspecified, so what a checkout writes
depends on the machine's `core.autocrlf` — LF for mine, set to `input`, and **CRLF for a GitHub
Windows runner, set to `true`.** Git was rewriting the fixture in transit and the test was correctly
reporting that the bytes did not match.

Reproduced locally before fixing anything, by checking the file out the way the runner does:

```
git -c core.autocrlf=true checkout-index -f --prefix=... pkg/metrics/testdata/exposition.golden
→ CRLF count: 49
```

`.gitattributes` now pins `*.golden text eol=lf` — `eol=lf` rather than `-text`, so the fixture still
diffs readably when it legitimately changes — and the same command afterwards yields LF at the
identical 3 503 bytes.

**The general lesson, which is about goldens rather than about metrics: a fixture asserted
byte-for-byte is only as exact as its checkout.** The assertion and the version-control layer have to
agree, and the default does not guarantee that. Worth noting that this repository had **no
`.gitattributes` at all** until now, so every previous byte-sensitive fixture was relying on
contributors' Git configuration happening to match.

## What the implementation cost, and what it bought

Measured with both implementations compiled into **one binary** (ADR-0037's rule, confirmed by 13.2):

| | v1 SDK | v2 direct |
|---|---|---|
| record | 223.0 ns, **1 alloc** | 63.4 ns, **0 allocs** |
| record, contended on one label pair | 155.9 ns | 104.0 ns |
| nine-series scrape | 48.4 µs, **436 allocs**, 52 KB | 11.4 µs, 6 allocs, 14 KB |

**The single v1 allocation was isolated, not inferred.** Feeding v1 a pre-rendered status string
drops it to zero allocations — so `WithLabelValues` itself does not allocate, and the 3 B/op was the
three-character `"200"` minted on every request to serve as a label value. v2 avoids it by keying the
series map on `struct{method string; code int}` and rendering the string **once per series**. That is
the whole trick, and it is only available to code that owns its storage: with the SDK, the label value
*had* to be a string.

v2 is faster even with that allocation removed (190.7 → 63.4 ns), which stops being surprising once
the work is listed: v1 does two collector lookups plus label hashing, v2 does one map lookup and three
atomic adds, with a single counter backing both `http_requests_total` and the histogram's `_count` —
so those two can no longer drift apart at all.

**Two optimizations were found by measuring rather than guessed.** The first scrape figure was 9
allocations: a buffer-doubling ladder, fixed by sizing the render from the series count. The residual
3 included `sort.Slice`'s reflection-based swapper, fixed by `slices.SortFunc`. 436 → 3.

**The honest asterisk: the advantage narrows to 1.5× under contention** (63 → 104 ns as goroutines
pile onto one label pair's atomics, while v1 goes the *other* way, 223 → 156). Not tuned: ADR-0038's
rule is measure-before-sharding, and 104 ns is two orders of magnitude under the HTTP request it is
measuring. Recorded so the next reader has the number instead of a hunch.

## Where the design says no

**Recording is lock-free, and a per-series mutex was rejected on ADR-0038's evidence.** It would have
been simpler and would have given a scrape an exactly consistent snapshot — but in a real server most
requests share one label pair, so that mutex would serialise the entire hot path on one lock, which is
the mistake `cache` already paid 7.5× to learn.

The accepted cost is stated rather than discovered: **a scrape can observe a histogram mid-update.**
`client_golang` prevents this with a hot/cold buffer swap that is not reproduced here. Instead the
count is written **last**, so the skew is one-directional, bounded by in-flight requests, and the
invariant a consumer leans on — `+Inf` never below the reported count — always holds.

ADR-0027's cardinality guarantees are untouched, and the bound is now **our own arithmetic**: ten
method values × the 100–999 codes `net/http` admits = **at most 9 000 series**, whatever a client
sends. With the SDK that was an argument about label domains; now it is a bound on a map this module
owns.

## Verification, and a rule about verifying

Both enforcement layers were edited, as ADR-0035 requires: `depguard`'s
`prometheus-belongs-to-metrics` became **`no-prometheus-sdk` with no exception at all** — not
`metrics`, not tests — and `import_graph_lint.py` lost `client_golang` from `RUNTIME_DEPS` and
`client_model` from `TEST_ONLY_DEPS`.

Verifying the depguard change had a twist: with the SDK gone from `go.mod`, importing it fails to
*build*, so the usual deliberate violation is impossible. I proved the mechanism I had actually
changed — `files: ['$all']` with no `!$test` — using an importable package instead. **The first
attempt at that was inconclusive and looked like a refutation:** a probe rule aimed at `yaml.v3`
flagged `config.go` but no test file, which I nearly wrote up as "so `$all` does not cover tests". It
does; my `grep` had matched `yaml:"addr"` **struct tags**, and no test file imported the parser at
all. A temporary test file with a real import settled it in one run.

Two lessons, one old and one new. The old one: use a **real** import, never a blank one — ADR-0035
documents that depguard is silent on `_` imports. The new one: **a probe that finds nothing has not
disproved anything until you have checked that the probe could have fired.**

Everything else green: 100% coverage (including the series-creation race branch, pinned
deterministically by calling the creator twice rather than leaving it to the scheduler), all four
policy tools, `golangci-lint` 0 issues, `govulncheck` 0 called, surface 139 → 141.

`.golangci.yml`'s preamble also claimed "rules apply to production files only (`!$test`)", which was
**already** false before this item — `no-driver-sdks` has always used `$all`. Amended in place with
the verified behaviour.

## State

Milestone 13 is **6 of 10**. Next is 13.7 (`WaitForSignals(timeout, sigs...)`), which reopens
ADR-0025's deliberate "no hidden shutdown timeout" — a documented deviation kept on purpose, so
taking the v2 signature means overturning that reasoning explicitly rather than quietly. 13.9 still
waits for the `v2.0.0` tag (ADR-0040).

**Carried forward, not fixed:** `orchestrator/project.yaml` still describes the v1 API and the
two-entry dependency ring. That drift is not this item's — 13.2 through 13.5 left it too, and it
also predates M13 (an 80% coverage floor, the retired MAJOR-intent compatibility clause,
`errors.Wrap`). Fixing one line of it while its neighbours stay stale would make the file less
coherent, not more; it wants one sweep, scoped and verified like M11's language-floor sweep.
