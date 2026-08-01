# 2026-08-01 — post-v2.0.0 audit: contrib released, and the drift a release does not show

**`v2.0.0` is published** and Milestone 13 landed complete: PR #80 merged 13.9, so `contrib/*` is on
the core's `/v2` in the repository. This session audited what the release left behind, and the answer
split cleanly in two: the *release* was sound, the *repository around it* had drifted.

## The release itself holds up

Verified rather than assumed: the tag is annotated and points at the release commit (`b732098`), the
Release is published and latest, CI is green on master, and all four policy tools pass — consistency,
import-graph (3 runtime deps in their owning packages, 1 sanctioned internal edge), coverage (23
packages, weakest `fanout` at 93.3%), spec-api (141 identifiers, no stale entries). On the proxy,
`…/egl-utils-go/v2@latest` resolves to v2.0.0 while `…/egl-utils-go@latest` still resolves to v1.1.1,
which is the whole point of a major suffix: **both majors coexist and a consumer migrates when it
chooses.** pkg.go.dev renders the `/v2` landing page with a valid `go.mod` and MIT detected.

## contrib is released — `v0.2.0`, and the version was the argument

`contrib/redishealth/v0.2.0` and `contrib/pgxhealth/v0.2.0` are tagged and pushed. Until this point
13.9 was **real in the repository and not real for consumers** — `go get` still resolved v0.1.0, which
requires the core's v1 — and that gap is the thing a "milestone complete" line can hide.

The choice was `v0.2.0` against `v1.0.0`, and it turns on what actually broke. **No identifier
changed**: `Check`, `Option`, `WithTimeout` are spelled exactly as in v0.1.0. What changed is that
`Check` now returns the `health.Check` of a *different module path*, so wiring it into a v1
`health.Handler` no longer compiles. That is breaking for consumers with an unchanged surface — a
minor bump inside `v0` is the honest encoding. `v1.0.0` was declined because it is a stability
commitment on an API pinned to a **driver** major: `go-redis/v9`, `pgx/v5`. Their next major would
force a `contrib/<name>/v2` path suffix on a module whose own code need not change, and `v0` costs
nothing today.

Recorded in [release.md](../../workflow/release.md) rather than left as folklore, because the
procedure has a trap: **`release.yml` triggers on `tags: ["v*.*.*"]`, which a `contrib/…` ref does not
match.** No draft Release, and — the part that matters — **no CI run on the tagged tree**. So the
verification has to happen before the tag, by hand: both modules built, vetted and tested from their
own directories, plus the `contrib` CI job green on the exact commit tagged. A submodule release is
the one release in this repo with no machine watching it.

## What the audit actually found: governance drift, not code

`docs/workflow/github-setup.md` describes the repo-level configuration that cannot live as a
committed file. Comparing it against the live repository, three of its five sections had never been
applied — and this is the interesting failure mode, because **nothing about it is visible in a diff,
a test run, or a policy tool.** A documented setup step that was never executed reads exactly like one
that was.

The concrete costs, each verified before being fixed:

- **The label set never existed.** `.github/labels.yml` declares ten labels; the repository had only
  GitHub's nine defaults. `dependabot.yml` asks for `build` and `contrib`, so **every Dependabot PR
  got an error comment** — visible on #81 and #82: *"The following labels could not be found."* And
  `contrib` was not even in the manifest, so syncing it would not have been enough. Both fixed: the
  eleven labels now exist, and `contrib` is in the file with a comment saying why a non-type label is
  in a list of Conventional-Commit types.
- **Squash-only was policy in prose only.** Merge commits and rebase merges were both still enabled,
  and `delete_branch_on_merge` was off — which is why **63 merged branches had accumulated on the
  remote**, one per PR since M1. Deleted, after confirming every one of them had a merged PR (the
  check is against the PR list, not ancestry: squash merges leave no ancestor, so `--is-ancestor`
  reports all 63 as unmerged and would have been the wrong instrument).
- **`required_linear_history` and `required_conversation_resolution` were off**, though the rest of the
  branch protection was in place and in one respect stricter than documented (`enforce_admins: true`).
  Left for the maintainer: the API call that sets them replaces the whole protection object, and it was
  refused by this session's tooling — recorded rather than worked around.

Also closed: the **M13 milestone was still open** on a completed, released milestone. `M10` was never
created at all, which is why its thirteen PRs carry no milestone — left as it is, since inventing a
milestone after the fact would put a tidier history in the board than the one that happened.

## `orchestrator/project.yaml`: the sweep it kept asking for

Yesterday's checkpoint called this file "open, unowned — four kinds of stale in one file". It is the
interview manifest: no tool reads it at runtime, its whole audience is a **regeneration**, and its
convention (ADR-0003, ADR-0041) is to amend in place with dated notes rather than rewrite the record.
So the test for what to fix is narrow: *would a regeneration produce something wrong?*

By that test: the namespace and include hint lacked `/v2` and `pkg/`; `coverage_target: 80` described a
module-wide gate that ADR-0036 rejected as unfailable (the real gate is 85 **per package**);
`prometheus/client_golang` was still listed as a vetted runtime dependency nine months of ADRs after
ADR-0050 removed it and its eight transitive modules; the compatibility clause still promised a
"MAJOR-intent note in the PR", which is the pre-1.0 posture — past 1.0 breakage is not noted, it is
scheduled through a ledger and discharged in a major.

The `public_api` block was the largest piece, and rewriting all of it would have been the wrong
instinct: **it is not the source of truth** — `docs/specs/v2` §5 is, gate-enforced against the compiled
surface by `spec_api_lint.py`. So the block gained a note saying exactly that, plus in-place
corrections for the seven entries the major reshaped, each naming its ADR. The additive v1.1.0
capabilities are deliberately *not* duplicated into it: a second list free to drift is the problem the
note exists to prevent. Two package names in the architecture listing were also v1 spellings a
regeneration must not re-emit — `errors` (now `errx`) and a `GracefulShutdown` function that never
existed under that name.

## State

The core is at **v2.0.0, published**; `contrib/*` at **v0.2.0**, on the core's `/v2` and now real for
consumers. No open issues, no open PRs, no open bugs, four policy tools green, CI green.

Open, in the maintainer's court: the two branch-protection flags above. Open and known-low:
`pgxhealth`'s transitive `x/text@v0.29.0` carries the uncalled **GO-2026-5970**, whose fix is
floor-safe here and is left to Dependabot. `v0.1.0` has a git tag but still no GitHub Release, which is
now the only asymmetry left in the release history.

Not started, and the first real question after a major: **there is no Milestone 14.** The ledger that
drove M13 is empty by design, so the next milestone has to be *chosen* rather than discharged.
