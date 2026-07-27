# Session Journal

Dated end-of-session checkpoints — what got done, where the project stands, and how the
next session resumes. One file per session that changed the project's state, at
`docs/journal/<YYYY>/<MM>/<YYYY-MM-DD>-<short-slug>.md`. The journal is the dated trail;
`ROADMAP.md` is the forward plan — checkpoints never live inline in the roadmap.

At the close of a state-changing session, the agent:

1. Creates the dated file under `docs/journal/<YYYY>/<MM>/`.
2. Adds a link row to this index (newest first, grouped by year/month).
3. Updates the *Latest checkpoint* pointer in `ROADMAP.md`.

## Index

### 2026

_(newest first)_

#### 07 — July

- [2026-07-27 — v1.1.1 release cut: a patch that was supposed to need a major](2026/07/2026-07-27-v1.1.1-release-cut.md) —
  master green again (0 failing jobs, both `-race` included). Asked whether v1.2.0 was due: **no** —
  ADR-0030's additive bucket was fully consumed by M10, so MINOR has nothing to carry. **v1.1.1
  instead, and it overturns a decision carried since 10.10**: the `middleware.HeaderName` allocation
  cost was recorded as blocked behind a MAJOR-only change to the exported constant, but the
  constant's *value* and the *cost of using it as a map key* are separable. An unexported canonical
  spelling for map access removes **2 allocs/request** with `HeaderName` untouched
  ([ADR-0044](../adr/0044-canonical-header-key-for-map-access.md)); ratchet lowered and proved to
  bind. BUG-0001's `fixed-in` resolves to v1.1.1.
- [2026-07-27 — contrib v0.1.0 released; BUG-0001, a red master nobody could see](2026/07/2026-07-27-contrib-release-and-bug-0001.md) —
  `contrib/redishealth/v0.1.0` + `contrib/pgxhealth/v0.1.0` tagged and **verified live on
  proxy.golang.org and sum.golang.org**. Then found `master` red since 2026-07-26 — **the `v1.1.0`
  release commit included** — in the two `-race` jobs.
  **[BUG-0001](../bugs/2026/07/BUG-0001-race-detector-breaks-allocation-and-pool-identity-assertions.md)**:
  no data race, four assertions measuring allocation counts and `sync.Pool` identity, both of which
  `-race` perturbs. Fixed by `//go:build !race` exclusion, **not** by raising the budgets — that
  would mask the regressions the ratchet exists to catch. Nobody saw it because `-race` needs cgo
  and this workstation has no C compiler, so every "local gauntlet green" skipped it entirely.
- [2026-07-27 — M12: §5 reconciled with the real exported surface, then gated](2026/07/2026-07-27-m12-public-interface.md) —
  roadmap 12.1. §5 had never been updated after M10, so **twelve identifiers shipped in v1.1.0 were
  missing** and `pubsub.NewBroker`'s option type was given as `Option` where it is generic
  `Option[T]` — code written to the spec would not compile. Rebuilt from `go doc`; **110 identifiers
  verified present** by script. The substantive half: the versioning clause bound "all exported
  identifiers **above**", making the enumeration the boundary of the promise and leaving the drifted
  ones outside it — **narrower than the v1.0.0 changelog**. Now binds the whole surface.
  *Addendum, 12.2:* §4's "adoptable in isolation" absolute struck and superseded by the **existing**
  ADR-0033 — **no new ADR minted**, since that record already holds the decision; the replacement
  says *governed* exception, because `import_graph_lint.py` fails both if an unsanctioned edge
  appears and if `config → validator` disappears. Plus §6/§3 understatements (rapid in 8 packages
  not 3, benchmarks in 7 not 4, `prometheus/client_model` omitted). *Addendum 2, 12.3:*
  `tools/spec_api_lint.py` ([ADR-0043](../adr/0043-spec-api-lint.md)) gates §5 against `go doc` in
  **both** directions and **found a tenth divergence on its first run** — `workerpool.ErrPoolClosed`,
  invisible to every earlier scan because it is the *second* member of a `var (…)` block. It reports
  **130** identifiers where 12.1's throwaway checker saw 110. Verified by deliberate violation in
  three shapes; also documented the three policy tools that had never reached AGENTS.md or the PR
  template. **Milestone 12 complete (3/3).**
- [2026-07-27 — Governance: namespace contract & spec reconciliation](2026/07/2026-07-27-series-namespace-contract.md) —
  Milestone 11 (2/2, docs only). "Can we move to `src/main/go/it/d4np/utils`?" answered with the
  language: in Go an import path *is* a directory path, and the tree is 1-for-3 across the siblings
  anyway. [ADR-0041](../adr/0041-series-logical-namespace.md) replaces the physical tree with the
  **logical namespace `it.d4np.utils.<component>`**; module path deliberately unchanged, the
  vanity/org alternatives declined and ledgered in ADR-0030 §2 as *free only at a `/v2` boundary*.
  Closes the regeneration caveat ADR-0003 left open. **11.2** then reconciled the **frozen** v1 spec,
  which had diverged in four places: three stale facts amended in place (language floor 1.24 → 1.25,
  coverage 80% → 85% *per package*, the dead goleak hedge) and one replaced *rule* —
  [ADR-0042](../adr/0042-post-1.0-compatibility-contract.md) retires the MAJOR-intent note and makes
  the `/v2` ledger the only destination for a breaking change. Upstream action still open: the EADOS
  `go.yaml` profile.
- [2026-07-27 — v1.1.0 release cut](2026/07/2026-07-27-v1.1.0-release-cut.md) — Milestone 10 cut as
  **[v1.1.0](../releases/v1.1.0.md)**: `[Unreleased]` moved into `docs/changelog/v1/v1.1.0.md`, version
  constant/badge/release notes in lockstep (verified the gate fails on a mismatch). Two findings carried
  forward deliberately — NFR-01's unachievable 0-alloc target and the non-canonical
  `middleware.HeaderName`. The contrib modules are **not** in this tag. Remaining agent step: tag after merge.
- [2026-07-27 — M10.13: contrib/ submodules — **Milestone 10 complete**](2026/07/2026-07-27-m10-contrib-submodules.md)
  — roadmap 10.13 (spec v2 item 22, ADR-0040); `contrib/redishealth` + `contrib/pgxhealth` as separate
  modules requiring the released core (no `replace`, no `go.work`). Closed the three things that
  silently ignore a nested module — the import-graph lint, the coverage gate and CI — since a contrib
  directory losing its `go.mod` would drag a driver into the core unnoticed. Next: the v1.1.0 cut.
- [2026-07-26 — M10.12: pubsub.WithDropOldest](2026/07/2026-07-26-m10-pubsub-drop-oldest.md) —
  roadmap 10.12 (spec v2 item 2, ADR-0039); opt-in policy that evicts the **oldest** buffered message, for
  state-like streams. **Best-effort by construction** — evicting from a channel is receive-then-send, and
  retrying without bound would break ADR-0006's "Publish never blocks". The drop handler reports the
  *evicted* message, so the accounting invariant survives. pubsub 96.4% → **100%** coverage.
- [2026-07-26 — M10.11: cache sharding (the bench demanded it)](2026/07/2026-07-26-m10-cache-sharding.md)
  — roadmap 10.11 (spec v2 item 17, ADR-0038); 10.10's numbers answered the brief's "shard only if the
  bench demands", so `Cache` is now 32 shards keyed by `maphash.Comparable`: **NFR-06's 90/10 mix goes
  349.8 → 46.6 ns (7.5×)**, at a recorded ~5 ns cost to uncontended operations. One sweeper per cache
  preserved and pinned at a thousand caches.
- [2026-07-26 — M10.10: the NFR suite (two NFRs do not hold)](2026/07/2026-07-26-m10-nfr-suite.md) —
  roadmap 10.10 (spec v2 §5, ADR-0037 + [report](../benchmarks/2026-07-26-nfr-suite.md)); gates the
  hardware-independent NFRs and reports the rest. **NFR-01's 0-alloc target is unachievable** (replaced by
  a ratchet) and **NFR-06 is not met** — a single RWMutex serialises readers behind every `Set`, which is
  10.11's sharding evidence. NFR-02/06 tails are unmeasurable on Windows (100% of adjacent
  `Now`/`Since` pairs read 0 ns).
- [2026-07-26 — M10.9: per-package coverage gate](2026/07/2026-07-26-m10-coverage-gate.md) —
  roadmap 10.9 (spec v2 §7, ADR-0036); 85% enforced **per package**, because with 16 of 21 packages at
  100% a module-wide gate could never fail. Real low-water mark is fanout 93.3%. Also discharges
  AGENTS.md §10's outstanding "finalized in an ADR".
- [2026-07-26 — M10.8: import-graph enforcement](2026/07/2026-07-26-m10-import-graph-enforcement.md)
  — roadmap 10.8 (spec v2 §3, ADR-0035); ADR-0004's rings and the layer graph become build-breaking rules
  via depguard **plus** a resolved-graph assertion, because **depguard does not report a blank import of a
  sibling package** (verified) and cannot see a new direct requirement or a dead exception. Every rule
  verified by deliberate violation.
- [2026-07-26 — M10.7: fuzzing (and a contract violation it found)](2026/07/2026-07-26-m10-fuzzing.md)
  — roadmap 10.7 (spec v2 §7, ADR-0034); `FuzzValidatorTags` asserts a contract-shaped invariant
  (`runtime.Error` vs documented panic) because `validator.Struct` panics by design, and its tag space is
  bounded because `reflect.StructOf`'s type cache never evicts. **Found and fixed a real defect:
  `config.Load` returned a partially decoded struct on error despite promising the zero value.**
- [2026-07-26 — M10.6: config.WithStructValidation()](2026/07/2026-07-26-m10-config-struct-validation.md)
  — roadmap 10.6 (spec v2 item 13, ADR-0033); opt-in option, tags run before `Validator` and a tag
  failure skips `Validate`. Small code, but it establishes the module's **first internal package edge**
  (`config → validator`, L2 → L2) — 10.8's depguard rules must permit it.
- [2026-07-26 — M10.5: hash.HashPasswordCost (security-relevant)](2026/07/2026-07-26-m10-hash-password-cost.md)
  — roadmap 10.5 (spec v2 item 20 + §7, ADR-0032 extending ADR-0024); the cost range is validated
  **locally** because bcrypt silently promotes sub-`MinCost` values and honours costs 4–9 verbatim
  (verified empirically, pinned by a test); error not panic; `Cost()` makes rehash-on-login actionable;
  cost-sizing report shows exact doubling and that verify costs the same as hash — the per-login DoS
  trade-off, mitigated by 10.4's middleware. Control C-4 extended, 3 threat-model rows.
- [2026-07-26 — M10.4: ratelimit.Middleware() + ErrLimited](2026/07/2026-07-26-m10-ratelimit-middleware.md)
  — roadmap 10.4 (spec v2 item 8, ADR-0031, the milestone's first new ADR); `(*Limiter).Middleware()`
  sheds via `Allow` rather than queueing via `Wait`, 429 + constant `Retry-After`, no logging (a
  client-triggerable log line would be a flood amplifier), 0-alloc admit path; global-budget
  limitation carried as control C-5.
- [2026-07-26 — M10.3: lifecycle.Trigger()](2026/07/2026-07-26-m10-lifecycle-trigger.md) —
  roadmap 10.3 (spec v2 item 21, ADR-0030); coordinator-scoped `triggered` channel closed once via
  `sync.Once`, `WaitForSignals` selects over signals + trigger, so a trigger arriving first latches
  rather than being lost. Also re-tidied `go.mod`/`go.sum`, which the #44 Dependabot bump had left
  unbuildable.
- [2026-07-16 — M10.2: circuitbreaker.State()](2026/07/2026-07-16-m10-circuitbreaker-state.md) —
  roadmap 10.2 (spec v2 item 6, ADR-0030, PR #38); exported State type + String() and a pure read-only
  `(*Breaker).State()` that reflects the lazy transition without performing it (no mutation, no
  probe). First feature item of Milestone 10.
- [2026-07-16 — M10 opens: spec v2 reconciliation (hybrid)](2026/07/2026-07-16-m10-reconciliation.md)
  — (PR #37) maintainer chose hybrid adoption; spec v2 imported verbatim to docs/specs/v2/; ADR-0030 records
  the three-bucket disposition (13 additive items → Milestone 10 → v1.1.0; breaking → /v2 ledger;
  deviations maintained with their ADRs).
- [2026-07-15 — Spec v2.0 discovered: gap analysis](2026/07/2026-07-15-spec-v2-gap-analysis.md) —
  found untracked `.spec/` (spec v2.0 + 3 maintainer ADRs, post-intake, never reconciled); wrote
  `docs/specs/02_spec_v2_gap_analysis.md` — all 25 items classified, breaking vs additive deltas,
  v2's internal YAML/zero-dep contradiction, adoption paths. Informational; disposition open.
- [2026-07-15 — Release: v1.0.0 (feature-complete)](2026/07/2026-07-15-v1-release.md) — first stable
  release (PR #34); version.go → 1.0.0, CHANGELOG [Unreleased] (M2–M9) rolled into docs/changelog/v1/v1.0.0.md,
  release notes drafted; SemVer API-stability commitment. Agent tags v1.0.0 after the merge.
- [2026-07-15 — M9.5: errors.Wrap — feature-complete](2026/07/2026-07-15-m9-errors.md) — roadmap 9.5
  (ADR-0029, PR #33); %w-transparent Wrap/Wrapf, one-time origin stack (StackTracer + %+v), Wrap(nil)=nil.
  **Final feature — the library is feature-complete (all 25 spec features).**
- [2026-07-15 — M9.4: syncpool.BufferPool](2026/07/2026-07-15-m9-syncpool.md) — roadmap 9.4 (ADR-0028, PR #32);
  sync.Pool of bytes.Buffer, reset-on-Put, discards buffers over a 64 KiB cap (retention trap);
  AllocsPerRun zero-alloc proof; Object Pool pattern → Implemented (catalogue row 10).
- [2026-07-15 — M9.3: metrics.Prometheus](2026/07/2026-07-15-m9-metrics.md) — roadmap 9.3 (ADR-0027, PR #31);
  request counter + latency histogram labelled (method, code) — no path label, method normalized
  (cardinality-DoS mitigation); adds prometheus/client_golang v1.23.2 (floor-preserving); one
  uncalled x/sys advisory kept to preserve the 1.24 floor.
- [2026-07-15 — M9.2: health.Handler](2026/07/2026-07-15-m9-health.md) — roadmap 9.2 (ADR-0026, PR #30);
  concurrent dependency probes on the request context, 200/503, status-only JSON body (never the
  probe error — info-disclosure); loud panic on empty/dup name or nil probe.
- [2026-07-15 — M9 opens: lifecycle.GracefulShutdown](2026/07/2026-07-15-m9-lifecycle.md) — roadmap
  9.1 (ADR-0025, PR #29); LIFO shutdown hooks, run-all + errors.Join, exactly-once convergent Shutdown, no
  hidden timeout, zero owned goroutines; injected signal seam for Windows-deterministic tests.
- [2026-07-15 — M8.2: hash (bcrypt) — Milestone 8 complete](2026/07/2026-07-15-m8-hash.md) — roadmap
  8.2 (ADR-0024, PR #28); bcrypt hashing/verify — default cost 10, per-hash salt, ErrPasswordTooLong,
  constant-time ErrMismatch; adds x/crypto v0.48.0 (floor-preserving); control C-4 + auditor sign-off.
  Milestone 8 complete.
- [2026-07-15 — M8 opens: validator.Struct](2026/07/2026-07-15-m8-validator.md) — roadmap 8.1
  (ADR-0023, PR #27); hand-rolled reflection validator (required/email/min/max/oneof), literal rules, nested
  recursion with dotted paths, full aggregation; data violations returned, tag misuse panics.
- [2026-07-15 — M7.2: db.Transaction — Milestone 7 complete](2026/07/2026-07-15-m7-db.md) — roadmap
  7.2 (ADR-0022, PR #26); auto-rollback transaction helper — commit on nil, rollback+return on error
  (errors.Join if rollback fails), rollback+re-panic on panic; fake sql driver in tests. M7 complete.
- [2026-07-15 — M7 opens: cache.InMemory](2026/07/2026-07-15-m7-cache.md) — roadmap 7.1
  (ADR-0021, PR #25); generic TTL cache — expiry enforced by Get (stale reads impossible), one sweeper
  goroutine with sync.Once Close (goleak-gated), fake-clock boundary tests, 0 allocs/op hot paths.
- [2026-07-15 — M6.2: logger.Context — Milestone 6 complete](2026/07/2026-07-15-m6-logger-context.md)
  — roadmap 6.2 (ADR-0020, PR #24); `WithFields`/`FromContext` carry accumulating logger fields through
  context (Field = slog.Attr alias), `FromContext` enriches slog.Default. Milestone 6 complete.
- [2026-07-15 — M6 opens: logger.Structured](2026/07/2026-07-15-m6-logger-structured.md) — roadmap
  6.1 (ADR-0019, PR #23); `NewStructured` returns a slog JSON-handler `*slog.Logger` tuned for ES/Loki, with
  WithWriter/WithLevel/WithSource/WithAttrs; composes with `middleware.Logger`.
- [2026-07-15 — M5.2: env.GetDefault — Milestone 5 complete](2026/07/2026-07-15-m5-env.md) —
  roadmap 5.2 (PR #22); typed env reads (`GetDefault`/`GetInt`/`GetBool`/`GetDuration`) with safe fallbacks;
  no ADR (routine). Milestone 5 complete.
- [2026-07-15 — M5 opens: config.Loader](2026/07/2026-07-15-m5-config.md) — roadmap 5.1
  (ADR-0018, PR #21); generic `Load[T]` for JSON/YAML with `${VAR}` env expansion and a `Validator` hook;
  selects + pins `gopkg.in/yaml.v3` (already indirect) under ADR-0004's budget.
- [2026-07-15 — M4.4: HTTP middleware (Cors) — Milestone 4 complete](2026/07/2026-07-15-m4-cors.md)
  — roadmap 4.4 (ADR-0017, PR #20); fourth/last M4 middleware — CORS preflight (terminal 204), deny-by-default
  origins, exact-origin echo + Vary, loud panic on the Fetch-forbidden credentials+`*` combo (new
  control C-3). Milestone 4 complete.
- [2026-07-15 — M4.3: HTTP middleware (Recoverer) + ADR-0015 backfill](2026/07/2026-07-15-m4-recoverer.md)
  — roadmap 4.3 (ADR-0016, PR #19); third HTTP middleware — panic→clean 500, no stack/panic leaked to
  the client (info-disclosure, C-2), server-side Error log, `http.ErrAbortHandler` passthrough;
  also backfills ADR-0015 (enterprise posture) to close the referenced-but-unwritten record.
- [2026-07-14 — M4.2: HTTP middleware (Logger)](2026/07/2026-07-14-m4-logger.md) — roadmap
  4.2 (ADR-0014, PR #18); second HTTP middleware — one structured `slog` line per request,
  Unwrap-aware status/bytes capture, status-derived levels, path-only logging (extends the
  threat model's Info-disclosure row + compliance C-2).
- [2026-07-14 — M4 opens: HTTP middleware (RequestID)](2026/07/2026-07-14-m4-middleware.md)
  — roadmap 4.1 (ADR-0013, PR #17); first HTTP middleware — adopts Decorator, crosses the
  first untrusted-input boundary (threat model + compliance C-2), `crypto/rand.Text` IDs.
- [2026-07-14 — M3 opens: circuitbreaker](2026/07/2026-07-14-m3-circuitbreaker.md) —
  roadmap 3.1 (ADR-0010, PR #14) + addenda 3.2 retry (ADR-0011, PR #15) and 3.3 ratelimit
  (ADR-0012, PR #16 — **Milestone 3 complete**, first benchmark report); healed the red
  master (2.6 go.sum handoff) via a portable Go toolchain; first local verification.
- [2026-07-12 — M1 bootstrap](2026/07/2026-07-12-m1-bootstrap.md) — Go module + quality
  configs; ADR-0003 (root layout) + ADR-0004 (dependency policy); Milestone 1 complete.
  Addenda 1–7 carry the whole of Milestone 2 (2.1–2.6, same-day sessions).
