# ADR-0056: the build-time supply chain — every action pinned to a digest, one job with a write token, an SBOM whose provenance claim stops where the checksum database begins, and no signed tags

- **Status:** Accepted
- **Date:** 2026-08-05
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** roadmap 14.7 (this item); [ADR-0055](0055-contrib-release-workflow.md) (written at
  `contents: read` in anticipation of this ADR, and the source of the "a generator nobody has run"
  argument); [ADR-0054](0054-examples-service-module.md) (the required-status-check trap that keeps
  this ADR's gate out of a new CI job); [ADR-0004](0004-runtime-dependency-policy.md) (the runtime
  dependency policy the SBOM turns out to state exactly);
  [ADR-0036](0036-coverage-gate.md)/[ADR-0035](0035-import-graph-enforcement.md)/[ADR-0043](0043-spec-api-lint.md)
  (the policy-tool line and its verify-by-deliberate-violation precedent);
  [ADR-0015](0015-enterprise-governance-posture.md) (the posture that makes this a control rather
  than a preference); `docs/workflow/github-setup.md` (where the declined signing lands);
  roadmap 14.12 (the release that first exercises the attestation)

## Context

Milestone 14 called this repository **half-secured, which reads as a policy and is not one**. The
measurement is worse than the roadmap line, and the shape of the error is the point.

**Pinning.** Of 36 `uses:` references across four workflows, 15 named a commit digest and **21
floated on a mutable tag**. The roadmap blamed three actions; the truth is less deliberate than
that. `actions/checkout` was digest-pinned in 11 places and floating in 2 — `ci.yml:80` and
`ci.yml:289` — *in the same file as its own pins*, which is not a decision anybody made about
`checkout` but a step copied from a job that predated the pinning. `actions/cache/restore` and
`actions/cache/save` floated too and went unmentioned. A half-pinned tree is worse than an unpinned
one precisely because the pins that exist imply the omissions were considered.

**Token scope.** Three of the four workflows already declared `permissions: contents: read` at the
workflow level, so least-privilege was mostly *done* and nowhere written down. The real exposure was
`release.yml`: its `contents: write` sat at the **workflow** level, where it is inherited by every
job added to the file afterwards. No job in the repository declared a scope of its own, so the one
write token in the project was addressed to the file rather than to the step that needed it.

**Release artifacts.** `docs/workflow/release.md` step 10 has promised "CI builds & attaches
artifacts on the tag push" since the first release. `v1.0.0`, `v1.1.0`, `v1.1.1` and `v2.0.0` each
have **zero attached assets**: `release.yml` calls `action-gh-release` with no `files:` at all. The
step described something that had never once happened.

**Signing.** The words *sign*, *signature* and `required_signatures` appear nowhere in this
repository's documentation. There is no policy to amend; this ADR states one for the first time.

Two facts outside the tree complete the picture: the repository-level Actions setting
`sha_pinning_required` is `false`, so nothing server-side refuses an unpinned reference; and
`default_workflow_permissions` is already `read`, which is a floor a repository administrator can
lower at any time without touching a file under review.

## Decision

**Pin every action to a commit digest with the version in a comment, move every token scope onto the
job that uses it, attach a reproducible CycloneDX SBOM to the release with a provenance attestation
whose claim is scoped to that document, gate all of it in `tools/consistency_lint.py`, and decline
signed tags with the reason recorded.**

### (a) Digests, and the comment that is not decoration

Every `uses:` now reads `owner/action@<40-hex> # vX.Y.Z`. Both halves are required by the gate. The
digest is what pins; the comment is what makes the pin **reviewable**, because forty hex characters
carry no information a reviewer can act on, and it is what Dependabot rewrites when it bumps one.

The standard objection to digest pinning is that it freezes actions and buries security fixes. That
objection was already answered before this ADR and simply not stated: `.github/dependabot.yml` has
watched the `github-actions` ecosystem weekly since the repository was generated. Pinning without an
updater would be a real regression; pinning *with* one converts an invisible mutable dependency into
a reviewed pull request. This is worth writing down because it is the reason the decision is cheap
here and would not be cheap in a repository without that entry.

`actions/setup-go@v7` resolved to the same commit as `v7.0.0` at the time of the sweep, so the
pinning of the twelve `setup-go` steps is behaviour-preserving on the day it lands — the change is
that tomorrow's `v7` cannot become something else without a diff.

### (b) The workflow grants nothing; every job asks

Each workflow declares `permissions: {}` and each job declares exactly the scopes it uses. Thirteen
jobs across four files: twelve at `contents: read`, and `release.yml`'s `draft-release` alone at
`contents: write` + `id-token: write` + `attestations: write`.

The rule is stricter than "narrow the workflow-level block" and the extra strictness is the whole
value: with no grant at the workflow level, **a job added later cannot inherit a scope**, so the
defect `release.yml` actually had becomes unexpressible rather than merely fixed. `ci.yml` also
carries a `workflow_call:` trigger, where a caller's grant caps what the called workflow can hold;
per-job blocks mean the cap is not the only thing standing between a caller and eleven jobs holding
write access.

Nothing else needed a scope, which was verified rather than assumed:
`golangci/golangci-lint-action` is used without `only-new-issues`, so it wants no
`pull-requests:` scope; `actions/upload-artifact` v4+ and the `actions/cache` endpoints authenticate
to the Actions runtime service rather than with `GITHUB_TOKEN`; and no workflow in this repository
invokes `gh`, comments on a pull request, or pushes anything.

### (c) The SBOM, and where its provenance claim stops

`release.yml` generates a CycloneDX document with `cyclonedx-gomod@v1.10.0` — installed by
`go install …@v1.10.0`, the same version-pinned pattern `gofumpt@v0.10.0` and `govulncheck@v1.4.0`
already use in these jobs, and a form that resolves outside the current module, so `go.mod` and
`go.sum` are untouched. Every flag was chosen against a measurement:

- **`-type library`.** The tool's default is `application`. This module is not one, and a document
  whose root component claims otherwise is wrong in the first field a scanner reads.
- **No `-test`.** With test dependencies the inventory is eight components; without them it is
  **exactly three — `golang.org/x/crypto`, `golang.org/x/sync`, `gopkg.in/yaml.v3`.** That is
  ADR-0004's runtime dependency set, and the coincidence is the argument: a consumer of a library
  links those three and never links `testify`, `goleak` or `rapid`. Including them would inflate the
  count consumers act on by 167% to describe this repository's own test suite. The SBOM is the
  dependency policy made machine-readable, which is a better artifact than a complete-but-misleading
  graph.
- **`-noserial -notimestamp`.** Verified, not assumed: two runs over the same tree produce
  **byte-identical output** (4 063 bytes). This makes the document re-derivable from the tag it
  names, which is what lets a consumer check the attestation against something rather than accept
  it. `ci.yml` asserts the property with `cmp` on every pull request, so "reproducible" is a gate
  and not a sentence in this ADR.
- **No `-licenses`.** This one was measured and reversed. Detection is cheap (about 3 s) and
  **wrong on all three components**: `x/crypto` and `x/sync` are reported `BSD-Source-Code` when
  both carry the standard three-clause Go licence (checked in the module cache — "Redistributions of
  source code", "Redistributions in binary form" and "Neither the name of Google LLC", so
  `BSD-3-Clause`), and `yaml.v3` is reported `Apache-2.0` when its own LICENSE opens "This project
  is covered by two different licenses: MIT and Apache." The tool reports detected licences as
  *evidence* precisely because it cannot guarantee them, and `-assert-licenses` exists to promote
  them to claims. With three dependencies, the inventory a scanner needs is name, version, purl and
  hash — all exact — and licence data that is three-for-three wrong is worse absent than present
  under a hedge, because a compliance tool reads the field and not the hedge.
- **No `-std`.** The standard library is not a dependency a consumer resolves; it would add
  hundreds of components describing the toolchain rather than the module.

The provenance step is `actions/attest` — **not** `actions/attest-build-provenance`, which as of its
v4 is a composite wrapper that forwards its inputs verbatim to `actions/attest` and whose own release
notes say new implementations should use that action directly. Reading the wrapper's `action.yml`
settled a second question at the same time: `create-storage-record` defaults to `true` while also
requiring `push-to-registry`, which is `false` here, and a storage record is what would demand an
`artifact-metadata: write` scope this job does not hold. It is therefore set to `false` explicitly.
Since a tag push is the earliest this step can ever execute, an ambiguity like that is worth settling
by reading rather than discovering during a release.

**The subject is the SBOM.** This is the decision in this section, so it is stated plainly:

> The attestation says that *this document* was produced by *this workflow* at *this commit*. It
> does **not** claim provenance for the module a consumer resolves, and this ADR refuses to imply
> that it does.

The code a consumer actually gets is anchored in the Go checksum database, which is a transparency
log this project neither operates nor can retroactively edit. `github.com/danielPoloWork/egl-utils-go/v2
v2.0.0` is recorded there as `h1:fg0qkFnLtGTVZK7QA63aWQi/+KMoEEscgNjMURXTy9k=`, the same value in
the `go.sum` of all three in-repo consumers, and `proxy.golang.org` additionally records the origin
of the version — `refs/tags/v2.0.0` at commit `b732098` — so re-pointing a published tag is
detectable by anyone who has ever resolved it. An attestation over the module would add a weaker
claim next to a stronger one.

What *had* no integrity story is the release asset set, because until this ADR it was empty. An
unattested file attached to a Release is a file anyone holding write access could replace; that is
the gap the attestation closes, and it is a real one precisely because the artifact is new.

The module zip was considered as a subject and is not available: obtaining the canonical zip means
asking the proxy, and ADR-0055 established that a release job must not assert what the proxy has
seen — the index and the proxy pick up a tag on their own schedule, so a job that waits on them
fails on lag rather than on correctness.

Generation also runs on **every pull request**, in `ci.yml`'s existing `imports` job, where the two
properties the release depends on are asserted: that the document describes this module (compared
against `go list -m`, so the check cannot rot the way a written-down path would — this module path
has already changed once, at `/v2`) and that it is byte-reproducible. ADR-0055 had to admit that
`contrib-release.yml` "has never run and cannot have"; a generator whose first execution is a
release is the same defect chosen deliberately, and running the identical command per pull request
is what avoids repeating it. The job's `name:` is untouched, so no required status context moves.

### (d) The gate lives in `consistency_lint.py`, not in a fifth tool

Two new checks — `action-pins` and `workflow-permissions` — bring that tool to ten. They are there
rather than in `tools/workflow_lint.py` for a reason outside the code: **`consistency / lint` is
already one of master's thirteen required status checks** *(fourteen since 2026-08-08 —
`examples / service`; the argument is unchanged)*, and ADR-0054 recorded that adding a job
does not add a required context. A new tool would have shipped a gate that could go red without
blocking anything until someone edited branch protection by hand, and would have rewritten "all four
policy tools" in `AGENTS.md` §10, the pull-request template and nine journal files to describe five.

> *Amended 2026-08-08: **eleven.** [ADR-0057](0057-additive-capability-ledger.md) added
> `ledger-coverage` on exactly the reasoning in this paragraph — `consistency / lint` is already
> required, a new job would not be. Nothing here is superseded; the count moved.*

`workflow-permissions` is the half with teeth. It refuses a workflow-level grant, requires every job
to declare a block, and allows a `write` scope only where an explicit allowlist names the file and
job. A future pull request adding `contents: write` to a test job fails the build.

Both checks are standard-library only, like the other four tools, because the `consistency` CI job
installs no packages. Both were verified **by deliberate violation** — ten cases: a floating tag, a
digest with no comment, a comment that is not a version, a 39-character digest, a `uses:` with no
version at all, a job with no permissions block, `contents: write` on a CI job, `attestations: write`
on the nightly job, a workflow-level grant restored, and the case below. Each was applied, detected
with the right file and line, and reverted.

**The tenth case was a hole in the check itself, and it is worth recording because of how it
presented.** `workflow-permissions` scanned for scopes with a pattern anchored to end-of-line, while
the only job in the repository that holds write access documents each of its three scopes with an
end-of-line comment. So the check matched none of them, computed an empty write set, compared it
against the allowlist, and **passed** — green, on the single job it exists to police, with the
allowlist entry doing nothing. It was found by asking what the check *saw* rather than whether it
passed, which is the same lesson ADR-0043 recorded about the throwaway surface checker in 12.1: a
gate with a blind spot is worse than no gate, because it certifies the part it cannot see. The fix is
one optional comment group in the pattern; the finding is that "the suite is green" was not evidence
of anything here until the positive case had been inspected too.

### (e) Signed tags are declined; `required_signatures` on `master` is recommended

The roadmap framed this as one decision resting on the claim that "the agent cannot produce signed
commits". Tested, that claim is **false for commits and true for tags**, and the two halves separate
cleanly.

Every commit on `master` is already signed. `68fd847`, `b8c1165` and `51b9310` each report
`verified=true`, `reason=valid`, committer `GitHub` — because the repository is squash-only and
GitHub creates and signs the squash commit itself with its web-flow key. An agent's unsigned
feature-branch commits never land on `master` as themselves. So `required_signatures: true` costs
nothing, breaks nothing, and closes the paths that remain (an administrator's direct push, or a
future change of merge strategy). It is recommended to the maintainer as a hand-off, with the
evidence, rather than adopted here, because it is a repository setting and no diff can carry it.

Signed **tags** are the real block and are declined. `release.md` step 8 gives tag creation to the
agent as carry-through; requiring signatures on tags moves that to the maintainer and rewrites three
tables in lockstep (`AGENTS.md` §6.1, `release.md` step 8 and its `## Boundary` table). The cost is
concrete and the benefit is thin, for the reason set out in (c): a tag is not what a consumer
verifies. Giving an automated process an unattended signing key would also make the signature attest
"a machine with a key did this", which is not what a signature claims.

So the compliance register carries the claim positively rather than by omission: this project does
not sign tags, its commits are signed by GitHub, and the artifact integrity story is the checksum
database plus a provenance attestation over what CI produces.

## Alternatives Considered

- **A fifth policy tool, `tools/workflow_lint.py`, with its own CI job. This is the strongest
  alternative** — the other four tools each own one concern, and pinning is not "congruence" in the
  sense the others mean it, so the new checks sit slightly awkwardly in a tool whose docstring is
  about internal agreement between documents. Rejected on a mechanical consequence rather than
  taste: a new job is not a required status check until someone adds the context by hand (ADR-0054),
  so the version of this decision that looks cleaner ships a gate that can be red on a mergeable
  pull request. The awkwardness is paid for in one docstring paragraph; the alternative is paid for
  in branch-protection state that lives outside the repository.
- **Adopt `zizmor` (or `actionlint`) instead of writing checks.** Genuinely catches classes these
  two do not: template injection, artifact poisoning, cache-scope confusion, and heuristics for
  over-broad permissions. Rejected for now because a supply-chain gate that adds a third-party
  binary to every CI run is spending the thing it is protecting, and because the two properties this
  repository actually got wrong — an unpinned reference and an inherited write token — are exactly
  the two a seventy-line standard-library check decides exactly. Recorded as the reason a future
  reader might revisit this: the coverage gap is real, and the injection class was checked by hand
  here and found clean (`contrib-release.yml` reads the tag as `$GITHUB_REF_NAME`, not as a `${{ }}`
  interpolation into a `run:` block, so the one workflow that parses a ref name is not injectable).
- **Sign tags, and move tagging to the maintainer.** The strongest posture available and the one a
  security reviewer would ask for first. Rejected because the artifact it would protect is not the
  artifact a consumer verifies (the checksum database already covers that, append-only), while the
  cost is a permanent human step in every release and three boundary tables restated. Revisit if the
  project ever ships a binary, where a signature has a subject that matters.
- **Decline signing wholesale**, as the roadmap line assumed, and change no setting. Rejected
  because the assumption behind it turned out to be wrong: commits on `master` are already signed, so
  "this project does not sign" would have been a false statement in the compliance register, and
  `required_signatures` would have been left off for a reason that does not exist.
- **Keep the token scopes at the workflow level and just narrow `release.yml`.** Much smaller diff —
  thirteen two-line blocks are churn in files nobody reads for pleasure. Rejected because it fixes
  the instance and leaves the shape: a workflow-level grant is inherited by jobs that do not exist
  yet, and this repository has already demonstrated that a step gets copied into a job whose
  requirements differ from the one it came from (twice, for `go-version`, and twice for `checkout`).
- **Include `-licenses` in the SBOM**, since licence inventory is one of the main reasons consumers
  want one, and CycloneDX reports detection as evidence rather than assertion. Rejected on the
  measurement in (c): all three detections are wrong. An artifact published under this project's
  name should not contain a field that is three-for-three incorrect, however carefully the schema
  hedges it. Reversible the moment the detector improves, and the flag is one word.
- **Include test dependencies (`-test`)**, on the argument that an SBOM should describe the whole
  module graph and that `go.sum`'s 24 lines are the honest total. Rejected: an SBOM's reader is
  asking what they are shipping, and a library's consumer resolves three modules. The complete graph
  is one `go mod graph` away for anyone who wants it.
- **Use `actions/attest-sbom` as well as, or instead of, build provenance.** It is the purpose-built
  action for this artifact type. Rejected because it associates an SBOM with a *subject* — a binary
  or image the SBOM describes — and a source-only library has no such subject available in CI. The
  honest shape here is provenance over the document itself.
- **Generate the SBOM only on the tag.** Smaller CI, no per-pull-request cost. Rejected as the
  defect ADR-0055 named being chosen on purpose: the first execution would be the release, and a
  release is the worst place to discover a broken generator.
- **Attach SBOMs for the `contrib/*` submodules too.** Their graphs are the interesting ones — a
  redis or pgx driver's full closure, against the core's three. Rejected because ADR-0055
  deliberately gives those tags no GitHub Release, so the only home is a workflow artifact that
  expires in ninety days, and a record with an expiry date is not a record. Left as a candidate for
  the ledger in 14.10 rather than solved badly here.

## Consequences

- **A future unpinned action fails CI, and so does a future write token.** The two properties are
  now gated rather than swept, which is the difference between this milestone's complaint and its
  fix. The gate rides an already-required status check, so it blocks from the first run.
- **Pinning's cost is transferred to Dependabot, which was already watching.** Expect a weekly pull
  request per action bump, each rewriting a digest and its comment together. If that entry is ever
  removed, the pins silently become stale — the one failure mode this ADR creates and cannot itself
  detect.
- **The release gains its first artifact in the project's history, and `release.md` step 10 becomes
  true.** A consumer can now run `gh attestation verify` against a downloaded SBOM; the command is
  documented next to the step that produces it.
- **The provenance claim is deliberately narrow, and the narrowness is the contribution.** A reader
  who wants to know whether the module they resolved is authentic is pointed at the checksum
  database, not at this repository's attestations. Overclaiming here would have been the easiest
  thing in the ADR to do.
- **The SBOM is byte-reproducible, and that is enforced.** It also means the document is only
  reproducible *given the same platform and toolchain*: the root component's purl carries
  `goos`/`goarch`, and the `tools` block records the generator binary's own hash, so a different
  runner or Go version produces different bytes. CI is fixed to `ubuntu-24.04` and Go 1.26, and the
  attestation covers the exact bytes uploaded rather than a promise about anyone else's rebuild.
- **The attestation and the upload cannot run until a tag exists**, so they are unexercised by this
  pull request; roadmap 14.12's `v2.0.1` is their first execution. Mitigated, not solved, by
  generating on every pull request — the generator and its two invariants are exercised, the
  attestation and the asset upload are not.
- **`contrib/*` releases get no SBOM**, which is the one place this ADR leaves a gap it can name.
- **Three settings are handed to the maintainer** and no gate can prove they were applied — the
  2026-08-01 lesson restated: `sha_pinning_required: true` (server-side refusal of an unpinned
  reference, satisfied the moment this lands), `required_signatures: true` (free, per (e)), and the
  two long-open branch-protection flags plus `examples / service` as a required context. They are
  recorded in `github-setup.md` as commands, not run, because the whole-object `PUT` is the class of
  call that was blocked before and tool-shopping around that block is not on the table.
- **`orchestrator/project.yaml` was amended in place**, because its embedded workflow templates
  carried `setup-go@v5`, `checkout@v6` and `golangci-lint-action@v6` with no permissions blocks: a
  regeneration would have emitted a tree that fails the gate this ADR adds. This is the first time
  that file's convention — dated notes, audience is a regeneration — had a live policy to protect
  rather than a stale fact to annotate.
- **No Go file, exported identifier, behaviour or dependency changed.** The surface stays at 141 and
  `go.mod`/`go.sum` are untouched; `cyclonedx-gomod` enters as a CI tool, not a module requirement.

## References

- Roadmap 14.7 (this item), 14.12 (the release that exercises the attestation), 14.10 (where the
  contrib SBOM gap belongs); ADR-0055 (`contents: read` and the never-run argument), ADR-0054 (the
  required-context trap), ADR-0004 (the dependency policy the SBOM restates), ADR-0035/0036/0043
  (the policy-tool line and verify-by-deliberate-violation), ADR-0015 (the posture).
- The files that are the decision: `.github/workflows/{ci,release,contrib-release,nfr-nightly}.yml`,
  `tools/consistency_lint.py` (checks 9 and 10), `orchestrator/project.yaml` (`ci:` templates),
  `docs/compliance/README.md` (control C-6), `docs/security/threat-model.md` (the third trust
  boundary), `docs/workflow/{release,github-setup}.md`.
- Prior art in the vendored factory, cited but not this project's ADR line:
  `.eados-core/docs/adr/0009-ci-supply-chain-pinning.md` and
  `.eados-core/docs/adr/0013-dependabot-action-pin-auto-remediation.md`.
- External rules relied on: a GitHub Actions `permissions:` block at job level replaces the
  workflow-level one, and `{}` grants nothing; a called workflow cannot hold more than its caller
  granted; `actions/attest` needs `id-token: write` + `attestations: write` (and `artifact-metadata:
  write` only when it creates a storage record, which requires `push-to-registry`) and is
  free for public repositories; `go install <pkg>@<version>` resolves outside the current module
  since Go 1.16; `cyclonedx-gomod` flag semantics as of v1.10.0; and the Go checksum database at
  `sum.golang.org`, whose `lookup` endpoint is a write — a mis-cased query during this work added a
  permanent record for a module path that does not match `go.mod`, which is harmless and instructive.
