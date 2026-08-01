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

#### 08 — August

- [2026-08-01 — post-v2.0.0 audit: contrib released, and the drift a release does not show](2026/08/2026-08-01-post-v2-audit-and-contrib-tags.md) —
  no roadmap item; post-release maintenance. `v2.0.0` **published** and #80 merged, so the audit asked what
  the release left behind. The release itself verified sound (tag on the release commit, both majors
  coexisting on the proxy, CI and all four policy tools green). **`contrib/*` tagged `v0.2.0`** — a minor
  inside `v0` because the move to the core's `/v2` changed the `health.Check` type they return while **no
  identifier changed**, and `v1.0.0` would commit to stability on an API pinned to a driver major. The
  procedure is now in `release.md`, trap included: **`release.yml` never fires for a `contrib/…` tag**, so
  there is no CI run on the tagged tree and the verification must precede the tag. The rest was
  **governance drift invisible to any diff or gate**: three of `github-setup.md`'s five sections had never
  been applied — the label set did not exist (so every Dependabot PR errored), squash-only was prose only
  and `delete_branch_on_merge` off (**63 stale branches**, deleted after checking merged PRs rather than
  ancestry), two protection flags still off (left to the maintainer). M13's milestone was still open.
  Finally the sweep `project.yaml` kept asking for, held to one test — *would a regeneration produce
  something wrong?* — which is why `public_api` got a note pointing at the gate-enforced spec instead of a
  rewrite.

#### 07 — July

- [2026-07-30 — 13.9: six lines, and the constraint that made them wait](2026/07/2026-07-30-m13-contrib-to-v2.md) —
  roadmap 13.9. **`v2.0.0` tagged and pushed** (agent carry-through; CI drafted the Release, Publish is
  the maintainer's), then both `contrib/*` modules moved to `require …/egl-utils-go/v2 v2.0.0` and
  `…/v2/pkg/health`. Four import lines and two `go.mod` lines — **and the item's substance is why they
  could not ship inside the major**: `unknown revision v2.0.0` before the tag, resolvable after it.
  With the rejected `replace ../..` this could have shipped any time with CI green while the `require`
  consumers resolve went unexercised, so **the constraint that delayed it is what makes it verified**.
  Migrated using the `sed` snippet published in the release notes — contrib is that guide's first real
  consumer, and it confirmed why the pattern anchors on the closing quote. Side effect invisible in the
  diff: MVS raised `pgxhealth`'s `x/sync` v0.17.0 → v0.22.0. `x/text`'s uncalled GO-2026-5970 is
  pre-existing, and its fix is **floor-safe** here (measured) — left to Dependabot. **contrib is
  deliberately not tagged**, so consumers still resolve v0.1.0 against the core's v1.
  **Milestone 13 complete, 10 of 10.**
- [2026-07-30 — v2.0.0 release cut: the milestone that outlives its own release](2026/07/2026-07-30-v2.0.0-release-cut.md) —
  roadmap 13.10. `version.go` → 2.0.0, `[Unreleased]` rolled into a new `docs/changelog/v2/`, release
  notes carrying the whole migration (import rewrite **plus** the seven API changes), and **ADR-0030
  §2's ledger marked empty** with a per-item discharge table that says out loud what the ledger did
  *not* say: an empty ledger is not a closed one, and five items shipped wider or narrower than their
  shorthand. Ran **before 13.9** because ADR-0040 needs the released core — verified with
  `go list -m …/v2@v2.0.0` returning "unknown revision", not assumed. **`consistency_lint` refused my
  claim that M13 was complete** and was right: 13.9 remains, so the milestone outlives its own
  release. The migration `sed` was *run*, not written — the conventional `s|…|…|` form dies on the
  package alternation's own `|`, and matching the closing quote is what keeps a `contrib/*` import out
  of the rewrite. Awaiting the maintainer's merge; the tag is the agent's carry-through.
- [2026-07-30 — 13.8: a constant is a contract, and the default is the decision](2026/07/2026-07-30-m13-hash-default-cost.md) —
  roadmap 13.8. `hash.HashPassword` produces bcrypt cost 12. **Nothing in the surface moves — no
  identifier, no signature, no compile error** — and that is exactly why it needed a major: the
  constant *is* the contract, re-measured at **×4.03** per login before flipping it. The capability was
  never the point (`HashPasswordCost` shipped in v1.1.0); the item is what a caller who expresses no
  opinion gets, and the mechanism is the gap between default and floor — cost 10 stays legal but has to
  be written down. The module also **stops inheriting `bcrypt.DefaultCost`**, so its most
  security-relevant number is no longer movable by a dependency bump. No hash is invalidated and no
  migration exists to run — pinned against a *captured* cost-10 hash. Second consecutive item
  `spec_api_lint` could not have caught. ADR-0052; **ledger item 20 discharged — the ADR-0030 §2 ledger
  is empty.**
- [2026-07-29 — 13.7: overturning a deviation by reading what it actually said](2026/07/2026-07-29-m13-lifecycle-timeout.md) —
  roadmap 13.7. `WaitForSignals` takes a shutdown timeout. The only M13 item that overturns a
  *deliberate* documented deviation — and it turned out ADR-0025's objections were narrower than its
  heading: it refused a deadline that was hidden, invented, default and silent, all of which describe
  one the **caller did not choose**. So the reasoning is preserved and only the conclusion reverses,
  and it is that reasoning which decided `0` still means no deadline — the right posture where a
  platform grace period already exists. First item where the gap-column tie-breaker pointed *at* the
  change rather than away from it. The deadline starts at the signal, not the call: verified by moving
  the derivation earlier and watching the test fail. ADR-0051; ledger item 21 discharged.
- [2026-07-29 — 13.6: nine modules for two metrics, and the 37 families nobody chose](2026/07/2026-07-29-m13-metrics-no-sdk.md) —
  roadmap 13.6. The Prometheus SDK leaves the module: `metrics` writes text exposition format directly
  and `Prometheus(reg)` becomes `metrics.New()` with `Middleware()`/`Handler()` methods. Nine of
  eighteen modules left the graph, which also retired ADR-0027's uncalled-advisory trade-off by
  deleting its subject. `promhttp` turned out to be serving **37 families nobody had chosen** — the
  measurement that made this a maintainer decision rather than a refactor. Conformance is pinned by a
  golden captured from the reference encoder *before* it was removed, so the check outlives the
  library. Recording 1 alloc → 0, scrape 436 allocs → 3. ADR-0050; ledger item 23 discharged.
- [2026-07-29 — 13.5: the pubsub reshape, or how to add a context and an error without letting Publish block](2026/07/2026-07-29-m13-pubsub-reshape.md) —
  roadmap 13.5. Context-scoped subscriptions, `Publish(ctx, topic, msg) error`, `ErrSlowSubscriber`, and
  a three-valued `SlowSubscriberPolicy`. ADR-0006 had made "Publish never blocks" *unarguable* by giving
  it nothing to return; the promise now has to be kept explicitly, so `ctx` is consulted once before any
  delivery and the error only ever reports what already happened. Cancel is the unsubscribe, and
  `context.AfterFunc` is what keeps the broker's zero-goroutine guarantee. `Disconnect` came from the gap
  analysis, `topic` stayed on 13.4's tie-breaker. ADR-0049; ledger item 2 discharged.
- [2026-07-29 — 13.4: one shutdown verb, and the parameter that should not follow it](2026/07/2026-07-29-m13-workerpool-close.md) —
  roadmap 13.4. `workerpool.Stop` → `Close`, `ErrPoolClosed` → `ErrClosed`. The vocabulary sweep found
  exactly one outlier of three shutdowns. `Close` keeps its `ctx` against the ledger's literal
  `Close() error`, so v2 is uniform on the verb and deliberately not on the signature — the pool is the
  only shutdown that waits on work the caller wrote. Cost stated out loud: `*Pool` is not an
  `io.Closer`. No new test, because the rename created no new invariant; and a `\bStop\b` rename has
  two blind spots of the same shape — `b.StopTimer()` and `TestStop…` names.
- [2026-07-28 — 13.3: the error channel that carried one bit](2026/07/2026-07-28-m13-cache-comma-ok.md) —
  roadmap 13.3. `cache.Get` → `(V, bool)`, `NewInMemory` → `New`, `ErrNotFound` deleted rather than
  kept (a sentinel nothing returns makes `errors.Is` compile and never be true — a compile break
  turned into silent falsehood). **ADR-0021 is only partially superseded, and the surviving half is
  load-bearing:** expiry judged at call time is what makes the boolean sound.
  *A grep found two call sites; the compiler found four.*
- [2026-07-28 — 13.2: errx, and the 276 nanoseconds v1 spent looking for a stack it already had](2026/07/2026-07-28-m13-errx-opt-in-stacks.md) —
  roadmap 13.2. 13.1's PR was blocked by a **red required check**, not a missing click: the `pkg/`
  move left CI's only two **hand-written** package paths behind, and the nightly one would have
  failed unwatched. Then stacks became opt-in — and the benchmark found that v1's `Wrap` paid
  **276 ns even when it captured nothing**, an `errors.As` walk per wrap to find a stack it already
  had. *Symbolization is 6.5× the capture, which is why `[]Frame` resolves lazily.*
- [2026-07-27 — M13 opens: `/v2`, `pkg/`, and a layout decided by building both](2026/07/2026-07-27-m13-v2-pkg-layout.md) —
  roadmap 13.1. The root had reached **40 entries** and the maintainer could not read it. The Maven
  tree was **built, verified green, and reverted** once the working version gave the number three
  rounds of prose had not: **86-character imports against `pkg/`'s 59**, for an identical root.
  *The clutter was never the packages being at the root — it was there being twenty-one of them.*
  [ADR-0045](../adr/0045-pkg-layout-and-v2.md): `pkg/` + `/v2`, root 40 → 19. Milestone 13 also
  **empties the ADR-0030 ledger in the same major** — a boundary opened and not used has to be
  opened again.
- [2026-07-27 — v1.1.1 release cut: a patch that was supposed to need a major](2026/07/2026-07-27-v1.1.1-release-cut.md) —
  master green again (0 failing jobs, both `-race` included). Asked whether v1.2.0 was due: **no** —
  ADR-0030's additive bucket was fully consumed by M10, so MINOR has nothing to carry. **v1.1.1
  instead, and it overturns a decision carried since 10.10**: the `middleware.HeaderName` allocation
  cost was recorded as blocked behind a MAJOR-only change to the exported constant, but the
  constant's *value* and the *cost of using it as a map key* are separable. An unexported canonical
  spelling for map access removes **2 allocs/request** with `HeaderName` untouched
  ([ADR-0044](../adr/0044-canonical-header-key-for-map-access.md)); ratchet lowered and proved to
  bind. BUG-0001's `fixed-in` resolves to v1.1.1. *Addendum:* v1.1.1 tagged (CI checked green on the
  merge commit **before** tagging); **branch protection applied to `master`**, which had been
  unprotected; and the **NFR-01 amendment**, which turned out not to be an edit — the target lives
  in an unmodifiable verbatim import, the gap analysis is a dated snapshot, and the frozen v1
  contract never made the claim, so it lands as a **maintained deviation in ADR-0030 §3**.
  ADR-0037 had filed it under the `/v2` ledger, a bucket with no exit. **Both of 10.10's
  carried-forward findings are now discharged.**
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
