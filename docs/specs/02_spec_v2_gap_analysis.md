# Spec v2.0 Gap Analysis — `.spec/` draft vs the implemented v1.0.0

| | |
|---|---|
| **Date** | 2026-07-15 |
| **Status** | Informational — records differences; adopts nothing |
| **Compared** | `docs/specs/01_spec_utils.md` (frozen at intake, built against) **vs** `.spec/d4np-go.md` v2.0 ("Reviewed draft", 2026-07-14, addresses spec-review issue #8) + its ADR-001/002/003 |
| **Implementation** | `v1.0.0` (tag `962883e`) — all 25 features of the frozen spec, Spec Coverage Map ✅ |

## 1. Context — why two specs exist

The repository was generated and built against the spec **frozen at intake**
(`docs/specs/01_spec_utils.md`, derived from the v1 brief). A **revised spec v2.0** — produced by a
spec review two days after intake — exists at `.spec/` but is **untracked in git** and never entered
the governed tree, so no PR, ADR, or lint ever saw it. Every implementation decision that diverges
from v2 is nonetheless documented in this repo's own ADRs with alternatives considered; they were
judgment calls made without knowledge of v2, not oversights.

Verdict in two frames:

- **Against the frozen spec: fully conformant.** All 25 features, §5 signatures verbatim, quality
  gates green (that is what v1.0.0 certifies).
- **Against spec v2.0: substantial deltas** — itemized below. Several v2 corrections contradict the
  shipped v1.0.0 API, which now carries a SemVer stability commitment: those are only reachable via
  a `/v2` major. Others are additive or behavioral and adoptable in v1.x.

Legend — **Conformant** ✅ · **Convergent** 🟢 (we independently made v2's correction) ·
**Additive gap** 🟡 (adoptable in v1.x, non-breaking) · **Behavioral delta** 🟠 (same API, different
behavior/defaults) · **Breaking delta** 🔴 (contradicts the shipped API; needs `/v2`).

## 2. The 25 functional items

| # | Item | Spec v2.0 requires | Implemented (v1.0.0) | Delta |
|---|------|--------------------|----------------------|-------|
| 1 | workerpool | `Submit(ctx, task)` block/fail-fast; **`Close() error`**; `ErrQueueFull`, `ErrClosed`; task funcs receive ctx | Same semantics; **`Stop(ctx) error`**; `ErrQueueFull`, `ErrPoolClosed`; `Task func(ctx)` ✅ | 🔴 method name (`Stop`→`Close`); sentinel name differs trivially |
| 2 | pubsub | `Subscribe(ctx, filter)` auto-removed on ctx cancel; `Publish(ctx, T) error`; `ErrSlowSubscriber`; explicit slow-subscriber policy (bounded buffer, **drop-oldest or disconnect**) | Topic-based `Subscribe(topic, filter) (<-chan T, func())`; `Publish(topic, msg)` (no error); bounded buffer ✅ (`WithSubscriberBuffer`) with **drop-newest** observable via `WithDropHandler`; no sentinel | 🔴 API shape · 🟠 drop policy |
| 3 | fanin | Terminate on all-inputs-closed **or** ctx cancel; no orphan forwarders | Exactly this (goleak-verified) | ✅ |
| 4 | fanout | Same cancellation contract | Exactly this | ✅ |
| 5 | semaphore | Wrap `golang.org/x/sync/semaphore` (v2 ADR-002) | Thin adapter over x/sync (ADR-0009) | ✅ 🟢 |
| 6 | circuitbreaker | States/thresholds/half-open probes configurable **and observable**; `ErrOpen` via `errors.Is` | Configurable ✅, `ErrOpen` ✅; `State()` observability explicitly deferred (ADR-0010) | 🟡 observability |
| 7 | retry | Exponential backoff with **full jitter**; last error **wrapped with attempt count** | **Proportional jitter**; last error **verbatim** (ADR-0011, deliberate) | 🟠 ×2 |
| 8 | ratelimit | **Wrap `x/time/rate`** (v2 ADR-002); add `Middleware()`, `ErrLimited` | **Hand-rolled** lazy token bucket — our ADR-0012 explicitly rejected x/time/rate (uninjectable clock); `Allow`/`Wait` only | 🟠 engine (internal — API matches, swap possible without break) · 🟡 `Middleware()`+`ErrLimited` |
| 9 | middleware.RequestID | Extract/generate into context | ✅ + sanitization beyond spec | ✅ |
| 10 | middleware.Logger | Latency/bytes via the §2.15 slog logger; NFR-01 ≤3 allocs/op | Takes `*slog.Logger` ✅; alloc budget never benchmarked | ✅ API · 🟡 NFR unverified |
| 11 | middleware.Recoverer | Panic → clean 500, server survives | ✅ (+ no stack to client, ErrAbortHandler passthrough) | ✅ |
| 12 | middleware.Cors | Robust configurable CORS | ✅ (deny-by-default + Fetch-spec guard) | ✅ |
| 13 | config | JSON/YAML/env; **struct validation via item 19**; parser is a **fuzz target** | JSON/YAML/env ✅; validation via a composable `Validator` interface (not wired to `validator.Struct`); no fuzz target | 🟡 wiring · 🟡 fuzzing |
| 14 | env | Safe-fallback env reads | ✅ | ✅ |
| 15 | logger.Structured | Built on `log/slog` (v2 ADR-001); opinionated JSON defaults (RFC 3339 UTC, source key) for Elastic/Loki | slog JSON handler ✅ (ADR-0019 ≡ v2 ADR-001 option A); slog default keys kept, source opt-in, no forced UTC | ✅ 🟢 core · 🟠 minor tuning |
| 16 | logger.Context | Fields through `context.Context`, slog attrs under the hood | ✅ (attrs accumulate copy-on-write; logger resolved from `slog.Default`) | ✅ |
| 17 | cache | `New(...)` starts janitor, `Close()` stops; **`Get` → `(V, bool)`** (absence not an error); sharded RWMutex; goleak test creating/closing **1 000 caches** | `NewInMemory` ✅ lifecycle; **`Get` → `(V, error)` + `ErrNotFound`**; single RWMutex (sharding deferred, ADR-0021); goleak per-test, no 1 000-cache test | 🔴 `Get` signature · 🟡 sharding (internal, benchmark-gated) · 🟡 mass-lifecycle test |
| 18 | db.Transaction | Rollback on error or panic, **re-panics after rollback** | Exactly this (+ `errors.Join` on rollback failure) | ✅ 🟢 |
| 19 | validator | Tag-based struct validation | ✅ | ✅ |
| 20 | hash | bcrypt (hashing, not "encryption"); **configurable cost, default 12** (min 10); `ErrPasswordTooLong` surfaced, no silent truncation; doc note recommending argon2id + migration path | bcrypt ✅; `ErrPasswordTooLong` surfaced ✅ 🟢; **fixed cost 10**, not configurable; argon2id only in ADR alternatives, no godoc migration note | 🟠 default cost · 🟡 configurability · 🟡 doc note |
| 21 | lifecycle | **`WaitForSignals(timeout, sigs...)`** with bounded deadline; **`Trigger()`**; hook errors `errors.Join` | `WaitForSignals(sigs...)` — **deliberately no timeout** (ADR-0025); `Shutdown(ctx)` covers programmatic use; `errors.Join` ✅ 🟢 | 🔴 signature · 🟡 `Trigger()` (additive alias) · 🟠 bounded-deadline philosophy |
| 22 | health | Core probe = `func(ctx) error`, no driver imports; **DB/Redis probes in `contrib/` submodules** | Core ✅ 🟢 exactly (`Check.Probe`); no `contrib/` submodules exist | ✅ core · 🟡 contrib |
| 23 | metrics | **No Prometheus SDK** — emit exposition text format directly (v2 ADR-003) | Imports `client_golang` **and exposes `prometheus.Registerer` in the public API** (our ADR-0004 pre-approved it; ADR-0027) | 🔴 SDK in API + dependency |
| 24 | syncpool | `sync.Pool` buffer reuse; NFR-05 zero allocs | ✅, NFR-05 **proved** (AllocsPerRun + bench) | ✅ 🟢 |
| 25 | errx | Package named **`errx`** (shadowing stdlib `errors` is called out as v1's mistake); stack capture **opt-in** — `WithStack(err)`, `StackTrace(err) []Frame`; `%w` preserves chain, not stacks | Package named **`errors`**; stack captured **implicitly** at first wrap; `StackTracer` interface returning `[]uintptr` | 🔴 name + stack semantics + accessor shape |

## 3. Architecture & dependency policy (v2 §3 / ADR-003)

| v2 requirement | Implemented | Delta |
|---|---|---|
| **Zero-dependency core** — stdlib + `golang.org/x/*` only | Core carries `gopkg.in/yaml.v3` (config) and `prometheus/client_golang` + `client_model` (metrics), per this repo's own ADR-0004 two-dep budget | 🔴/⚠️ see note below |
| L1/L2/L3 layered import graph | **De facto clean** — no production package imports another EGL package (verified) | ✅ substance |
| CI enforcement: `depguard` + `go mod graph` assertion job | Absent | 🟡 |
| `contrib/redishealth`, `contrib/pgxhealth` nested submodules (own `go.mod`, independent tags) | Absent | 🟡 |
| Module path `github.com/danielpolowork/d4np-go` | `github.com/danielPoloWork/egl-utils-go` — deliberate EGL-series rebrand at intake (ADR-0003) | ✅ recorded intake decision |

**Note on the zero-dep conflict — v2 contains an internal contradiction.** v2 item 13 requires YAML
config parsing, but the standard library and `golang.org/x` provide no YAML parser; v2's ADR-003
(stdlib + `x/` only) therefore cannot be satisfied together with item 13 as written. This repo's
ADR-0004 (a bounded two-dependency budget) was the pragmatic resolution for YAML. The **metrics**
half of the delta is different: v2 shows the SDK *is* avoidable (emit the exposition text format
directly), so `client_golang` — and especially `prometheus.Registerer` in the public API — is a
genuine divergence, not a forced one.

## 4. NFRs & verification (v2 §5, §7)

| Requirement | Status |
|---|---|
| NFR-01 middleware chain ≤1 µs / 0 allocs (non-logging), Logger ≤3 allocs | 🟡 never benchmarked |
| NFR-02 workerpool ≥1 M tasks/s, Submit p99 ≤2 µs | 🟡 never benchmarked against target |
| NFR-03 pubsub ≥500 k msgs/s ×10 subscribers | 🟡 never benchmarked against target |
| NFR-04 ratelimit ±1 % admission accuracy (10 s bursty) | 🟡 admission latency benched (~25 ns), accuracy never measured |
| NFR-05 BufferPool 0 allocs steady-state | ✅ **proved** (AllocsPerRun assertion + bench) |
| NFR-06 cache Get p99 ≤200 ns @1 M entries, 90/10, 8 goroutines | 🟡 ~28 ns Get-hit on small maps; the 1 M-entry p99 scenario never run |
| Methodology: `benchstat`, ≥10 runs, nightly CI, >10 % regression fails | 🟡 a CI benchmark job exists (M1) but has no thresholds, no benchstat, no nightly regression gate |
| Fuzzing: `FuzzConfigLoader`, `FuzzValidatorTags`, committed corpora, 10-min PR budget | 🟡 no fuzz targets exist |
| goleak in every package `TestMain` | ✅ substance — enforced **per-test** (`defer goleak.VerifyNone(t)`), which is stricter; the letter (TestMain) differs |
| Coverage gate ≥ 85 % | 🟠 policy here is ≥ 80 % (AGENTS.md §10) and it is not CI-enforced; **actual coverage ≈ 100 %** per package, so substantively exceeded |
| bcrypt cost-factor benchmark documented for deployers | 🟡 absent |
| `govulncheck` per PR · `-race` in CI · two most recent Go releases · SemVer | ✅ all in place |

## 5. Independent convergences worth noting

Without access to v2, this implementation independently arrived at several of its corrections:
**slog as the logging base** (ADR-0019 ≡ v2 ADR-001, option A, near-identical reasoning);
**wrapping `x/sync/semaphore`** (half of v2 ADR-002); bcrypt as *hashing* with
**`ErrPasswordTooLong` surfaced and no silent truncation** (v2 §2.20's exact correction);
**`errors.Join`** for lifecycle hook errors (v2 §4); **re-panic after rollback** in
`db.Transaction`; the fanin/fanout **cancel-or-close, no-orphan-forwarders** contract; health-core
probes as plain `func(ctx) error` with **no driver imports**; and the layered import graph, which
holds de facto (no cross-package imports) even though unenforced.

## 6. Adoption paths (informational — no decision recorded here)

- **Adoptable in v1.x (non-breaking):** `State()` observability (6), `Trigger()` (21, additive
  alias), `Middleware()`+`ErrLimited` (8), configurable bcrypt cost via an additive constructor +
  argon2id doc note (20), config↔validator wiring option (13), fuzz targets + corpora (13, 19),
  NFR benchmark suite + benchstat/nightly gate (§5), `depguard`+`go mod graph` CI jobs (§3),
  `contrib/` submodules (22), 1 000-cache goleak test and internal sharding if benchmarks justify
  (17), CI coverage gate at 85 %.
- **Requires a `/v2` major (breaks the v1.0.0 API-stability commitment):** `errors`→`errx` rename
  with opt-in `WithStack`/`[]Frame` (25), `cache.Get`→`(V, bool)` (17), `workerpool.Stop`→`Close`
  (1), the pubsub API reshape (2), removing `prometheus.Registerer` from the metrics API / dropping
  the SDK (23), `WaitForSignals(timeout, …)` (21).
- **Deliberate deviations that may simply be recorded as such** (each already argued in an ADR):
  proportional vs full jitter and verbatim last error (7, ADR-0011), hand-rolled bucket engine
  (8, ADR-0012 — the public API already matches v2's shape, so the engine could also be swapped
  invisibly), no-hidden-timeout shutdown (21, ADR-0025), lazy Get-enforced cache expiry (17,
  ADR-0021), yaml.v3 in core (v2-internal contradiction, §3 note above).

## 7. Root cause & recommendation

The divergence is a **spec-versioning breakdown**, not an implementation-discipline one: v2.0 was
authored after intake, stayed in an untracked folder, and no governed process could see it — while
AGENTS.md §7 ("never let spec and implementation drift") presumes the spec lives *inside* the
governed tree. Recommendation: whatever disposition is chosen, first **import the v2 draft into
`docs/specs/` under version control** so any future revision lands where the contract, the lint,
and the PR flow can see it.
