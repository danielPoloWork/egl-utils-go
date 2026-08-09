# 2026-08-09 — a release review board, and the promise the README could not keep

Seven specialist reviewers evaluated `master` as a release candidate against `v2.0.1`. The verdict
was **not approvable**, on one sentence in the README. The tracker went from zero issues to 43.

## How the board was run

Seven members, one discipline each — release engineering, site reliability, product security, test
engineering, API review, technical writing, programme management. Each worked **blind to the
others**: no shared findings, no consensus round, no round-robin. Disagreement between two members
was treated as data to adjudicate, not as a problem to smooth over.

Two rules did most of the work:

**The decision rule was frozen before any report was read.** `≥1 surviving BLOCKER ⇒ not
approvable`; `0 BLOCKER and (≥3 MAJOR or ≥2 MAJOR from ≥2 disciplines) ⇒ approvable with
conditions`; otherwise approvable. Written down first so the verdict would be the application of a
rule rather than a summary assembled to fit whatever came back.

**An unverifiable mandatory gate counts as failed, never as passed.** Silence is not a pass.

Both mattered. The second is why the CI evidence was checked at *step* granularity rather than
accepted as a green tick, and the first is why a single evidenced BLOCKER outranked six
`APPROVE_WITH_CONDITIONS` verdicts — the six had not looked at what the one looked at.

## The BLOCKER

`README.md:18` promises **"no package here imports another"**, restated at `:105-107` and in
`CHANGELOG.md:31-32`. It is false. `pkg/config/config.go:47` imports `pkg/validator` — the only
intra-module edge, verified exhaustively rather than sampled — and
`tools/import_graph_lint.py:98-100` sanctions that edge **by name**, with `:211-216` failing the
build if it ever disappears. The project *mandates* the exception its front page denies. Worse,
**ADR-0035, cited on the page as the authority for the promise, is the document that records the
edge**: a reader who follows the link finds the contradiction.

The runtime harm is small — no extra external dependency, and "three runtime dependencies in total"
stays true. **The severity comes from permanence and cost asymmetry.** `README.md` carries no
nested `go.mod`, so it travels inside the published module zip, and Go module versions are
immutable in `sum.golang.org`. One sentence before a tag; a whole new version after.

## The finding that inverts 14.12's reasoning

`v2.0.1` was argued on the grounds that **the tag is the payload** — pkg.go.dev renders from the
tagged tree, so 55 examples sat invisible in `master` until a tag existed. That was true, and it is
true only of Go code.

**pkg.go.dev renders Go doc comments from a published version. It does not render `README.md`, or
any Markdown in the module.** So for this delta:

| | |
|---|---|
| The README rewrite | already reached consumers when #105 merged — GitHub serves the default branch |
| `docs/usage/README.md`, 438 lines | **will never reach pkg.go.dev**, with or without a tag |
| A `v2.0.2` documentation page | would be **byte-identical** to `v2.0.1`'s |

The only `.go` file in the whole delta is `examples/service/service_test.go`, in a separate module
absent from the core module's zip. A tag would deliver nothing to any consumer while still billing
every downstream a Dependabot PR and a re-scan. **The board's recommendation is not to tag**, and
to fold `[Unreleased]` into the next functional release.

The reusable form: *the argument that justified the last release is not automatically available for
this one — check which of its premises still holds.*

## Three adjudications, and what each taught

**A BLOCKER that was really a precondition.** Release engineering filed "tagging `9ea5460`
publishes a version declaring itself 2.0.1" as a BLOCKER. The evidence was correct and the severity
was not: `release.md:27-28` puts the bump inside the release PR, so `master` between releases is
*supposed* to look exactly like that. Reclassified as a precondition of the release act, not a
defect in the work, and recorded as a downgrade with its reason.

**Read the whole `govulncheck` block, never one line.** A member filed "1 vulnerability in modules
you require" as MAJOR. The same block reads `No vulnerabilities found.` and `Your code is affected
by 0 vulnerabilities.` — the quoted phrase is the **non-reachable bucket**, which is precisely the
distinction the tool exists to draw. Downgraded to MINOR ([#131](https://github.com/danielPoloWork/egl-utils-go/issues/131)).
The real residual is narrow and worth keeping: nothing names the module or the CVE, so a change
that made it reachable would flip the line's meaning with nobody watching. `-show verbose` closes it.

**BUG-0002's sweep was scoped to the defect's syntax, not its lesson.** Two members split: one said
the "repository-wide sweep found no third instance" claim held, the other said library code had
never been swept. The record settles it — the heading claims *repository-wide*, the sentence
supporting it covers only `_test.go` files. Non-test library code has **four** non-blocking `select`
sends: `pkg/pubsub/pubsub.go:162,178,189` — a documented policy, every loss reported through
`reportDrop`/`ErrSlowSubscriber` — and `pkg/workerpool/workerpool.go:103`. That last one is
reachable from the public API as `workerpool.New(n, 0, WithNonBlockingSubmit())`, since `queueSize
0` is legal and documented as "direct hand-off", and **no test covers it**. It fails *closed* with
`ErrQueueFull` rather than hanging, so it is not BUG-0002 again — but the pool then sheds by
scheduler timing rather than by load, producing 503s no capacity metric explains.

## What the board could not coordinate, and therefore counts

Members were blind to each other, so convergence is evidence rather than echo.

- **`README.md:90-91` was hit by three disciplines with three different arguments**: the
  `ListenAndServe` error is discarded, leaving a process alive and silent (SRE); it teaches exactly
  what `examples/service/main.go:95-108` forbids in writing (API review); no `ReadHeaderTimeout`
  means Slowloris exposure with gosec G112 enabled (security). **The quickstart is a production
  template people paste — it earns the same review as production code.**
- **The retry recipe** was found by two. The technical writer supplied the cause: the *code* was
  derived faithfully from `pkg/retry/example_test.go`, and only the comment that made it safe to
  show was left behind — a failure mode no gate catches, because the guide's snippets are fragments
  CI never compiles.

## What came back genuinely green

Worth recording, because most of the report is what is wrong. Run `31278259456` on `9ea5460`:
`success`, 14/14 jobs, mapping one-to-one onto the 14 required contexts, verified at step
granularity. `-race` four times; coverage **enforced**, not printed (23 packages, all ≥ 85 %); all
**55 `Example*` functions carry `// Output:`** and therefore execute; 21 packages bijective in both
directions; **354 relative links across 22 changed files, 0 broken**; 37/37 actions digest-pinned
with version comments; signing verified on both halves — the setting *and* the commits; the SBOM
attestation real and not overclaimed. Compatibility was proven by **blob and tree hash**, not by
diffstat: consumers of `…/v2` need do nothing.

## The tracker, and a tension left open

All 43 actionable findings were filed as issues `#106`–`#148` — into a tracker that had held zero
issues in the project's life. Deliberate choices in doing so: **no milestone on any of them** (M15
does not exist, and that is a decision), and **no new label**, because `.github/labels.yml` manages
labels as code and creating one out of band desynchronises the file that declares them.

That leaves a question this board did not answer. `docs/bugs/README.md` declares itself *the source
of truth for defects*; `ROADMAP.md` holds planned work; these 43 issues are now a third register,
indexed by [`ISSUES.md`](../../../../ISSUES.md). The index says in its own header that it is a
pointer and never the record — but if the tracker is where this work is going to live,
`docs/bugs/README.md` and `AGENTS.md` §7 should say so, before a future maintainer has to guess
which of three places is authoritative.
