# ADR-0057: The additive-capability ledger — a deferred capability is an entry with a trigger, not a backlog item

- **Status:** Accepted
- **Date:** 2026-08-08
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** [ADR-0030](0030-spec-v2-reconciliation.md) §2 (the breaking-delta ledger this mirrors);
  [ADR-0042](0042-post-1.0-compatibility-contract.md) (which names ADR-0030 §2 as the destination for
  a breaking change); ROADMAP 14.10; `CONTRIBUTING.md` §7 (the intake this ledger receives from);
  every ADR cited in §A and §B below

## Context

Since ADR-0019 this project has ended design records with a line naming what it deliberately did not
build. The convention is stable enough to grep — thirteen ADRs write `Deferred, additive` or
`Deferred/additive` in almost those words — and it has been doing real work: each of those lines is
an argument that the capability is **reachable later without a break**, which is what made it safe to
omit at the time.

The problem is that the lines are invisible. They sit in the Consequences section of a document
nobody re-reads once its decision has shipped, and there is no index. Milestone 14's preamble
estimated "roughly twenty" of them across "sixteen ADRs". **The census run for this item found
neither number to be right, and the gap matters in both directions.**

- **The corpus is larger than believed.** Counting distinct capabilities rather than lines, and
  following the restatements across superseding ADRs, §A below holds **49 open capabilities drawn
  from 26 ADRs**, plus one entry that has no ADR at all. Thirteen ADRs use the greppable marker; the
  rest say the same thing in prose that no pattern finds (`additive later`, `free to defer`,
  `a possible additive later`, `Deferred: add via a spec amendment when a consumer needs it`).
- **Some of it has already shipped.** Eleven deferrals in §C were discharged between v1.0.0 and
  v2.0.1 and the deferring ADR was never updated to say so. `cache`'s "sharding" was deferred by
  ADR-0021 and built in 10.11 under ADR-0038. `hash`'s "configurable cost" and "cost-upgrade helper"
  were deferred by ADR-0024 and both shipped in 10.5. ADR-0029 deferred "a bespoke frame type" and
  `errx.Frame` has existed since 13.2. **A ledger assembled by grepping for the word `deferred` and
  transcribing what it finds would therefore have proposed, as future work, capability the module
  already exports.** That is the specific failure this ADR's status discipline exists to prevent, and
  it was found by checking each entry against the source rather than against the ADR that deferred it.
- **Not everything called additive is additive.** Two entries do not survive the test: making `*Pool`
  satisfy `io.Closer` requires a `Close` without a context (ADR-0048 states the cost plainly), and
  making argon2id the *default* is a documented-behaviour change of exactly the kind ADR-0052 held to
  a major. Both are reclassified in §D.

ADR-0030 §2 proved the shape works. A table of deltas with a destination turned "should we break
this?" from an argument into a schedule, and it is empty today because it was **discharged** — seven
items, seven PRs, seven ADRs — not because it was abandoned. That precedent is why this ledger exists
at all.

But ADR-0030 §2 had a property this ledger does not. Its entries were already *decided*: each was a
breaking change the spec had asked for, waiting only on a major to be opened. Every entry below is
the opposite — a capability that was argued against on the merits, and whose ADR said so. Copying
ADR-0030's shape without that difference produces a backlog: a list of twenty-odd things "to do
eventually", which invites precisely the speculative surface growth each of those ADRs refused. The
milestone that contains this item adds no exported identifier for that reason.

## Decision

**The ledger is adopted with one column ADR-0030 did not need: the trigger — the evidence that would
move an entry into a milestone. An entry without a trigger is not accepted into the ledger, because
an entry without a trigger is a backlog item.**

Six decisions follow.

### 1 · The ledger lives in this ADR, mirroring ADR-0030 §2

§A and §B below are the ledger. They are amended in place with dated notes, the convention ADR-0003
and ADR-0041 established for this repository, rather than rewritten.

A separate living document under `docs/` was considered and rejected in the alternatives. The
deciding argument is symmetry with a mechanism that already works: ADR-0042 names ADR-0030 §2 as the
single destination for a breaking change, so a contributor asking "where does a deferred change go?"
should find exactly two answers — §2 if it breaks, here if it does not — and not a third kind of
artifact with its own index, its own conventions and its own way to drift.

### 2 · Every entry carries a trigger, and a trigger names evidence

A trigger is falsifiable. It says what would have to be observed, by whom, before the entry is
scheduled — not "when it becomes important". Six kinds are in use, and the kind is recorded because
it says who is expected to report it:

| Kind | Fires when | Who reports it |
|---|---|---|
| **consumer** | A report naming the behaviour, the workaround in use today, and its cost — `CONTRIBUTING.md` §7's four questions | Someone outside the repository |
| **second-consumer** | The same need arrives independently a second time | The maintainer, on noticing a repeat |
| **in-repo** | One of the module's own consumers needs it — `examples/service`, a `contrib/*` module, or the test suite | Whoever writes that consumer |
| **measurement** | A number crosses a stated threshold | The benchmark suite or a policy tool |
| **external** | The standard library, the Go toolchain or a dependency changes such that the original objection dissolves | Dependabot, a release note, a toolchain bump |
| **paired** | The work is free only alongside something else already scheduled — most often a major | The roadmap |

The **in-repo** kind is not a theoretical category. It has already fired once: writing
`examples/service` in 14.5 needed a rate limit as a float and `env` has `GetDefault`, `GetInt`,
`GetBool` and `GetDuration` and no float getter, so the example converts an `int` at the call site.
Composing the module surfaced a gap that reading its surface had not, which is the argument for
keeping the module's own consumers in the trigger taxonomy rather than waiting only on the outside
world.

### 3 · Two ledgers, because the trigger means two different things

§A holds capabilities that would change the **public surface**. §B holds deferred **internal and
process** work — a fuzz target, a coverage mode, a CI gate.

They are separated rather than merged because the trigger column would otherwise mean two
incompatible things. A surface capability waits on *demand*, and demand arrives from outside; the
whole point of §A is that nobody inside the repository may declare it. An internal deferral waits on
a *repository event* the maintainer observes alone, and no consumer will ever report it. One table
holding both would license the reading that the maintainer can fire a §A trigger by wanting to.

### 4 · An entry must name the shape that makes it additive

If the entry cannot say what signature, option or new identifier delivers the capability **without
breaking an existing one**, it does not belong here — it belongs in ADR-0030 §2. §D records the two
entries reclassified on this rule when the census was taken, and both were genuinely believed
additive until the shape was written down.

This is the test that keeps the ledger honest. "argon2id" is not an entry; "an additive
`HashPasswordArgon2` alongside bcrypt" is one, and "argon2id becomes what `HashPassword` uses" is a
§2 entry, because it is the bcrypt-cost decision again (ADR-0052) with a different constant.

### 5 · Status is recorded, and discharge is recorded where the entry lives

An entry is **Open**, **Discharged** (with the roadmap item and ADR that did it, per §C),
**Reclassified** (moved to ADR-0030 §2, per §D), or **Withdrawn** (the objection hardened into a
permanent no — none yet).

Discharge is written into §C in the PR that ships the capability. The eleven entries in §C were
reconstructed from the roadmap and the source rather than from the deferring ADRs, because none of
those ADRs had been updated — which is the drift this rule exists to stop.

### 6 · A new deferral is added to the ledger in the PR that defers it, and marked so a gate can see it

An ADR that defers a capability adds its §A or §B row in the same pull request, and writes the
deferral with the canonical marker **`Deferred, additive:`**.

`consistency_lint.py` gains an eleventh check, `ledger-coverage`, asserting both directions between
the marker and this ADR: every ADR carrying the marker is cited by the ledger, and every ADR the
ledger cites exists. It is a check in the existing required tool rather than a fifth policy tool, for
the reason ADR-0056 recorded — `consistency / lint` is already a required status context, and adding
a job does not add one.

**What that gate cannot see is stated rather than discovered.** It keys on the marker, which 13 of
the 26 ADRs cited in §A use; the other 13 express the same deferral in prose no pattern matches
without false positives (`a deferred recover()`, "Go's `defer` intuition" and "deferring a
nil-pointer dereference" all appear in these documents and none is a deferral). Retrofitting the
marker into 13 historical ADRs to satisfy a regex was rejected: it edits accepted records to please a
checker, and the ledger — not the marker — is the artifact of record. The marker is a tripwire for
**new** deferrals, where it costs nothing, and §A is complete by census regardless.

## §A · The ledger — public surface

Every row is **Open**. Sorted by package. "Source" is the ADR that deferred it; where a later ADR
restated the deferral, both are cited and the later one governs.

### cache

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| `SetWithTTL(key, value, ttl)` — a per-entry TTL overriding the cache-wide one | [0021](0021-cache-inmemory-design.md) | A consumer with two lifetimes in one cache, who today runs two `Cache` values and reports what that costs | consumer |
| `Len()` — the live entry count | [0021](0021-cache-inmemory-design.md) | A consumer needing capacity signals; note the count is inherently racy under the sweeper, so the report must say what it would do with the number | consumer |

### config

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| Field-level env overlay by struct tag (`env:"ADDR"`) | [0018](0018-config-loader-design.md) | A consumer for whom `${VAR}` expansion is insufficient — one who must override a field with no placeholder in the file | consumer |
| Strict-missing-env (fail rather than expand to empty) | [0018](0018-config-loader-design.md) | A report of a production incident caused by an unset variable expanding silently | consumer |
| A general `WithValidator` / `WithValidate(func(T) error)` hook | [0018](0018-config-loader-design.md), [0033](0033-config-struct-validation.md) | A consumer whose validation cannot be expressed in `validator` tags, naming the rule | consumer |
| Normalise-then-validate ordering | [0033](0033-config-struct-validation.md) | A consumer who must canonicalise a field before it can pass its own rule | consumer |

### db

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| A `TxOptions` / isolation-level variant of `Transaction` | [0022](0022-db-transaction-design.md) | A consumer needing an isolation level other than the driver default, stating which and why | consumer |
| A generic `Transaction[T]` returning a value | [0022](0022-db-transaction-design.md) | Two independent reports of the closure-capture workaround; one is a style preference, two is a gap | second-consumer |

### env

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| A float getter (`GetFloat(key string, fallback float64) float64`) | *none — 14.5* | **Already fired.** `examples/service` converts an `int` at the call site to reach `ratelimit.NewLimiter(float64, int)`. The only entry in this table whose trigger has been observed | in-repo |

### errx

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| A `Cause` / root-error accessor | [0029](0029-errors-wrap-design.md) | A consumer for whom repeated `errors.Unwrap` is insufficient; note `errors.Is`/`As` already answer most of what a root accessor is asked for | consumer |
| Configurable stack depth | [0029](0029-errors-wrap-design.md) | A measurement: a report that the fixed depth truncates a real trace, or that capture cost matters at a stated rate | measurement |
| A raw-PC accessor (`PCs()`) for tracing exporters | [0046](0046-errx-opt-in-stacks.md) | An exporter integration — OTel or Sentry — that needs counters rather than resolved frames | consumer |
| `Frame.String()` | [0046](0046-errx-opt-in-stacks.md) | Two consumers formatting frames the same way; `Frame`'s fields are exported, so the workaround is one line | second-consumer |

### hash

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| An additive `HashPasswordArgon2` alongside bcrypt | [0024](0024-hash-password-design.md) | A consumer with a policy requirement naming argon2id, or a credible break in bcrypt. Note ADR-0004: `x/crypto` is already a dependency, so argon2id costs no new module | consumer / external |
| Exporting `MinCost` / `MaxCost` / `DefaultCost` | [0032](0032-hash-password-cost-design.md), [0052](0052-hash-default-cost-12.md) | A consumer pre-validating an operator-supplied cost. **ADR-0052 records this as the irreversible direction** — a published constant is a promise about a number that ADR expects to move again — so the trigger is a real need, never tidiness | consumer |

### health

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| A verbose / error-exposing mode, off by default | [0026](0026-health-handler-design.md) | An operator who cannot diagnose a failing probe from logs alone. Security-relevant (control C-2): the current handler deliberately names *which* check failed and never *why* | consumer |
| `Check.Timeout` — a per-probe deadline | [0026](0026-health-handler-design.md) | A consumer whose probe outlives the request context, with the measured latency | measurement |
| Method restriction (reject non-`GET`) | [0026](0026-health-handler-design.md) | A consumer whose infrastructure probes with another method and gets a misleading answer | consumer |

### lifecycle

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| An exported `Coordinator` | [0025](0025-lifecycle-shutdown-design.md) | A multi-coordinator scenario — two independent lifecycles in one process. The internal type already exists, so the cost is surface, not work | consumer |
| Force-exit on a second signal | [0025](0025-lifecycle-shutdown-design.md) | A consumer wanting impatient Ctrl+C. Would arrive as an option, never a default: `os.Exit` from library code bypasses every remaining hook | consumer |
| Per-hook shutdown budgets | [0025](0025-lifecycle-shutdown-design.md), [0051](0051-lifecycle-shutdown-timeout.md) | A consumer whose one slow hook consumes the whole `WaitForSignals` budget, with the hook named | consumer |
| A return value from `WaitForSignals` | [0051](0051-lifecycle-shutdown-timeout.md) | A consumer needing the shutdown outcome in-process rather than from the `slog.Default` Error line. Source-compatible for a statement call, hence additive | consumer |

### logger

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| `WithReplaceAttr` — a custom schema | [0019](0019-logger-structured-design.md) | A consumer whose collector requires field names the slog defaults do not produce | consumer |
| A text-handler variant | [0019](0019-logger-structured-design.md) | A consumer needing human-readable local output, who today builds their own `slog.Handler` | consumer |
| Sampling | [0019](0019-logger-structured-design.md) | A measurement: a stated log volume and its cost | measurement |
| A `FromContext` variant taking an explicit base logger | [0020](0020-logger-context-design.md) | A consumer who cannot use `slog.Default` as the base — typically two loggers in one process | consumer |
| A `Fields(ctx) []Field` accessor | [0020](0020-logger-context-design.md) | A consumer needing the attributes themselves rather than a logger carrying them | consumer |

### metrics

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| An in-flight-requests gauge | [0027](0027-metrics-prometheus-design.md) | A consumer who cannot derive concurrency from the existing counter and histogram | consumer |
| A response-size histogram | [0027](0027-metrics-prometheus-design.md) | A consumer sizing egress. Note ADR-0027's cardinality bound (at most 9 000 series) is the ceiling any new family must fit | consumer |
| Configurable histogram buckets | [0027](0027-metrics-prometheus-design.md) | A consumer whose latency profile the default buckets misrepresent, with the distribution attached | measurement |
| `WithRuntimeMetrics()` over `runtime/metrics` | [0050](0050-metrics-without-the-sdk.md) | A consumer who lost the 29 `go_*` families when 13.6 removed the SDK and says which they used. **Requires naming them** — picking winners among 29 is how a bounded HTTP instrumentation package becomes a metrics library | consumer |
| `WriteTo(io.Writer)` — merge our exposition into a consumer's endpoint | [0050](0050-metrics-without-the-sdk.md) | A consumer who cannot serve a second scrape path. The format concatenates cleanly across disjoint families, so the work is small; the surface is the cost | consumer |

### middleware

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| `Logger` options — fields, a level function, a clock | [0014](0014-middleware-logger-design.md) | A consumer wrapping `Logger(l)` to change one of these, and saying which | consumer |
| An additive `RecovererWithLogger` | [0016](0016-middleware-recoverer-design.md) | A consumer who cannot use `slog.Default` for panic reporting | consumer |
| `Recoverer` options — custom status, custom body, on-panic hook | [0016](0016-middleware-recoverer-design.md) | A consumer needing a panic response other than a bare 500, named. Security-relevant (control C-2): the current handler leaks nothing, and any custom body must preserve that | consumer |
| CORS subdomain / regex / suffix origin patterns | [0017](0017-middleware-cors-design.md) | A consumer with a genuine wildcard-subdomain deployment. **Security-relevant: a bad pattern is an open door**, so this arrives with its own threat-model row or not at all | consumer |
| CORS options-based construction | [0017](0017-middleware-cors-design.md) | A consumer whom the frozen `CorsConfig` cannot express | consumer |
| CORS preflight passthrough | [0017](0017-middleware-cors-design.md) | A consumer whose handler must see `OPTIONS` itself | consumer |

### pubsub

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| Per-subscription slow-subscriber policy | [0039](0039-pubsub-drop-oldest.md), [0049](0049-pubsub-reshape.md) | A consumer with two subscribers on one broker needing different policies. Today two brokers serve this; the objection is a per-subscriber branch on the hot fan-out path, so the report should include the fan-out rate | consumer / measurement |

### ratelimit

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| A deny hook and a configurable refusal status/body | [0031](0031-ratelimit-middleware-design.md) | A consumer for whom the four lines against `Allow` and `ErrLimited` are insufficient — typically one who needs the refusal *observed*, not just shaped | consumer |
| Per-client (per-IP, per-key) limiting inside the middleware | [0031](0031-ratelimit-middleware-design.md) | **The policy most consumers eventually want**, and the one with the largest hidden design: a fair per-key limiter needs a key-extraction policy and an eviction policy. Fires on a consumer who states both, not on the wish | consumer |

### resilience — circuitbreaker and retry

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| An error classifier (stop retrying / do not trip on a classified error, e.g. `context.Canceled`) | [0010](0010-circuitbreaker-design.md), [0011](0011-retry-design.md) | A consumer whose `fn`-wrapping workaround is insufficient. **Needs a spec amendment, not just an option** — both ADRs deferred it to one, so the entry is paired with a §5 change | consumer / paired |

### syncpool

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| A configurable retention cap (`NewBufferPool(maxCap int)`) | [0028](0028-syncpool-bufferpool-design.md) | A consumer whose working set the fixed 64 KiB cap misfits, with the measured buffer-size distribution | measurement |
| A generic `Pool[T]` | [0028](0028-syncpool-bufferpool-design.md) | A second reusable type in real use. The obstacle is that the reset/retention policy is type-specific — it hinges on `Cap`/`Reset` — so the entry needs the second type's policy, not just the type | second-consumer |
| Pools for other reusable types | [0028](0028-syncpool-bufferpool-design.md) | Same trigger as the generic pool, and the two are alternatives: whichever arrives first likely withdraws the other | second-consumer |

### validator

| Capability | Source | Trigger | Kind |
|---|---|---|---|
| `omitempty` — skip rules on a zero value | [0023](0023-validator-struct-design.md) | A consumer for whom pointer-for-optional is insufficient. The most-requested shape in comparable libraries, so expect this one first | consumer |
| `dive` — apply rules to slice/map elements | [0023](0023-validator-struct-design.md) | A consumer validating collection elements, who today writes a loop | consumer |
| Custom rules | [0023](0023-validator-struct-design.md) | A consumer whose rule cannot be expressed literally, naming it. Overlaps the `config` `WithValidator` entry — one may discharge both | consumer |
| JSON-tag field names in errors | [0023](0023-validator-struct-design.md) | A consumer surfacing validation errors to an API client, where the Go field name is the wrong vocabulary | consumer |

## §B · The ledger — internal and process deferrals

No consumer will report these. Each waits on a repository event the maintainer observes.

| Deferred work | Source | Trigger | Kind |
|---|---|---|---|
| Fuzz targets for further parsers of untrusted input | [0034](0034-fuzzing-strategy.md) | A new parser of untrusted input enters the module. There were none beyond config and validator at the time of writing, and there are none now | in-repo |
| Enforcing the test-only ring's *composition* — which test dependencies each package may use | [0035](0035-import-graph-enforcement.md) | A second test-only dependency, or a test import that the current ring check admits and should not | in-repo |
| Diff-scoped coverage | [0036](0036-coverage-gate.md) | A package where whole-package measurement stops being meaningful — in practice, one large enough that 85% hides an untested region | measurement |
| A same-runner A/B benchmark gate on PRs | [0037](0037-nfr-benchmark-methodology.md) | Benchmark noise defeating the nightly comparison often enough to be counted. **ADR-0037 forbids gating a tail**, so this can only ever gate a mean | measurement |
| A spec amendment for NFR-01's allocation target | [0037](0037-nfr-benchmark-methodology.md) | Maintainer decision; the measurement is already recorded | paired |
| A spec answer for what NFR-06's 200 ns target measures | [0037](0037-nfr-benchmark-methodology.md), 14.8 | **Maintainer decision, outstanding.** 14.8 measured `GetHit` uncontended at 32.9 ns (met) and the same `Get` under the 8-way mix at ~775 ns (not met); which one the target means is a spec question, and ADR-0030 §3 is the register | paired |
| CycloneDX SBOMs for `contrib/*` tags | [0055](0055-contrib-release-workflow.md), [0056](0056-build-time-supply-chain.md) | A record that outlives 90 days. Blocked by design, not effort: ADR-0055 gives contrib tags no GitHub Release, so there is nothing to attach an SBOM to, and a 90-day artifact is not a record | paired |

## §C · Discharged

Reconstructed from the roadmap and the source, because **not one of the deferring ADRs had been
updated to say the capability had arrived.** This is the state the ledger exists to make impossible.

| Capability | Deferred by | Discharged in | ADR |
|---|---|---|---|
| `cache` internal sharding | [0021](0021-cache-inmemory-design.md) | 10.11 | [0038](0038-cache-sharding.md) |
| A configurable bcrypt cost (`HashPasswordCost`) | [0024](0024-hash-password-design.md) | 10.5 | [0032](0032-hash-password-cost-design.md) |
| A cost-upgrade helper (`Cost` + rehash-on-login) | [0024](0024-hash-password-design.md) | 10.5 | [0032](0032-hash-password-cost-design.md) |
| A bespoke frame type (`errx.Frame`) | [0029](0029-errors-wrap-design.md) | 13.2 | [0046](0046-errx-opt-in-stacks.md) |
| `circuitbreaker.State()` | [0010](0010-circuitbreaker-design.md) | 10.2 | [0030](0030-spec-v2-reconciliation.md) §1 |
| Programmatic shutdown (`lifecycle.Trigger()`) | [0025](0025-lifecycle-shutdown-design.md) | 10.3 | [0030](0030-spec-v2-reconciliation.md) §1 |
| Config ↔ validator wiring (`WithStructValidation`) | [0018](0018-config-loader-design.md) | 10.6 | [0033](0033-config-struct-validation.md) |
| A slow-subscriber policy beyond drop-newest | [0039](0039-pubsub-drop-oldest.md) | 10.12, reshaped in 13.5 | [0049](0049-pubsub-reshape.md) |
| Re-measuring NFR-06 after 10.11 | [0037](0037-nfr-benchmark-methodology.md) | 14.8 | [0037](0037-nfr-benchmark-methodology.md), amended |
| `middleware.HeaderName` canonicalisation | [0037](0037-nfr-benchmark-methodology.md) | v1.1.1 | [0044](0044-canonical-header-key-for-map-access.md) |
| Selecting the YAML parser | [0004](0004-runtime-dependency-policy.md) | 5.1 | [0018](0018-config-loader-design.md) |

Two of these are worth reading twice. **The pubsub row was discharged by an ADR's own rejected
alternative**: ADR-0039 rejected "an enum option (`WithDropPolicy(...)`) instead of a boolean" as
more surface for the same expressiveness, and 13.5 shipped exactly that enum as
`WithSlowSubscriberPolicy` once a third policy made it three-valued. A rejection is not permanent
either — it is an entry whose trigger was a change in the option count.

And the **`HeaderName` row discharged with its stated reason refuted.** ADR-0037 deferred it as
"needing an API-visible change"; ADR-0044 fixed it without one. The lesson generalises to every row
above: the *reason* an entry is deferred can be wrong independently of the deferral being right, so a
trigger names evidence to look for rather than an obstacle to wait out.

## §D · Reclassified — not additive, so not in this ledger

| Capability | Believed | Actually | Destination |
|---|---|---|---|
| `*Pool` satisfying `io.Closer` | Additive | Needs `Close()` without a context; ADR-0048 rejected a second `CloseContext` as re-creating the ambiguity `Close` removed | [ADR-0030](0030-spec-v2-reconciliation.md) §2 |
| argon2id as what `HashPassword` uses | Additive | A documented-behaviour change — the bcrypt-cost decision again with a different constant | [ADR-0030](0030-spec-v2-reconciliation.md) §2 |

ADR-0030 §2 is empty and this ADR does **not** write into it: §2 is discharged, and populating it
would reopen a closed record on the strength of a capability nobody has asked for. These two rows are
the pointer. If either trigger fires, the entry is written into §2 in that PR, and a `/v3` is a
maintainer decision that this table does not pre-empt.

## Alternatives Considered

- **A living document under `docs/`** (`docs/ledger/additive-capabilities.md`). Genuinely better on
  one axis: a table edited on many future PRs is a poor fit for a record whose status is "Accepted",
  and a separate file would carry no implication of immutability. Rejected on the symmetry argument
  in Decision 1 — ADR-0042 already sends breaking changes to an ADR section, and two destinations of
  the same kind is a mechanism a contributor can hold in their head where three is not. The
  amend-in-place convention (ADR-0003, ADR-0041) already covers a document that changes.
- **One table instead of two.** Rejected: see Decision 3. The trigger column would mean "demand from
  outside" in one half and "an event the maintainer notices" in the other, and the ambiguity runs the
  wrong way — it licenses firing a surface trigger from inside.
- **A `Priority` or `Wanted` column.** Rejected as the backlog in disguise. Priority is a claim about
  the future made by someone with no evidence; the trigger is a claim about evidence. Adding both
  means the priority is what gets read.
- **A GitHub issue per entry, with a label.** Rejected for the reason `CONTRIBUTING.md` §7 gives:
  issues accrete reactions and age, and 49 open issues describing capability the project has argued
  against would misrepresent the project to anyone browsing it. The ledger is read by someone asking
  "has this been considered?", which is a documentation question.
- **Retrofitting the `Deferred, additive:` marker into the 13 ADRs that phrase it differently**, so
  the gate covers the whole corpus. Rejected: it edits accepted records to satisfy a regex, and buys
  coverage of a set that is already fully enumerated here by census. The marker's job is to catch the
  *next* deferral.
- **A `consistency_lint.py` check that each entry's trigger is non-empty.** Considered and rejected
  as theatre: any string satisfies it, and "when someone needs it" would pass while being exactly the
  entry this ADR refuses. The trigger's quality is a review property, and `CONTRIBUTING.md` §7 plus
  this ADR's Decision 2 are what a reviewer checks against.
- **Doing nothing and grepping when needed.** This is the status quo, and the census refutes it: the
  grep does not find 13 of the 26 ADRs, and it cannot tell a discharged deferral from an open one —
  it would have re-proposed eleven capabilities that already exist.

## Consequences

- **The milestone after this one can be chosen from demand.** That was the item's purpose. As of
  today the honest reading of §A is that **exactly one trigger has fired** — `env`'s float getter,
  from an in-repo consumer — which is a finding rather than a disappointment: it is what "wait for a
  consumer" looks like in a library whose consumers are still arriving. A milestone built from §A
  today would be one entry long, and that is the ledger working.
- **`consistency_lint.py` goes from ten checks to eleven** (`ledger-coverage`), verified by
  deliberate violation in both directions and by printing the sets it sees — 13 marked ADRs against
  36 cited, neither empty, because a check whose inputs are empty passes vacuously. ADR-0056 §(d)
  ("brings that tool to ten") is amended in place with a dated note; the count in ROADMAP 14.7's
  annotation is left alone, since it records what that item built and remains true as history.
- **Surface, behaviour and dependencies are unchanged.** No Go file is touched. The one entry with a
  fired trigger is deliberately **not** implemented here: 14.10 builds the ledger, and scheduling
  from it is the next milestone's job. Discharging an entry in the PR that creates the register would
  make the register look like a to-do list on its first day.
- **The gate's blind spot is documented in Decision 6 rather than left to be discovered**, which is
  ADR-0043's 12.1 lesson and 14.7's `workflow-permissions` hole arriving somewhere new: a checker
  that cannot see part of its domain must say so, because otherwise green reads as complete.
- **A known cost: §A will be wrong the first time an ADR defers something in prose without adding a
  row**, and the gate will only catch it if that ADR uses the marker. The mitigation is that
  `AGENTS.md` §7 and `CONTRIBUTING.md` §6 now name the ledger row as part of writing an ADR, so it is
  a documented step and not folklore.
- **`CONTRIBUTING.md` §7 stops pointing at an unwritten document.** It said the ledger "is roadmap
  item 14.10 and is not written yet"; it now points here, which was the coupling `AGENTS.md` §7
  required in the same pull request.

## References

- [ADR-0030](0030-spec-v2-reconciliation.md) §2 — the breaking-delta ledger this mirrors, and its
  discharge table.
- [ADR-0042](0042-post-1.0-compatibility-contract.md) — the compatibility contract that makes
  ADR-0030 §2 the destination for a breaking change.
- [ADR-0056](0056-build-time-supply-chain.md) — why a new gate extends `consistency_lint.py` rather
  than becoming a fifth policy tool.
- [ADR-0043](0043-spec-api-lint.md) — a checker with a blind spot certifies what it cannot see.
- `CONTRIBUTING.md` §7 — the proposal intake, and the four questions that produce a trigger.
- `ROADMAP.md` 14.10.

