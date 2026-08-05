# 2026-08-05 — 14.7: the half-pinned repository, one write token, and a gate that could not see its own subject

Milestone 14 reaches **7/12**. 14.7 was three sweeps and two decisions, and the interesting part of
each was a measurement that disagreed with the roadmap line describing it.

## What shipped

- **Every `uses:` in `.github/workflows/` pinned to a 40-character commit digest** with the release
  in a `# vX.Y.Z` comment — 21 of 36 references were floating; 37 are now pinned (the extra is the
  new attestation action).
- **`permissions: {}` at every workflow's top level, and a block on every one of the thirteen jobs.**
  Twelve at `contents: read`; exactly one — `release.yml`'s `draft-release` — at `contents` +
  `id-token` + `attestations`, all write.
- **A reproducible CycloneDX SBOM as the project's first release artifact**, attested with
  `actions/attest`, generated and checked on every pull request as well as on the
  tag.
- **Two new checks in `tools/consistency_lint.py`** (`action-pins`, `workflow-permissions`), taking
  it from eight to ten, gating all of the above on an already-required status context.
- **[ADR-0056](../../../adr/0056-build-time-supply-chain.md)**, control **C-6**, a third trust
  boundary in the threat model, and the project's first written statement about signing.
- The two no-matrix `go-version` expressions 14.6 deferred, and the generator that produces them.

## The half-pinning was copy-paste, not a policy

The roadmap said `setup-go`, `golangci-lint-action` and `upload-artifact` float while `checkout` and
`setup-python` are pinned. Counted: **`actions/checkout` floats too**, at `ci.yml:80` and
`ci.yml:289`, in the same file that pins it at eleven other sites. `actions/cache/restore` and
`actions/cache/save` float and went unmentioned entirely.

That changes the diagnosis. A repository where three actions float looks like three decisions; a
repository where the *same action* is pinned eleven times and floating twice is a step copied into a
job that came later. Which is the same failure mode as the `go-version` expression this item also
fixes, and as the `${{ matrix.toolchain … }}` copy in `release.yml`: **this codebase's characteristic
CI defect is a step pasted into a job whose requirements differ from the one it came from.** Worth
naming, because it is an argument for gates over sweeps — a sweep fixes today's copies.

The standard objection to digest pinning is that it freezes actions and buries security fixes.
`.github/dependabot.yml` has watched the `github-actions` ecosystem weekly since generation, so the
objection was already answered and simply never stated. That is the one dependency this decision
creates and cannot itself detect: remove that entry and the pins go stale silently.

## Least-privilege was three-quarters done and undocumented

`ci.yml`, `contrib-release.yml` and `nfr-nightly.yml` already declared `contents: read` at the
workflow level. The item reads as though token scope were wide open; it was not. The single real
exposure was `release.yml`, whose `contents: write` sat at the **workflow** level — inherited by any
job added to that file afterwards.

So the rule shipped is stricter than "narrow it": **no workflow grants anything, every job asks.**
That is what makes the defect unexpressible rather than merely fixed, and given the copy-paste
pattern above, "unexpressible" is worth thirteen two-line blocks. `ci.yml`'s `workflow_call:` trigger
is a second reason — a caller's grant caps what the called workflow holds, and per-job blocks mean
that cap is not the only thing between a caller and eleven jobs with write access.

Verified rather than assumed, because the temptation was to grant defensively:
`golangci-lint-action` needs no `pull-requests:` scope without `only-new-issues`;
`upload-artifact` v4+ and the `cache` endpoints authenticate to the Actions runtime service, not with
`GITHUB_TOKEN`; no workflow here invokes `gh`, comments, or pushes.

## Every SBOM flag was a measurement

`cyclonedx-gomod@v1.10.0`, and each flag decided against output rather than documentation:

- **`-type library`.** The default is `application`. The root component's type is the first field a
  scanner reads and it would have been false.
- **No `-test`: three components rather than eight.** Without test dependencies the inventory is
  exactly `golang.org/x/crypto`, `golang.org/x/sync`, `gopkg.in/yaml.v3` — **ADR-0004's runtime
  dependency set, and `import_graph_lint.py` independently reports "3 runtime deps" on the same
  tree.** A consumer of a library links those three and never links testify. The SBOM turns out to be
  the dependency policy in machine-readable form, which is a better artifact than a complete graph
  that overstates by 167% what a consumer resolves.
- **`-noserial -notimestamp` → byte-identical output across runs** (4 063 bytes, verified by running
  it twice). `ci.yml` now asserts this with `cmp` on every pull request, so "reproducible" is a gate
  rather than a sentence.
- **No `-licenses`, and this one reversed on the measurement.** Detection is cheap (~3 s) and **wrong
  on all three components.** `x/crypto` and `x/sync` are reported `BSD-Source-Code`; their LICENSE
  files carry all three Go clauses including "Neither the name of Google LLC", so they are
  `BSD-3-Clause`. `yaml.v3` is reported `Apache-2.0`; its LICENSE opens *"This project is covered by
  two different licenses: MIT and Apache."* CycloneDX reports detected licences as *evidence*
  precisely because they are not guaranteed — but a compliance tool reads the field, not the hedge,
  and with three dependencies the exact part (name, version, purl, hash) is the whole value.

## Where the provenance claim stops, and why that is the decision

The subject of the attestation is **the SBOM**, not the module. Stated plainly in the ADR because the
easiest thing to do here was overclaim.

What a consumer resolves is anchored in the Go checksum database, which this project neither operates
nor can edit: `github.com/danielPoloWork/egl-utils-go/v2 v2.0.0` is recorded as
`h1:fg0qkFnLtGTVZK7QA63aWQi/+KMoEEscgNjMURXTy9k=`, the same value in the `go.sum` of all three
in-repo consumers, and `proxy.golang.org` separately records the version's origin
(`refs/tags/v2.0.0` at `b732098`). A re-pointed tag is detectable by anyone who has ever resolved the
version. An attestation over the module would be a weaker claim standing next to a stronger one.

What genuinely had no integrity story is the release **asset set** — because it was empty. All four
releases have zero attached assets, so `release.md` step 10 ("CI builds & attaches artifacts") has
been false since the first release. An unattested file on a Release page is one anybody with write
access could swap; that is the gap the attestation closes, and it is real only because the artifact
is new.

The module zip was considered as the subject and is unavailable: getting the canonical zip means
asking the proxy, and ADR-0055 established that a release job must not assert what the proxy has
seen — the index picks up a tag on its own schedule, so such a job fails on lag rather than on
correctness.

**A caution earned the hard way:** the first checksum-database lookup I ran used the wrong case for
the owner segment. It returned a valid-looking record with a *different* `h1`, under
`github.com/DanielPoloWork/…`, which does not match `go.mod`. A `lookup` request is what makes the
proxy fetch, so **it is a write, not a read** — the mis-cased query almost certainly added that
permanent entry to an append-only log, and its later tree position (58884989 against the canonical
58416554) is consistent with that. Harmless, and a good reminder that the module path is
case-sensitive while the GitHub URL is not. The authoritative check is a consumer's `go.sum`, not a
hand-typed URL.

## The gate that could not see its own subject

`workflow-permissions` scanned for scopes with a pattern anchored to end-of-line. The only job in the
repository that holds write access documents each of its three scopes with an end-of-line comment. So
the check matched none of them, computed an empty write set, compared that against the allowlist, and
**passed** — green, on the single job it exists to police, with its allowlist entry inert.

It was found by asking what the check *saw* rather than whether it passed. That is ADR-0043's lesson
from 12.1 arriving in a new place: a checker with a blind spot is worse than none, because it
certifies the part it cannot see. The fix is one optional comment group; the finding is that "the
suite is green" was not evidence of anything until the positive case had been inspected too. It
became the tenth deliberate-violation case, alongside a floating tag, a digest with no comment, a
comment that is not a version, a 39-character digest, a `uses:` with no version, a job with no
permissions block, `contents: write` on a CI job, `attestations: write` on the nightly, and a
workflow-level grant restored.

## The signing premise was false, and splitting it was the answer

The roadmap assumed signing is blocked because "the agent cannot produce signed commits". Tested:
`68fd847`, `b8c1165` and `51b9310` each report `verified=true`, `reason=valid`, committer **GitHub**.
The repository is squash-only, so GitHub creates and signs the squash commit itself; an agent's
unsigned feature-branch commits never land on `master` as themselves.

So the decision separates. **`required_signatures` on `master` is free** — already satisfied by every
commit on the branch, and worth turning on to close an administrator's direct push or a future change
of merge strategy. **Signed *tags* are declined**: tag creation is the agent's carry-through, so
requiring them moves tagging to the maintainer and rewrites three boundary tables, and the benefit is
thin because a tag is not what a consumer verifies. Recorded positively in the compliance register
rather than by omission, and `github-setup.md` now carries the project's first written word about
signing at all.

## Also changed

- **`orchestrator/project.yaml` amended in place** (dated notes, per ADR-0003/0041). Its embedded
  workflow templates carried `setup-go@v5`, `checkout@v6`, `golangci-lint-action@v6`, no permissions
  blocks, and the source of both no-matrix `go-version` expressions. The convention's test — *would a
  regeneration produce something wrong?* — answered yes for the first time against a **live policy**
  rather than a stale fact: a regeneration would have emitted a tree failing the gate this item adds.
- `release.md` step 10 describes what now happens, with the consumer-side `gh attestation verify`
  next to it; the `## Boundary` table's artifact row names the SBOM and its attestation.
- `AGENTS.md` §10 gains a **Supply chain** row. §6.1's boundary table is deliberately untouched —
  that is what declining signed tags means in practice.
- `consistency_lint.py`'s docstring documented the `posture` check, which had been in `CHECKS` and
  absent from the enumeration since it was written.

## Accepted costs, stated rather than discovered

- **The attestation and the asset upload cannot run until a tag exists**, so this pull request does
  not exercise them; 14.12's `v2.0.1` is their first execution. The per-PR generation step covers the
  generator and its two invariants, not the attest-and-upload path — mitigated, not solved.
- **`contrib/*` releases get no SBOM.** Their graphs are the interesting ones, but ADR-0055 gives
  those tags no Release, so the only home is a 90-day workflow artifact, and a record with an expiry
  date is not a record. A candidate for 14.10's ledger.
- **`zizmor` was declined**, so template injection and artifact poisoning stay unmitigated as a
  *class*. The current pipeline was checked by hand and is clean — `contrib-release.yml` reads
  `$GITHUB_REF_NAME`, never a `${{ }}` interpolation into a `run:` block — but nothing gates a future
  workflow that does otherwise.
- **Three settings are the maintainer's**, and no diff can prove them: `sha_pinning_required`,
  `required_signatures`, `examples / service` as a required context, plus the two long-open
  branch-protection flags.

## One local-environment finding

`cyclonedx-gomod` **cannot be compiled on this workstation.** The portable Go at
`…/Temp/go-portable/go` is a pruned distribution: `compress/bzip2`, `image/jpeg`, `archive/tar`,
`math/cmplx` and `index/suffixarray` exist as directories containing **zero `.go` files**, so
`go list std` reports 288 packages and those five are not among them. Nothing in this module or its
dependencies had ever needed them. The SBOM work used the official prebuilt binary instead, verified
against the published `checksums.txt` before running it — which is the appropriate amount of paranoia
for a supply-chain pull request.

## Where the next session picks up

Milestone 14 at **7/12**. Next is **14.8** — NFR-02's `Submit` p99 and NFR-06's mixed-load p99
measured on Linux CI, where the clock advances (on Windows the suite publishes
`tail-unmeasurable=1`), updating the 2026-07-26 report in place and keeping ADR-0037's split intact:
a tail is measured and reported, never gated.

Open for the maintainer, now four items: `sha_pinning_required: true`, `required_signatures: true` on
`master`, **`examples / service` as a required status check** (still, from 14.5 — 13 required against
14 rendered), and `required_linear_history` + `required_conversation_resolution`, off since the
project began while `github-setup.md` §3 documents both as true.
