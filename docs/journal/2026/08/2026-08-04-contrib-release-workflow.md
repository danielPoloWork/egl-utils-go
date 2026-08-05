# 2026-08-04 — 14.6: the parse is the job, and the Release that was refused

Roadmap **14.6**, [ADR-0055](../../../adr/0055-contrib-release-workflow.md). Milestone 14 is now
**6/12**. Closes the hole `docs/workflow/release.md` recorded on 2026-08-01: `release.yml` fires on
`tags: ["v*.*.*"]`, a `contrib/<name>/vX.Y.Z` ref does not match it, and so the first two submodule
releases were verified by a human running four commands from memory of a document.

## What shipped

`.github/workflows/contrib-release.yml`, on `tags: ["contrib/*/v*.*.*"]`. One job, seven steps,
`permissions: contents: read`.

The item asked for two decisions and no more, and **the second one reversed the item's own title**:
the workflow verifies and drafts nothing.

## (a) The tag pattern and the parse

The filter works because **a GitHub ref filter's `*` does not match `/`** — so `contrib/*/v*.*.*`
matches `contrib/redishealth/v0.2.0` and cannot match the core's `v2.0.0`. The two release workflows
*partition* the tag space rather than overlapping on it, which is also why merging them into one file
was rejected: the core drafts a Release and needs `contents: write`, a submodule drafts nothing and
needs `contents: read`, and every step in a merged file would branch on which kind of tag fired.

The parse from `github.ref_name` to a module directory **is** the job rather than a preamble to it,
for a reason worth stating plainly: a wrong answer would not fail. It would run `go build ./...` in
the repository root, pass, and report a green submodule release for a module nobody touched. So there
are five refusals, each exiting non-zero naming the value it rejected:

1. one path segment under `contrib/`, matching `^contrib/[A-Za-z0-9][A-Za-z0-9._-]*$`;
2. a SemVer remainder, pre-release and build metadata included;
3. **a `go.mod` in that directory** — the loud failure the item asked for by name. It also prints the
   module directories that *do* exist at that commit, because the realistic cause is a misspelling
   and the useful output is the list to spell it from;
4. a `module` line equal to `github.com/danielPoloWork/egl-utils-go/<dir>`;
5. the **`/vN` suffix Go requires from `v2` upward** — the check a hand-written tag most needs, since
   `contrib/x/v2.0.0` against a v1-shaped `go.mod` is a tag the proxy refuses *after* the push.

**The leading character class in the directory regex is load-bearing.** It is what rejects
`contrib/../v1.0.0`, whose `-f "$dir/go.mod"` test resolves to the *root* `go.mod` and therefore
passes; without it the only objection left is the module-path comparison, which is a much later and
much less obvious place to catch a traversal.

All eight cases were **run rather than read** — the two real tags, a legal `v1.0.0`, a legal
`v0.3.0-rc.1`, and the four failures (missing module, wrong major suffix, two path segments, no
`v` prefix).

## The sixth check, which is the one that could not be re-run

`git merge-base --is-ancestor "$GITHUB_SHA" origin/master`.

`ci.yml` runs on every push to `master`, and `contrib / redishealth` / `contrib / pgxhealth` are two
of `master`'s thirteen required status checks. So reachability is the **mechanical form of "the
contrib CI job green on the commit being tagged"** — the second half of the old hand procedure, and a
claim about *gating* that no amount of re-running commands on the tag can establish.

Worth flagging against the 2026-08-01 audit, which proved ancestry the **wrong** test there:
squash merges leave no ancestor, so `git merge-base --is-ancestor` called all 63 merged branches
unmerged and the audit had to check against the pull-request list instead. That does not apply here.
The commit being tagged is not a branch tip — it is a commit on `master`.

## The line the job draws instead of copying CI

**It repeats what would make the released module broken for a consumer, and not what would make it
merely untidy.** `go build`, `go vet`, `go test -race`, `go mod verify` — yes. gofumpt,
golangci-lint, govulncheck — no: those are properties of the tree, already gated per pull request,
and a formatting finding is not a reason to unpublish a version.

The build-and-test repeat is redundant with `ci.yml` in the ordinary case and kept anyway, so a
tag's verification is **self-contained rather than an inference across branch-protection state that
lives outside the repository** — the class of drift this project was burned by three days ago.

`go mod tidy -diff` was refused here although the `examples / service` job runs it, and the
distinction is real: that module has one requirement and no indirect ones, so a diff there means a
mistake, while a `contrib` module carries a driver's full indirect closure where a diff can come from
tidy behaviour rather than from the module being wrong. Failing a *release* on that is a stop with no
correct action behind it.

## (b) No GitHub Release — and the argument is not discoverability

A Release would genuinely improve discoverability; `contrib/*` versions are invisible today unless
you run `git tag -l` or query the proxy. The decision rests on something else:

> **An unpublished tag is a recoverable mistake and a published Release is not.**

Verification necessarily happens *after* the push, so this workflow's only possible output is "this
tag is wrong", and the only response is to remove it — which `release.md`'s boundary already permits
for an unpublished tag whose release run visibly failed. Drafting a Release attaches a human Publish
checkpoint to the very artifact whose deletability is the remedy, and a maintainer who publishes
before reading a red run has converted a fixable typo into a permanent one.

Two consequences fall out for free:

- the workflow needs **`contents: read`**, where `release.yml` needs `contents: write` — 14.7's
  least-privilege principle arriving early and at no cost;
- and it **cannot manufacture the asymmetry 14.11 has to clean up.** `v0.1.0` has a git tag and no
  Release precisely because a Publish step was skipped once; adding one to the module line that
  releases most often — driver bumps on Dependabot's cadence, not a milestone's — is a machine for
  making more of those.

Auto-generated notes were a second strike: for a submodule tag they summarise everything merged since
the previous `contrib/<name>/…` tag, which for a driver bump is mostly the core's unrelated work. The
hand-written annotated tag message says more in one line.

What replaces the Release as a record is the run itself: `$GITHUB_STEP_SUMMARY` carries the tag,
module path, version, directory and commit, plus the `go list -m <path>@<version>` command. That last
one is **left manual on purpose** — the module index and the proxy see a new tag on their own
schedule, so asserting it in the job would fail on lag rather than on correctness, which is exactly
how a release gate teaches people to re-run red jobs.

## Also changed

- `docs/workflow/release.md`'s contrib section goes from a hand procedure to a description of the
  gate, its five refusals, and the delete-and-repush remedy for a red run. The boundary table gains a
  **CI** row and no human one: the agent still tags and pushes, and there is still no Publish step to
  reach.
- `CHANGELOG.md` `[Unreleased]`, the ADR index, the ROADMAP checkbox and annotation.

## Accepted costs, stated rather than discovered

- **The gate is after the fact.** A bad tag is caught, not prevented. Bounded, because nothing is
  published; the one irreversible case is a *correct-looking* tag the index has already served, which
  is what the five parse checks make unreachable.
- **The workflow has never run.** Both existing submodule tags predate it and workflows are not
  retroactive, so the first real exercise is the next `contrib/*` release. Validation was therefore
  the parse table above plus a YAML parse confirming the trigger, the single job and its seven steps.
- **A pre-release tag is accepted** (`contrib/x/v0.3.0-rc.1` parses and verifies). No submodule has
  used one; refusing it would be a policy this project has not decided, and a regex is the wrong
  place to invent one.
- **`examples/*` is not covered, by construction** — those modules are never tagged (ADR-0054), so
  the tag space partitions into exactly two.

## One adjacent defect, noticed and deliberately left

`release.yml`'s `setup-go` step and `ci.yml`'s `benchmark` job both read
`go-version: ${{ matrix.toolchain == 'go-1.25' && '1.25' || '1.26' }}` **in jobs that have no
matrix**. Harmless — both evaluate to `'1.26'` — but they claim a matrix that does not exist. Not
fixed here: 14.7 edits every step of both files, and a one-line cleanup landing inside a `contrib`
release PR is a change a reviewer has no reason to expect.

## Where the next session picks up

Milestone 14 at **6/12**. Next is **14.7** — pin every action to a SHA, least-privilege
`permissions:` per job, and the signing decision (`required_signatures` on `master` versus
`actions/attest-build-provenance` + a CI-generated SBOM). 14.6 leaves it two gifts: a workflow
already written at `contents: read`, and the two no-matrix `go-version` expressions above, which are
in its sweep.

Still open for the maintainer from 14.5: **`examples / service` is not a required status check yet**,
and `required_linear_history` + `required_conversation_resolution` remain off.
