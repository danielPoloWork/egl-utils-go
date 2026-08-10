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

- [2026-08-10 — the quickstart is production code, and so is a doc comment](2026/08/2026-08-10-quickstart-listener-failure.md) —
  the first fix out of the review board's backlog ([#107](https://github.com/danielPoloWork/egl-utils-go/issues/107)),
  and the finding that came with it. The README quickstart discarded the `ListenAndServe` error and
  set no timeouts — hit independently by three of the seven reviewers, from availability, API-contract
  and Slowloris arguments that needed nothing from each other. **The same defect was sitting unfiled
  in `pkg/lifecycle`'s package doc comment**, verified against the tags as present since `v1.0.0`,
  which makes it the larger of the two: pkg.go.dev renders doc comments and not Markdown, so that
  line is what every `godoc` reader and IDE hover has been shown in five published versions. Both now
  check `http.ErrServerClosed` and route anything else through `lifecycle.Trigger` — which is the use
  the `Trigger` paragraph five lines below already claimed for it. The rule the board wrote down and
  this session kept: **a quickstart is a production template people paste, and earns production
  review.** Same session, same forty lines:
  [#108](https://github.com/danielPoloWork/egl-utils-go/issues/108) — no readiness endpoint in a
  service whose point is load shedding — where the finding was right and its one-line remedy was
  loose enough that following it literally would have contradicted the module's own design. `/readyz`
  now exercises the real admission path; `/healthz` stays deliberately checkless, and now says so.
  Third: [#109](https://github.com/danielPoloWork/egl-utils-go/issues/109) — the usage guide's
  retry recipe carried `pkg/retry/example_test.go`'s clock-free test policy without the comment
  that made it safe to show, demonstrating the retry storm the very next paragraph warns against.
  Now states a real `BaseDelay`/`Jitter`, the shape every other recipe in the guide already uses.
- [2026-08-09 — a release review board, and the promise the README could not keep](2026/08/2026-08-09-release-review-board.md) —
  seven specialist reviewers, blind to one another, evaluated `master` as a release candidate against
  `v2.0.1` under a decision rule frozen **before** any report was read. Verdict: **not approvable**,
  on one BLOCKER — the README's promise that **no package here imports another** is false
  (`pkg/config/config.go:47` imports `pkg/validator`, and `import_graph_lint.py` *mandates* that
  edge, while the ADR cited as the promise's authority is the record of the exception). It matters
  because `README.md` ships inside the module zip, where Go immutability makes the sentence
  permanent. **The finding that inverts 14.12's reasoning: pkg.go.dev renders Go doc comments from a
  published version and does *not* render Markdown** — so the README rewrite already reached
  consumers when #105 merged, the 438-line usage guide will never reach pkg.go.dev with or without a
  tag, and a doc-only tag delivers nothing while still billing every downstream a Dependabot PR.
  Also settled: BUG-0002's "repository-wide sweep" was scoped to `_test.go` only, and
  `workerpool.New(n, 0, WithNonBlockingSubmit())` is the same primitive reachable from the public
  API and untested; and `govulncheck`'s "1 vulnerability in modules you require" sits in the same
  block as "affected by 0" and is the **non-reachable bucket** — read the whole output, never one
  line. 43 findings filed as issues `#106`–`#148`, indexed by the new
  [`ISSUES.md`](../../ISSUES.md).
- [2026-08-08 — the README proved the project was well-run and never sold it](2026/08/2026-08-08-readme-and-usage-guide.md) —
  a product-lens evaluation of the front door, and the rewrite. Measured, not asserted: **0 lines of
  Go on the page**, `go get` absent (the first actionable command was `go build ./...`, which a
  *contributor* runs), 20 of 165 lines spent on an internal milestone tracker, and no statement of
  why to choose this over assembling four libraries. Now opens with the value proposition, `go get`,
  and a **complete ~40-line service**; milestones and the document index moved into a collapsed
  *Project governance* section. **Checking the lint constraints first changed the plan**: wrapping
  the milestone table in `<details>` keeps all 14 rows parseable, so the page got clean with **no
  gate loosened** — a gate should not be relaxed for a layout preference. New
  [usage guide](../usage/README.md) fills the layer between `go get` and per-identifier reference.
  **Every snippet derived from code CI compiles and runs**, because there is no Go toolchain here —
  which caught a genuine error: `fanout.Split` takes variadic output channels and returns nothing,
  so the draft's `outs := fanout.Split(ctx, in, 4)` would not have compiled, in the document that
  exists to show people how to use the library.
- [2026-08-08 — the front door that never said what was inside](2026/08/2026-08-08-readme-package-inventory.md) —
  the README described the module, its governance and its milestones, and **never listed its
  packages**. Adds a `Packages` section: 21 packages in eight groups, one sentence each, every name
  linking to **pkg.go.dev** — which ADR-0058 already made the doc site, so the README needed the
  *index into* it, not a second copy. Descriptions derived mechanically from each package's **own doc
  comment**, which independently confirmed two carried-forward numbers: all 21 have a doc comment,
  and the example count really is **55**. Gated by `consistency_lint` check 12 (`readme-packages`,
  a both-ways bijection), because a hand-written 21-row table is what goes stale when the 22nd
  package arrives. **The second deliberate violation FAILED TO FAIL** — a `…/pkg/semaphoreX` typo
  passed, because the name pattern `[a-z0-9_]+` matched the leading lowercase run and stopped, so
  *any* uppercase typo would have slipped through; widened to `[A-Za-z0-9_-]+`. **A test that fails
  to fail is the only signal separating "the check works" from "the check agrees with me today".**
- [2026-08-08 — the hand-off that was blocked by an endpoint, not a policy](2026/08/2026-08-08-enable-required-signatures.md) —
  `required_signatures: true` on `master`, discharging a hand-off open since 14.7. ADR-0056's premise
  (squash-only, so GitHub signs the commit that lands) was **re-checked against four newer commits
  before flipping anything**, not inherited. **The finding: the blocker was the ENDPOINT, not the
  setting.** ADR-0056 had generalised "the whole-object `PUT` is blocked" to all its hand-offs; in
  fact `required_signatures` and the `examples / service` context each have a narrow sub-resource
  endpoint and were closable alone, while `required_linear_history` and
  `required_conversation_resolution` genuinely need the whole-object call. **Prefer the sub-resource
  endpoint; only settings that lack one are blocked.** Tags are unaffected (branch protection binds
  the branch ref), reverting is one `DELETE`, and **the next merge is the real test** — `enforce_admins`
  is true, so the rule applies to the maintainer too.
- [2026-08-08 — the hole stays open, and now it is a decision](2026/08/2026-08-08-v0.1.0-release-not-backfilled.md) —
  a correction session. 14.11 drafted `v0.1.0`'s missing Release; the maintainer chose **not** to
  publish it and the draft was removed — but `v2.0.1` had already shipped announcing the backfill, so
  **a released changelog asserted something that does not exist**. Verified first: no `v0.1.0` release
  at all, tag intact. Keeping it deleted is defensible (that version predates every feature package,
  its own notes say not to install it, and `docs/releases/v0.1.0.md` already holds the record — the
  Release would publish little beyond "do not use this"), and the asymmetry is now **a recorded
  decision rather than an oversight**, which is what differs from the starting state. **A released
  record is corrected BESIDE the original claim, never in place of it** — deleting it would make the
  published release notes and the repository disagree about what `v2.0.1` said. Five documents
  amended; the 14.11 journal left untouched, because a dated trail exists precisely so history is not
  rewritten to match the present. `release.md`'s backfill procedure survives with the clause it was
  missing: **decide whether it is worth publishing before drafting it.**
- [2026-08-08 — the last job that blocked nothing](2026/08/2026-08-08-require-examples-service-context.md) —
  governance follow-through after M14 closed: **`master` now requires 14 status contexts, not 13** —
  `examples / service` is the new one, app-pinned like the rest. ADR-0054's warning ("adding a job
  does not add the context") **was not theoretical**: BUG-0002 rode two merges red, the second being
  the `v2.0.1` release PR. **Order was the decision — fix the flake first, require second**, because
  requiring an intermittently-failing job makes a repository unmergeable rather than stricter, and
  the pressure that follows is to drop the requirement. New `github-setup.md` §3.1 records the API
  detail: **never re-`PUT` the whole protection object** to add one context (it silently disables
  anything omitted); PATCH the sub-resource with **`checks`, not `contexts`** (the latter adds it
  *unpinned*), sending the full array because it replaces rather than merges. **The trap survives one
  level down** — the job is a matrix, so a second module yields a new, unrequired context.
- [2026-08-08 — BUG-0002: the signal that was dropped because nobody was listening yet](2026/08/2026-08-08-bug-0002-test-deadlock.md) —
  [BUG-0002](../bugs/2026/08/BUG-0002-unbuffered-started-channel-deadlocks-two-examples-service-tests.md),
  the first work after M14 closed and not a roadmap item. Two `examples/service` tests announced
  "a worker dequeued the first task" with a **non-blocking send on an unbuffered channel** — which
  only delivers if a receiver is already parked, so whenever the worker got there first the signal
  was dropped and both goroutines waited forever, to the 10-minute timeout. **The comment above it
  claimed "no timing assumption"; `select`/`default` IS one, and it was the one that failed.** Fixed
  with one character of buffer capacity; a sleep was rejected as converting a deadlock into a slow
  flake. Found by the release PR's own CI, after failing unremarked on #97 first — **`examples /
  service` is still not a required status check** (14.5's open hand-off), so both merged red, and a
  **monitor that times out is not a green result.** No consumer affected: the module is never tagged.
- [2026-08-08 — 14.12: the release whose entire payload is a tag](2026/08/2026-08-08-release-v2.0.1.md) —
  roadmap 14.12; **Milestone 14 complete, 12/12**. `v2.0.1` prepared: version constant, `[Unreleased]`
  moved into [the per-version changelog](../changelog/v2/v2.0.1.md) in roadmap order,
  [notes](../releases/v2.0.1.md) drafted, badge and milestone rows flipped. **A PATCH with no code in it and a real reason to cut
  it** — pkg.go.dev renders from the *tagged tree*, so 55 examples across 21 packages had been
  invisible in `master` for a week, and the release *artifacts* changed (first tag on which 14.7's
  SBOM path does anything; all four earlier releases have zero assets). The notes **lead with "no
  code changed" and prove it** — no `pkg/` file touched, surface still 141, `go.mod`/`go.sum`
  byte-identical. The `milestones` gate caught the README row being flipped before 14.12's own
  checkbox: **the last item of a milestone is the one most easily left unticked.** Lockstep verified
  by reverting `version.go` and the badge in turn. **M15 is deliberately unchosen** — 49 ledger
  entries, one fired trigger.
- [2026-08-08 — 14.11: the private reporting form nobody could reach](2026/08/2026-08-08-repo-metadata-and-v0.1.0-release.md) —
  roadmap 14.11, [ADR-0058](../adr/0058-no-documentation-site.md). **Private vulnerability reporting
  was DISABLED**, while `SECURITY.md`, `CODE_OF_CONDUCT.md` and two issue-chooser links all sent
  reporters to that form — reachable only by someone who can already create a draft advisory, so the
  entire audience for both documents had no route. 14.9 rejected the GitHub `noreply` address for
  being a contact that receives nothing and then chose a channel that was switched off: **the
  reasoning was right and the verification stopped one level too early.** Third instance of the
  governance-drift lesson — **a document pointing at a configuration makes a claim no gate here can
  see.** Also: the roadmap's "three unapplied §4 items" were **undocumented, not unapplied** (§4
  never mentioned description/topics/homepage), so the fix is a new §4.1 as much as a `PATCH`. Pages
  **costed and declined** — 30 of 579 links escape `docs/` and they are exactly the root governance
  files Pages cannot serve; the curated version is registered in ADR-0057 §B, which made ADR-0058 the
  **new ledger gate's first real exercise, one day old**. `v0.1.0`'s Release drafted with
  **`--latest=false`** — the flag that stops a backfill stealing the Latest badge from `v2.0.0`.
- [2026-08-08 — 14.10: the ledger that would have proposed work already shipped](2026/08/2026-08-08-additive-capability-ledger.md) —
  roadmap 14.10, [ADR-0057](../adr/0057-additive-capability-ledger.md). Both of the roadmap line's
  numbers were wrong: not "roughly twenty capabilities in sixteen ADRs" but **49 open capabilities
  across 26 ADRs** — and **eleven deferrals had already been built without the deferring ADR ever
  saying so**, so assembling the ledger the obvious way (grep `deferred`, transcribe) would have
  published as future work capability the module already exports. Each entry was checked against the
  **source** instead, which also found two that are not additive at all (`*Pool` as `io.Closer`,
  argon2id as the default) and belong to a future major. **The trigger column is the item**: falsifiable
  evidence over six kinds, never "when it becomes important" — and **exactly one trigger has fired**
  (`env`'s missing float getter, found by *writing* `examples/service`, not by reading the surface),
  so a milestone built from §A today would be one entry long, which is the ledger working rather than
  disappointing. Gated by `consistency_lint.py` check 11, verified by deliberate violation **and** by
  printing the sets it sees (13 marked, 36 cited, neither empty — an empty input passes vacuously,
  which is 14.7's hole); the marker's blind spot over the other 13 ADRs is documented rather than
  retrofitted away.
- [2026-08-08 — 14.9: the contact that looks real and delivers nothing](2026/08/2026-08-08-contributing-and-conduct.md) —
  roadmap 14.9; `CONTRIBUTING.md` + `CODE_OF_CONDUCT.md`, no ADR (0057 is reserved for 14.10). The
  Covenant needs a contact and the obvious one was a decoy: every commit carries
  `…@users.noreply.github.com`, already public and already the maintainer's, and it **accepts no
  incoming mail** — a contact that looks real and silently discards reports is worse than a missing
  one, because the reporter believes they have reported. The maintainer chose the private
  security-advisory form, and the security/conduct mismatch is **designated in writing three times**
  (Code of Conduct, issue chooser, `SECURITY.md` from the receiving end) so a reviewer finds a
  decision rather than a copy-paste error; a report about the maintainer routes to GitHub Support
  instead. `CONTRIBUTING.md` is written against what a contributor gets wrong — the `gofumpt@latest`
  trap that fails CI on a diff they never wrote, and the one-item-at-a-time rule given its mechanism
  (a squash merge leaves the branch as no ancestor of `master`) rather than as an instruction. §7
  refuses a feature-request template in favour of four evidence-shaped questions feeding 14.10's
  ledger. Also corrected: **`SECURITY.md` still promised `0.x` support "until `v1.0.0`"**. A drafted
  SPDX instruction was deleted after checking — **0 of 116 Go files carry one**.
- [2026-08-06 — 14.8: the job was already running, and the number it published was not a latency](2026/08/2026-08-06-nfr-tails-on-linux.md) —
  roadmap 14.8, [ADR-0037](../adr/0037-nfr-benchmark-methodology.md) amended, the
  [NFR report](../benchmarks/2026-07-26-nfr-suite.md) updated in place. Both premises of the roadmap
  line were false. **There was no job to add** — `nfr-nightly` had been publishing both tail
  percentiles on Linux since 10.10, into an artifact nobody opened for eleven nights, which is 14.7's
  lesson one layer out: **a job that runs is not a number that has been read.** **NFR-02's `Submit` p99
  is met at 176 ns against 2 µs**, conservatively, since `ThroughputCounted` proves the pipeline is
  consumer-limited and the tail includes back-pressure the NFR excludes. **NFR-06's p99 is not met at
  887 ns against 200 ns** — and the figure that mattered was not the 8-goroutine one: a wall-clock batch
  timed inside one of N goroutines on M cores with N > M measures **residency**, so 97.1 ns/op aggregate
  and a 743 ns/op batch p50 are the same work seen twice. `GetTailPerCore` removes the scheduling and
  the shortfall survives, so it is not a hardware gap. The same arithmetic makes 10.11's **"met at the
  mean, 46.6 ns" a throughput figure read as a latency**; ADR-0038's sharding result stands, the latency
  reading does not, and whether the 200 ns target means the uncontended `Get` (32.9 ns) or the loaded one
  (~775 ns) is handed to the maintainer as a spec question.
- [2026-08-05 — 14.7: the half-pinned repository, one write token, and a gate that could not see its own subject](2026/08/2026-08-05-supply-chain-pinning.md) —
  roadmap 14.7, [ADR-0056](../adr/0056-build-time-supply-chain.md), control C-6. Every action pinned
  to a commit digest (**21 of 36 floated**, including `actions/checkout` at two sites in a file that
  pinned it at eleven others — so the half-pinning was copy-paste, not three decisions), no
  workflow-level token grant so a later job cannot inherit one, and a reproducible CycloneDX SBOM as
  **the project's first release artifact** — all four prior releases have zero assets, so
  `release.md` step 10 had been false since v0.1.0. **The provenance claim is deliberately scoped to
  the SBOM and not to the module**, because what a consumer resolves is already anchored in
  `sum.golang.org`, and asserting more would put a weaker claim beside a stronger one. Every SBOM flag
  was decided against output: no `-test` yields **exactly ADR-0004's three runtime dependencies**,
  `-noserial -notimestamp` makes it byte-identical across runs (now gated by `cmp` per PR), and
  `-licenses` was **reversed on measurement** — it is wrong on all three components. **The signing
  premise in the roadmap was false**: `master`'s commits are already `verified=true`, signed by
  GitHub's web-flow key as a property of squash-only merging, so `required_signatures` is free and
  only signed *tags* are declined. And the tenth deliberate-violation case was a blind spot in the new
  check itself, which could not see a scope carrying a trailing comment and so passed green on the one
  job it exists to police — ADR-0043's 12.1 lesson in a new place.
- [2026-08-04 — 14.6: the parse is the job, and the Release that was refused](2026/08/2026-08-04-contrib-release-workflow.md) —
  roadmap 14.6, [ADR-0055](../adr/0055-contrib-release-workflow.md). `contrib-release.yml` — a
  `contrib/<name>/vX.Y.Z` tag is verified by CI instead of by hand, closing the hole `release.md`
  recorded three days earlier. **The parse from tag name to module directory is the job, not a
  preamble to it**, because a wrong answer would not fail — it would `go build ./...` in the
  repository root, pass, and report a green submodule release for a module nobody touched. Five
  refusals, all eight cases run rather than read; the leading character class in the directory regex
  is load-bearing, since `contrib/../v1.0.0`'s `go.mod` test resolves to the *root* module. The sixth
  check is the one that cannot be re-run: **reachability from `master` is the mechanical form of "the
  contrib CI job green on the commit being tagged"** — and ancestry is the right test here even though
  the 2026-08-01 audit proved it the wrong test for a merged branch. The job **repeats what would make
  the module broken for a consumer and not what would make it merely untidy.** **And it drafts no
  GitHub Release, on an argument that is not discoverability: an unpublished tag is a recoverable
  mistake and a published Release is not** — verification necessarily happens after the push, so the
  remedy is delete-and-repush, and a Publish checkpoint would attach itself to the very artifact whose
  deletability is that remedy. Falls out for free: `contents: read`, and no way to manufacture
  `v0.1.0`'s missing-Release asymmetry.
- [2026-08-04 — 14.5: the boundary that fails silently, and the floor that was not applied](2026/08/2026-08-04-examples-service-module.md) —
  roadmap 14.5, [ADR-0054](../adr/0054-examples-service-module.md). `examples/service` — one HTTP service
  composing eight packages, as a module of its own. **The silent-failure claim was verified and it is
  worse than in `contrib`**: with the `go.mod` moved aside, `go list ./...` reports the package *and*
  `go list -deps ./...` still succeeds, because everything a composition example imports is already in
  the core's graph — so the boundary can vanish with no error of any kind. **depguard is a second net
  that fires accurately and diagnoses wrongly** ("feature packages do not import each other", a rule
  nobody violated), which is the argument for short-circuiting on the cause. The check was generalized
  over `contrib` + `examples` and now walks **recursively**, closing a hole the old top-level listing
  had; and ADR-0040's "no `replace`, no `go.work`" — prose since 2026-07-27 — became three assertions,
  all confirmed by deliberate violation. **A new CI job rather than a renamed matrix, because
  `contrib / <module>` is a required status check and a renamed required context blocks a PR forever
  instead of failing it.** **The coverage decision is no floor**, measured before deciding: the module
  is at 56.2% because `main()` is 17 of 48 statements and none are reachable, while `service.go` is at
  87.1% and clears the gate alone — so CI **runs** it instead, ADR-0053 rule 2's mechanism
  transferred. One quotable number: **one `require` line, zero indirect requirements, 203 packages
  resolved of which zero are third-party.**
- [2026-08-02 — 14.4: the driver that is not there, and the set that finishes](2026/08/2026-08-02-examples-config-core.md) —
  roadmap 14.4. Twenty-nine examples across config, env, cache, db, validator, hash, syncpool and errx —
  the set is complete at **55 across all 21 packages**. **The `db` fork dissolved on one observation**:
  `database/sql/driver` is standard library, so a 35-line stub connector costs no dependency and ADR-0004
  was never in the way — and because the stub counts commits and rollbacks, the three examples *assert*
  the finalization contract, including the panic path where `recover()` sees the original value with the
  rollback already done. **`hash` was budgeted rather than avoided**: five bcrypt operations, 0.74 s
  measured, and `ExampleCost`'s explicit cost 10 is what a legacy store honestly contains. gosec's `G101`
  on a DSN literal was fixed by printing what expansion *did* rather than by a `//nolint`. Two rule-3
  judgements are recorded because they look like violations — `errx` prints a composed message whose every
  word is the example's own, and asserts a frame by function suffix, not by path or line.
- [2026-08-01 — 14.3: the examples that assert a security property, and the one that had to say less](2026/08/2026-08-01-examples-http-observability.md) —
  roadmap 14.3. Twelve examples across middleware, health, metrics, logger and lifecycle, with nothing
  re-decided — ADR-0053 held. **Rule 3 turned out to be the design constraint**: `slog` and the Logger
  middleware emit a moving `time` and `duration`, so those examples decode the line and print only the
  stable fields, which documents the schema better than dumping it would; `metrics` filters the scrape to
  the counter because histogram buckets depend on real latency. **Two examples assert a security property
  rather than a happy path** — Recoverer's prints `false` for "is the password in the body", health's
  shows a 503 naming *which* check failed and never why — which is where an example beats a promise in
  prose. **`lifecycle` was answered by writing less**: `WaitForSignals` blocks on a signal that will never
  come, so it is documented *as prose inside the example* while the runnable part proves LIFO order; the
  package is the module's only process-wide singleton, so the file tells the next contributor to keep this
  the only example, and the coupling was checked with `-shuffle` on four seeds. revive's unused-`ctx`
  finding was fixed by making the probe **honour** the context, not by hiding the parameter.
- [2026-08-01 — 14.2: documentation that runs, and the rule the unexported clocks forced](2026/08/2026-08-01-examples-convention.md) —
  roadmap 14.2, [ADR-0053](../adr/0053-runnable-examples-convention.md). Thirteen examples across eight
  packages, and the convention was the item. **The rule I did not get to choose:** the fake clocks that
  make the tests deterministic are **unexported fields**, so rule 1 (external test package) puts them out
  of an example's reach — two independently-chosen rules collided, and exporting a seam was refused twice
  over (M14 promises no identifier; a knob whose only caller is documentation ships forever). The
  replacement is honest instead of clever: a one-minute breaker timeout, `BaseDelay: 0`, a bucket that
  starts full, an `ordering` guarantee via a `started` channel, and a semaphore whose bound is proved by
  the example terminating. **`go test` compiles an `Example` with no `// Output:` and never runs it**, so
  all thirteen were confirmed with `-v` rather than a package-level `ok`. Whatever an example prints is
  enforced forever, hence shape and never strings. Two traps found by writing it: pubsub's **16-message
  default buffer** is the only reason a sequential example works (ADR-0049 — otherwise every message
  drops and the example hangs), and an empty header value leaves a **trailing space** the comparator does
  not trim per line. Recorded cost: a nondeterministic behaviour can be described but not shown, so cache
  TTL stays prose in 14.4.
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
