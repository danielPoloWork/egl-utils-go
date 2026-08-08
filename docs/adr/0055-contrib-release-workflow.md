# ADR-0055: verifying a `contrib/*` release on its tag — the parse is the job, and no GitHub Release is drafted

- **Status:** Accepted
- **Date:** 2026-08-04
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** roadmap 14.6 (this item); [ADR-0040](0040-contrib-submodules.md) (the submodule
  topology and the independent tag line this workflow serves);
  [ADR-0054](0054-examples-service-module.md) (the same topology for `examples/*`, and the
  required-status-check trap); [ADR-0045](0045-pkg-layout-and-v2.md) (a nested module carries its
  own major); [ADR-0035](0035-import-graph-enforcement.md) (the in-repo gate that already refuses a
  `replace` or a missing `go.mod`); `docs/workflow/release.md` (the hand procedure this replaces);
  roadmap 14.7 (action pinning and least-privilege `permissions:`, which this file is written to
  suit)

## Context

Releasing a `contrib/*` submodule is a separate act from releasing the core — that is ADR-0040's
whole point, and it is the reason the tag is `contrib/<name>/vX.Y.Z` rather than a core version.
The consequence was recorded on 2026-08-01, after the first two submodule releases, in
`docs/workflow/release.md`:

> **`release.yml` does not fire.** It triggers on `tags: ["v*.*.*"]`, which a `contrib/…` ref does
> not match — so there is **no draft GitHub Release and no CI run on the tagged tree**. Verify
> before tagging instead: from the submodule directory, `go build ./... && go vet ./... &&
> go test ./...`, plus the `contrib` CI job green on the commit being tagged.

So `contrib/redishealth/v0.2.0` and `contrib/pgxhealth/v0.2.0` were verified **by a human running
four commands from memory of a document**. Nothing mechanical would have stopped a broken one, and
a submodule tag is unusually easy to get wrong in ways the core's is not:

- the version is the *module's* own, so there is no `version.go`, no changelog row and no
  `consistency_lint.py` lockstep check to disagree with it — the tag is the only statement of the
  version anywhere;
- the tag name doubles as a **path**, so a typo does not produce an error, it produces a different
  module;
- from `v2` upward Go requires the major in the module path, so `contrib/x/v2.0.0` against a
  v1-shaped `go.mod` is a tag the module proxy refuses **after** it has been pushed;
- and a submodule is released precisely when its *driver* moves, which is Dependabot's cadence
  rather than a milestone's — the releases that most need a mechanical gate are the ones nobody
  plans.

The item named two decisions and no more. This ADR records both.

## Decision

**A new `.github/workflows/contrib-release.yml`, triggered on `tags: ["contrib/*/v*.*.*"]`, which
verifies the tagged submodule and drafts nothing.**

### (a) The tag pattern, and the parse that derives the module from it

GitHub's ref filters treat `*` as "any character except `/`", so `contrib/*/v*.*.*` matches
`contrib/redishealth/v0.2.0` and cannot match the core's `v2.0.0` — the two release workflows
partition the tag space rather than overlapping on it.

Everything the job then does happens **in a directory derived from a tag name**, and that is why
the parse is the job rather than a preamble to it: a wrong answer would not fail, it would run
`go build ./...` in the repository root, pass, and report a green submodule release for a module
that was never touched. The step therefore validates and **exits non-zero naming the value it
rejected** at each of five points:

1. the directory is one path segment under `contrib/` matching
   `^contrib/[A-Za-z0-9][A-Za-z0-9._-]*$` — the leading character class is what rejects
   `contrib/../v1.0.0`, whose `go.mod` test would otherwise resolve to the root module;
2. the remainder is SemVer, pre-release and build metadata included;
3. **that directory contains a `go.mod`** — the loud failure the roadmap item asked for by name. It
   also prints the module directories that *do* exist at that commit, because the realistic cause is
   a misspelling and the useful output is the list to spell it from;
4. the `module` line in that `go.mod` is exactly
   `github.com/danielPoloWork/egl-utils-go/<dir>`, so a tag cannot name a directory whose module
   declares itself as something else;
5. and for a major of 2 or more, that path must carry the `/vN` suffix Go requires.

Each of the five was confirmed by running the parse over a table of eight tags — the two real ones,
a legal `v1.0.0`, a legal pre-release, and the four failures above — rather than by reading it.

### The reachability check, which is how the tag inherits the quality gate

`ci.yml` runs on every push to `master`, and `contrib / redishealth` and `contrib / pgxhealth` are
two of `master`'s thirteen required status checks *(fourteen since 2026-08-08, when
`examples / service` was added; the two `contrib` contexts and this argument are unaffected)*. So
`git merge-base --is-ancestor $GITHUB_SHA
origin/master` is the mechanical form of "the `contrib` CI job green on the commit being tagged" —
the second half of the hand procedure, which no amount of re-running commands on the tag can
establish, because it is a claim about *gating*, not about passing.

Ancestry is the correct test here even though it is the wrong test for a merged branch: squash
merges leave no ancestor, which is why the 2026-08-01 audit had to check merged branches against the
pull-request list instead. The commit being tagged is not a branch tip — it is a commit on `master`.

Given that check, the job **re-verifies what would make the released module broken for a consumer
and does not re-verify what would make it merely untidy**: `go build`, `go vet`, `go test -race` and
`go mod verify` run again from the module directory; gofumpt, golangci-lint and govulncheck do not.
The build and test re-run is redundant with `ci.yml` in the ordinary case and kept anyway, so that a
tag's verification is self-contained rather than an inference across branch-protection state that
lives outside the repository — the class of drift this project has already been burned by. A
formatting violation, by contrast, is not a reason to unpublish a version.

### (b) No GitHub Release for a submodule — the annotated tag stays the record

This confirms today's behaviour as a decision rather than leaving it as an absence, and the reason
is not discoverability, which a Release would genuinely improve. It is that **an unpublished tag is
a recoverable mistake and a published Release is not.**

`release.md`'s boundary already says agents "only delete-and-repush an *unpublished* tag whose
release run visibly failed". That remedy is exactly what a red run of this workflow calls for: the
verification necessarily happens *after* the tag is pushed, so its only possible output is "this tag
is wrong" — and the response has to be to remove it. Drafting a Release would attach a
human-Publish checkpoint to the artifact whose deletability is the remedy, and a maintainer who
publishes before reading a red run has converted a fixable typo into a permanent one.

Two secondary consequences of the same choice, both worth having:

- the workflow needs **no write permission at all** (`permissions: contents: read`, against
  `release.yml`'s `contents: write`), which is 14.7's principle arriving early and for free;
- and it cannot create the asymmetry 14.11 has to clean up. `v0.1.0` has a git tag and no Release
  while four later core tags each have one, precisely because a Publish step was skipped once.
  Adding a per-driver-bump Publish step to the module line that releases most often is a machine
  for manufacturing that hole.

> *Amended 2026-08-08: **the hole is filled.** 14.11 backfilled `v0.1.0`'s Release from
> `docs/releases/v0.1.0.md`, so all five core tags now have one. Nothing here is superseded — the
> argument was that a skipped Publish step leaves an artifact only a human can restore, and
> restoring it by hand four weeks later is what that costs.*

What replaces the Release as a record is the workflow run: the final step writes the tag, module
path, version, directory and commit into `$GITHUB_STEP_SUMMARY`, together with the
`go list -m <path>@<version>` command that confirms what a consumer resolves.

## Alternatives Considered

- **Generalize `release.yml` to `tags: ["v*.*.*", "contrib/*/v*.*.*"]` instead of adding a
  workflow.** Fewer files, and the checkout/build steps are nearly the same. Rejected because the
  two acts differ in the one thing a release workflow is *for*: the core drafts a Release and needs
  `contents: write`; a submodule drafts nothing and needs `contents: read`. Merging them would mean
  the core's write token is held during every submodule tag, and every step in the file would have
  to branch on which kind of tag fired — including the ones that read `version.go` and the changelog,
  which do not exist for a submodule. Two triggers over disjoint tag spaces is the honest shape.
- **Call `ci.yml` via `workflow_call` on the tag.** Tempting: it would re-run the exact `contrib`
  job that gates the pull request, so there would be one definition of "green" instead of two.
  Rejected because `ci.yml` is a full core gate — coverage, fuzzing for ten minutes, the four-way
  build matrix, the three policy tools — and none of it says anything about the submodule being
  tagged. It also runs the matrix for *both* modules, so the tag for one would be blocked by the
  other's unrelated failure, and it has no way to receive the tag parse, which is the part that
  actually needs to exist.
- **Verify before the tag, in a `workflow_dispatch` a human runs first.** This is the strongest
  alternative, because it is the only one that can prevent a bad tag rather than report it. Rejected
  on who it depends on: it reintroduces the step whose being skippable is the whole finding — the
  hand procedure was already written down and was already the plan. A gate that fires on the tag
  cannot be forgotten, and combined with the delete-and-repush remedy the outcome is the same
  minus the discipline.
- **Assert `go list -m <path>@<version>` resolves through the proxy, as the last step.** It is the
  claim a consumer actually cares about and it would close the loop. Rejected on timing rather than
  on value: the module index takes minutes to see a new tag, and the proxy caches its version list
  independently, so this step would fail on lag far more often than on a real fault — a flaky gate
  on a release path teaches people to re-run red jobs. It stays in the run summary as the command
  to run by hand, where the person reading it can tell "not yet" from "wrong".
- **`go mod tidy -diff` in this job, as the `examples / service` job does.** Rejected because it
  checks a different property here. `examples/service` has one requirement and no indirect ones, so
  a tidy diff there means a real mistake; a `contrib` module's `go.mod` carries a driver's full
  indirect closure, where a diff can appear from a toolchain's tidy behaviour rather than from the
  module being wrong — and failing a *release* on that is a stop with no correct action behind it.
  `go mod verify`, which checks the recorded sums, is the part that matters for a consumer.
- **Draft a GitHub Release for each submodule tag, with `generate_release_notes: true`.** The
  discoverability argument is real: a Release is what a human browsing the repository finds, and
  `contrib/*` versions are currently invisible unless you know to run `git tag -l` or query the
  proxy. Rejected on the deletability argument above, and on a second point — auto-generated notes
  for a submodule tag are notes about *everything* merged since the previous `contrib/<name>/…` tag,
  which for a module whose releases are driver bumps is mostly the core's unrelated work. The
  annotated tag message, written by hand at the moment of the decision, says more in one line. If
  submodule discoverability becomes a real complaint, the cheaper answer is a table in
  `contrib/README.md`, not a release ceremony.
- **Make this workflow a required status check.** Rejected as a category error: it fires on tag
  pushes, and branch protection gates pull requests. There is nothing for it to be required on.
- **Pin `actions/setup-go` to a SHA here, ahead of 14.7.** Rejected deliberately so the sweep stays
  one change: this file uses `actions/checkout` pinned to the SHA already in use and `setup-go@v7`
  floating, exactly as `ci.yml` and `release.yml` do. A half-pinned repository is the finding 14.7
  opens with; a repository where one new file is pinned differently from its siblings is the same
  finding plus an inconsistency to explain.

## Consequences

- **The hand procedure in `release.md` becomes a fallback rather than the method.** Its `contrib`
  section is rewritten to describe the workflow, what each of its checks refuses, and the
  delete-and-repush remedy for a red run. The boundary table gains no row: the agent still tags and
  pushes, and there is still no Publish step to reach.
- **The gate is after the fact, and that is stated rather than hidden.** A tag can only be verified
  once it exists, so a bad tag is caught, not prevented. The recovery is bounded because nothing is
  published: delete the tag, fix, re-tag. The only irreversible case is a *correct-looking* tag that
  the module index has already served — which is precisely what the five parse checks exist to make
  unreachable, since every failure mode they cover produces a module path or a directory that is
  wrong on its face.
- **The `examples/*` modules are not covered, by construction.** They are never tagged (ADR-0054),
  so the tag space partitions into exactly two: the core's `v*.*.*` and `contrib/*/v*.*.*`. If an
  `examples` module is ever released, that is a decision to reopen ADR-0054 with, not a pattern to
  extend this file by.
- **A pre-release tag is accepted.** `contrib/x/v0.3.0-rc.1` parses, verifies, and drafts nothing —
  the same as any other submodule tag. No submodule has used one; the shape is legal because
  refusing it would be a policy this project has not decided, and inventing one in a regex is the
  wrong place for it.
- **The workflow has never run.** Both existing submodule tags predate it and workflows do not run
  retroactively, so the first real exercise is the next `contrib/*` release. Its logic was therefore
  validated the only way available: the parse was run locally over the eight-tag table above, and
  the YAML was parsed to confirm the trigger, the single job and its seven steps.
- **One adjacent defect noticed and left alone:** `release.yml`'s `setup-go` step reads
  `go-version: ${{ matrix.toolchain == 'go-1.25' && '1.25' || '1.26' }}` in a job that has no
  matrix, as does `ci.yml`'s `benchmark` job. Both evaluate to `'1.26'` and are harmless, but they
  claim a matrix that does not exist. Not fixed here — 14.7 edits every step of both files, and a
  one-line cleanup landing in a `contrib` PR is a change reviewers have no reason to expect.

## References
- ROADMAP 14.6; `docs/workflow/release.md` (the `contrib` procedure, rewritten by this item, and the
  boundary table); ADR-0040 (submodule topology, independent tags, the per-module major);
  ADR-0054 (the same topology for `examples/*`, and why a required status check must never be
  renamed).
- `.github/workflows/contrib-release.yml` (this decision), `.github/workflows/release.yml` (the
  core's, whose tag filter this complements), `.github/workflows/ci.yml` (the `contrib` job whose
  green result the reachability check inherits).
- Go module rules relied on: a nested module's tag is `<dir>/vX.Y.Z`; from `v2` the major belongs in
  the module path; the module index and the proxy see a new tag on their own schedule.
- GitHub Actions rules relied on: a ref filter's `*` does not match `/`; `permissions:` at workflow
  level applies to every job; a step's `working-directory` accepts an expression, which is what lets
  the parsed directory drive the later steps.
