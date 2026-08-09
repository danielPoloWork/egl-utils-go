# Issues — egl-utils-go

The open defect and improvement backlog as a checkbox-driven list, newest first. Every entry
mirrors a GitHub issue and carries the tier recommended to resolve it.

**Ordering is chronological, not by priority.** The newest issue is at the top; when an issue is
opened, add its line **at the top of the list in the same PR** that opens it. Never renumber and
never reorder — the position of a line is its age. Severity is carried by the `BLOCKER` /
`MAJOR` / `MINOR` badge on each line, so triage reads the badge, not the position.

When an issue is closed, flip its checkbox (`- [ ]` → `- [x]`) **in the same PR** that closes it,
and leave the line in place.

- **Registers:** this file indexes the GitHub issue tracker. Verified defects also carry a record
  in the [bug ledger](docs/bugs/), which remains the source of truth for defects; planned work
  lives in [`ROADMAP.md`](ROADMAP.md). A line here is a pointer, never the record itself.
- **Provenance:** entries `#106`–`#148` were filed on 2026-08-09 by a seven-member release review
  board run against `origin/master@9ea5460` versus `v2.0.1`. Its verdict was **not approvable as a
  release**, on the single BLOCKER below.

### Agent guidance (model × effort)

Each entry carries a per-issue tag (`*agent: <model> · <effort>*`) naming the Claude model and
effort level recommended to resolve it, following the same convention as
[`ROADMAP.md`](ROADMAP.md#agent-guidance-model-effort). Entries whose tier deviates from what the
work superficially looks like carry a short rationale.

Model lineup (current as of 2026-08): **Claude Fable 5** (strongest reasoning) for
concurrency-critical and one-way API-design work; **Claude Opus 5** for subtle but well-trodden
correctness work, for anything touching build-critical tooling, and for text whose wording becomes
permanent once published; **Claude Sonnet 5** for well-specified integration and mechanical work.
Claude Haiku 4.5 is deliberately unused: every change ships under the full quality bar
(AGENTS.md §10) and Haiku lacks the `effort` control. Effort scale (Claude Code):
`low · medium · high · xhigh · max`.

> **Lineup drift, deliberate and flagged.** `ROADMAP.md`'s guidance section still names
> **Opus 4.8** and is dated *current as of 2026-07*. Opus 5 supersedes it. The two documents must
> not disagree about the lineup — update the `ROADMAP.md` line in the same PR that lands this file.

Two standing rules explain most of the tiers below, so they are not repeated on every line:

1. **Anything editing `tools/consistency_lint.py` is Opus 5 · high or above**, regardless of how
   small the diff looks. The repository has already shipped a check that could not fail — check 12's
   negative test passed when it should have failed, because a name pattern matched a leading
   lowercase run and stopped. A gate that agrees with you today is not a gate.
2. **Anything correcting a published artifact is Opus 5**, because the wording is permanent. Go
   module versions are immutable in `sum.golang.org`, and a GitHub Release body cannot be fixed by
   any later tag.

---

## Open

- [ ] [#148](https://github.com/danielPoloWork/egl-utils-go/issues/148) · `MINOR` · `docs` — ADR-0055's amendment was corrected **in place**, the one record in the delta where the project's own new rule was not applied; restore the original sentence and supersede it beside. — *agent: Opus 5 · medium — the subject of the ADR is the release workflow, so the record of how releases are corrected is the one corrected the old way*
- [ ] [#147](https://github.com/danielPoloWork/egl-utils-go/issues/147) · `MINOR` · `docs` — ~700 lines of shipped documentation are tracked by no roadmap row; either add a section for between-milestone work or state in `AGENTS.md` that journal entries carry it. — *agent: Opus 5 · high — a governance choice that decides what "tracked" means for every future between-milestone PR*
- [ ] [#146](https://github.com/danielPoloWork/egl-utils-go/issues/146) · `MINOR` · `docs` — five of six PRs carry no milestone while `AGENTS.md:180-182` mandates one and says to create it if absent; add the between-milestones case. — *agent: Opus 5 · high — as written the rule instructs the next agent to invent Milestone 15, which is the outcome the project decided against*
- [ ] [#145](https://github.com/danielPoloWork/egl-utils-go/issues/145) · `MINOR` · `docs` — `README.md:202-204` promises an SBOM on every release; one of five published Releases has one. Scope the sentence to the releases it is true of. — *agent: Sonnet 5 · low*
- [ ] [#144](https://github.com/danielPoloWork/egl-utils-go/issues/144) · `MINOR` · `docs` — `Changed` and `Fixed` carry entries that by their own admission affect no consumer; move governance-only changes to the journal. — *agent: Opus 5 · medium — an audience judgement about a file that declares Keep a Changelog conformance*
- [ ] [#143](https://github.com/danielPoloWork/egl-utils-go/issues/143) · `MINOR` · `ci` `docs` — the numerals "21 packages" and "55 runnable examples" are hand-typed in seven places and asserted by nothing; assert the counts or stop writing them. — *agent: Opus 5 · high — standing rule 1*
- [ ] [#142](https://github.com/danielPoloWork/egl-utils-go/issues/142) · `MINOR` · `docs` — the "order is load-bearing" middleware recipe shows five positions where the implementation it cites has six, with `metrics` outside the limiter. — *agent: Sonnet 5 · medium*
- [ ] [#141](https://github.com/danielPoloWork/egl-utils-go/issues/141) · `MINOR` · `ci` — check 12 hard-codes the module path and `/v2`; on a `/v3` migration it fires 21 false errors naming the wrong cause. Derive it from `go.mod`. — *agent: Opus 5 · high — standing rule 1*
- [ ] [#140](https://github.com/danielPoloWork/egl-utils-go/issues/140) · `MINOR` · `ci` `docs` — the usage guide's 22 Go snippets are compiled by nothing; extract them into a compiled test file or generate the guide from its sources. — *agent: Opus 5 · xhigh — the largest piece of real engineering in the backlog, and the fix decides how a 438-line document stays true*
- [ ] [#139](https://github.com/danielPoloWork/egl-utils-go/issues/139) · `MINOR` · `ci` `test` — the race detector never runs on Windows or macOS, the two platforms the README promises to support. — *agent: Sonnet 5 · high — well-specified, but it changes CI cost and duration*
- [ ] [#138](https://github.com/danielPoloWork/egl-utils-go/issues/138) · `MINOR` · `ci` — check 12 reads one directory level and treats every directory as a package; discover packages by the presence of `.go` files. — *agent: Opus 5 · high — standing rule 1*
- [ ] [#137](https://github.com/danielPoloWork/egl-utils-go/issues/137) · `MINOR` · `ci` — check 12's bijection runs against the whole README while its identifier and all three error messages say "Packages section". — *agent: Opus 5 · high — standing rule 1*
- [ ] [#136](https://github.com/danielPoloWork/egl-utils-go/issues/136) · `MINOR` · `ci` — check 12 passes a dead link carrying an extra path segment; anchor the pattern at the end of the segment. — *agent: Opus 5 · high — standing rule 1, and this is the second regex defect in the same check*
- [ ] [#135](https://github.com/danielPoloWork/egl-utils-go/issues/135) · `MINOR` · `docs` `security` — `SECURITY.md`'s only concrete version link points at `v2.0.0`, which its own table marks unsupported. — *agent: Sonnet 5 · low*
- [ ] [#134](https://github.com/danielPoloWork/egl-utils-go/issues/134) · `MINOR` · `security` `ci` — `sha_pinning_required` is false and `allowed_actions` is `all`; the tree already complies, so enabling both costs nothing. — *agent: Sonnet 5 · medium — a settings hand-off; prefer the sub-resource endpoint*
- [ ] [#133](https://github.com/danielPoloWork/egl-utils-go/issues/133) · `MINOR` · `docs` — the usage guide teaches silent fallback on malformed input under a heading promising safety. — *agent: Sonnet 5 · medium*
- [ ] [#132](https://github.com/danielPoloWork/egl-utils-go/issues/132) · `MINOR` · `security` `ci` — `nfr-nightly` installs `benchstat@latest`, the one unpinned executable in a tree whose policy is total pinning. — *agent: Sonnet 5 · low*
- [ ] [#131](https://github.com/danielPoloWork/egl-utils-go/issues/131) · `MINOR` · `security` `ci` — `govulncheck` reports one non-reachable vulnerability and nothing names the module or CVE; run with `-show verbose` and record it. — *agent: Sonnet 5 · medium — read the whole output block before acting: the same block says "affected by 0"*
- [ ] [#130](https://github.com/danielPoloWork/egl-utils-go/issues/130) · `MINOR` · `ci` — nothing acts on an NFR regression, and `|| true` lets the comparison go blind while the job stays green. — *agent: Opus 5 · high — the project's own rule is never to gate a tail, so "make it fail" is not automatically the right fix*
- [ ] [#129](https://github.com/danielPoloWork/egl-utils-go/issues/129) · `MINOR` · `ci` — the nightly NFR baseline is the previous night, so gradual drift is structurally undetectable; anchor to a fixed release baseline too. — *agent: Opus 5 · high — a measurement-design decision, and this project has already read a throughput figure as a latency*
- [ ] [#128](https://github.com/danielPoloWork/egl-utils-go/issues/128) · `MINOR` · `docs` — one 15s budget covers both shutdown hooks, so accepted work is dropped on every deploy with one log line. — *agent: Opus 5 · medium*
- [ ] [#127](https://github.com/danielPoloWork/egl-utils-go/issues/127) · `MINOR` · `docs` — BUG-0002's "repository-wide sweep" heading rests on evidence scoped to `_test.go`; state the scope actually swept and record the library-code result. — *agent: Opus 5 · medium — pairs with #126; fix the record and the code path together or neither is closed*
- [ ] [#126](https://github.com/danielPoloWork/egl-utils-go/issues/126) · `MINOR` · `test` — `workerpool.New(n, 0, WithNonBlockingSubmit())` is BUG-0002's exact primitive reachable from the public API, and no test covers `queueSize 0`. — *agent: **Fable 5 · xhigh** — concurrency correctness plus an API decision: documenting the timing dependence and rejecting the combination are different promises, and one of them is breaking*
- [ ] [#125](https://github.com/danielPoloWork/egl-utils-go/issues/125) · `MINOR` · `ci` `contrib` — `contrib-release.yml` has never executed once, yet the runbook calls submodule tags "verified mechanically". — *agent: Opus 5 · high — exercising it publishes something; the first run must not be during a real release*
- [ ] [#124](https://github.com/danielPoloWork/egl-utils-go/issues/124) · `MINOR` · `ci` — the tag-to-Release step can half-fail, leaving the version distributed and the artifacts absent; draft first, attach, then publish. — *agent: Opus 5 · high — this is materially the shape `v0.1.0` is in*
- [ ] [#123](https://github.com/danielPoloWork/egl-utils-go/issues/123) · `MINOR` · `ci` — `CHANGELOG.md` is parsed by no gate at all, so a release PR could skip the roll entirely and stay green. — *agent: Opus 5 · high — standing rule 1; this adds a check rather than editing one*
- [ ] [#122](https://github.com/danielPoloWork/egl-utils-go/issues/122) · `MINOR` · `ci` — nothing compares the git tag to `version.go`; the one comparison spanning the repo/published boundary is the one nobody makes. — *agent: Sonnet 5 · medium*
- [ ] [#121](https://github.com/danielPoloWork/egl-utils-go/issues/121) · `MINOR` · `ci` — `release.yml` never checks the tag is reachable from `master`, while `contrib-release.yml:112-120` does. — *agent: Sonnet 5 · medium — a port of an existing step*
- [ ] [#120](https://github.com/danielPoloWork/egl-utils-go/issues/120) · `MINOR` · `docs` — no rollback path is documented and module-proxy immutability is nowhere acknowledged; name `retract` as the instrument. — *agent: Opus 5 · high — pairs with #115; the unwritten constraint is why the unsound remedy got into the runbook*
- [ ] [#119](https://github.com/danielPoloWork/egl-utils-go/issues/119) · `MAJOR` · `test` — a returning BUG-0002 would be caught roughly one run in three, by a 10-minute timeout, on a job that is now a required check. — *agent: **Fable 5 · high** — designing regression protection for a timing defect, where the wrong instrument (a sleep) converts a loud failure into a silent one*
- [ ] [#118](https://github.com/danielPoloWork/egl-utils-go/issues/118) · `MAJOR` · `test` — three `pkg/**` tests use a sleep as an ordering barrier and can pass **vacuously**: BUG-0002's class, silent instead of loud. — *agent: **Fable 5 · xhigh** — concurrency-critical; the repository already has an applied clock-seam style to follow rather than invent*
- [ ] [#117](https://github.com/danielPoloWork/egl-utils-go/issues/117) · `MAJOR` · `test` `ci` — check 12 has no negative test, and the lint's negative suite is not wired into CI at all, covering 3 of 12 checks against a different file. — *agent: Opus 5 · xhigh — standing rule 1 at its sharpest: the task is to make a gate provably able to fail*
- [ ] [#116](https://github.com/danielPoloWork/egl-utils-go/issues/116) · `MAJOR` · `docs` — the `CHANGELOG` `Added` section describes only documentation and carries no disclaimer, inviting a wrong `v2.1.0`. — *agent: Opus 5 · medium — a SemVer ruling, not a wording fix*
- [ ] [#115](https://github.com/danielPoloWork/egl-utils-go/issues/115) · `MAJOR` · `docs` — "delete and repush an unpublished tag" is unsound for a Go module: the tag push *is* the publication. — *agent: Opus 5 · high — a live trap in the runbook; the remedy would corrupt every consumer who already fetched*
- [ ] [#114](https://github.com/danielPoloWork/egl-utils-go/issues/114) · `MAJOR` · `docs` — compliance control register C-6 still says `required_signatures` is unenforced; the PR that enabled it touched no compliance file. — *agent: Sonnet 5 · medium*
- [ ] [#113](https://github.com/danielPoloWork/egl-utils-go/issues/113) · `MAJOR` · `security` `build` — the published SBOM covers 1 of 4 modules, and the three excluded ones are the ones with real dependency closures. — *agent: Opus 5 · high — extending the release workflow across a module boundary, or scoping a public promise*
- [ ] [#112](https://github.com/danielPoloWork/egl-utils-go/issues/112) · `MAJOR` · `security` `ci` — Dependabot alerts **and** security updates are both disabled; nothing watches this module between commits. — *agent: Sonnet 5 · low — two toggles, and the cheapest real risk reduction in this list*
- [ ] [#111](https://github.com/danielPoloWork/egl-utils-go/issues/111) · `MAJOR` · `docs` — the `v2.0.1` Release body still announces the `v0.1.0` backfill that never happened; the correction landed in six in-repo records and not in the artifact carrying the claim. — *agent: Opus 5 · medium — standing rule 2; no tag can fix a Release body, and the correction goes beside the original, not over it*
- [ ] [#110](https://github.com/danielPoloWork/egl-utils-go/issues/110) · `MAJOR` · `docs` `ci` — the README compatibility section overstates the CI matrix; Go 1.25, the declared minimum, is never tested on Windows or macOS. — *agent: Opus 5 · medium — widening the matrix and narrowing the sentence are different commitments*
- [ ] [#109](https://github.com/danielPoloWork/egl-utils-go/issues/109) · `MAJOR` · `docs` — the usage guide's retry recipe demonstrates the retry storm the paragraph beneath it warns against; the caveat exists in the source it was derived from. — *agent: Sonnet 5 · medium — the correct text already exists at `pkg/retry/example_test.go:14-21`*
- [ ] [#108](https://github.com/danielPoloWork/egl-utils-go/issues/108) · `MAJOR` · `docs` — the README quickstart ships a checkless `/healthz` returning a constant 200, and no readiness endpoint, in a service whose point is load shedding. — *agent: Opus 5 · high — readiness must reflect queue saturation, which is the whole lesson the composition exists to teach*
- [ ] [#107](https://github.com/danielPoloWork/egl-utils-go/issues/107) · `MAJOR` · `docs` `security` — the README quickstart discards the `ListenAndServe` error and sets no `ReadHeaderTimeout`: a process alive and silent, and Slowloris exposure. — *agent: Opus 5 · high — found independently by three disciplines; the fix must mirror `examples/service/main.go:72-83,95-108`, which already does it correctly*
- [ ] [#106](https://github.com/danielPoloWork/egl-utils-go/issues/106) · **`BLOCKER`** · `docs` `bug` — the README's headline promise "no package here imports another" is false: `pkg/config/config.go:47` imports `pkg/validator`, and `import_graph_lint.py` **mandates** that edge. — *agent: Opus 5 · high — standing rule 2. One sentence, and it is the sentence the release verdict turned on: it must name the sanctioned edge and cite ADR-0035 as the record of it, not as proof of its absence*
