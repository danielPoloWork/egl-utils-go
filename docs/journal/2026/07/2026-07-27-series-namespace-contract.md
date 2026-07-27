# 2026-07-27 — Governance: namespace contract & spec reconciliation (Milestone 11)

## What got done

The session opened with a maintainer question, not a task: *the other utils projects keep their
sources under `src/main/<lang>/it/d4np/utils` — can this repo be refactored to match?* The answer
turned out to require correcting the premise twice.

- **The refactor is blocked by the language, and the decision already existed.** In Go an import
  path *is* the directory path beneath the module root, so packages under the tree could only be
  imported as `…/src/main/go/it/d4np/utils/workerpool` (83 characters against today's 52). That is
  exactly why [ADR-0003](../../../adr/0003-adopt-idiomatic-go-root-layout.md) superseded ADR-0002
  in July. Git history shows the tree never held a single `.go` file — three `.gitkeep`s at
  bootstrap (PR #6), removed the same milestone — so re-imposing it would be a fresh move of 24
  packages, not a restoration. It would also force a `/v2` (relocating packages breaks every
  import under the v1.0.0 commitment), taking the import to 86 characters, and ADR-0040's contrib
  submodules would inherit the prefix in their module paths *and* their release tags
  (`src/main/go/it/d4np/utils/contrib/redishealth/v1.0.0`).
- **The premise was checked against the siblings rather than accepted.** The tree is 1-for-3 among
  the scaffolded repositories: `egl-util-cpp` keeps it; `egl-utils-c`'s own ADR-0002 is
  *adopt-**module-oriented**-source-layout* and ships `d4np/{core,concurrency,ds,io,mem,str,sys}/`
  at the root; `egl-utils-java` is unscaffolded (and will get the tree free, since Maven mandates
  it). Two of three abandoned it independently, each because its language binds names to paths
  differently. So the series had already converged on *idiomatic per language* — it just had not
  said so anywhere.
- **[ADR-0041](../../../adr/0041-series-logical-namespace.md) reframes what the series shares.**
  ADR-0002 fixed the **directory shape**; what actually needs fixing is the **namespace**. A
  physical layout cannot be a cross-language contract because in most of these languages the
  layout is not free — Maven dictates Java's, the include model dictates C/C++'s, the module
  system dictates Go's. A namespace can be. The contract is now
  `it.d4np.utils.<component>`, realized per language, with `<component>` spelled identically
  everywhere; Go realizes it at the module root, so
  `it.d4np.utils.workerpool` **is** `github.com/danielPoloWork/egl-utils-go/workerpool`.
- **The module path was deliberately not changed.** The maintainer weighed the two options that
  would render the namespace literally in Go source and declined both: a vanity
  `go.d4np.it/utils` (an element-for-element mirror of the reverse-DNS name, but it converts a
  documentation preference into a permanent supply-chain obligation — a domain that must keep
  resolving and a `go-import` meta tag that must keep being served for as long as anyone runs
  `go get`), and a `github.com/d4np/…` organisation (brand without the hosting commitment, but it
  renders neither `it.` nor a clean `utils`, so it pays a repo transfer and a breaking import
  change for a partial match). Both are module *identity* changes and therefore breaking either
  way. Imports are unchanged; no `/v2` opens for namespace reasons.
- **The `/v2` ledger gained an addendum rather than a pending item.** ADR-0030 §2 now records the
  module path as *declined with a condition*, because the economics are timing-dependent: the move
  is free at a `/v2` boundary and only there, since consumers rewrite every import at such a
  boundary regardless. If bucket 2 is ever opened, the path is re-evaluated in the same breath.
- **ADR-0003's regeneration caveat is finally closed.** That ADR noticed `orchestrator/project.yaml`
  still described the tree and that re-rendering would re-impose it — then resolved the conflict
  only by declaring ADR precedence, which protects a careful reader and nothing else. The manifest
  is now amended in place with a dated note at both hazardous spots, so the record still shows what
  the interview asked while the generator can no longer act on it.

- **A second thread opened from the first, and closed as 11.2.** Correcting the stale `Go 1.24`
  language floor (nine places: AGENTS §1/§5/§10, README, `local-build.md`, both GitHub templates,
  four `project.yaml` fields) led into `docs/specs/01_spec_utils.md`, which is **frozen** — and
  which had diverged in **four** places, not one. The useful discovery was that they are not the
  same kind of divergence:
  - **Three were facts that had drifted**, and the spec's own header authorises fixing them in
    place ("diverging implementation updates this spec in the same PR or adds an ADR"): the
    language floor (1.24 → 1.25), the coverage floor (80% → **85% per package** since
    10.9/ADR-0036 — "per package" being the half that matters, since with most packages at 100% a
    module-wide average could never fail), and the §3/§6 hedge promising "an in-repo stack-based
    guard until ROADMAP 2.6 lands the test-only deps", describing a state that ended in M2.
  - **One was a rule that had been replaced**, which is a different thing entirely. §3's
    compatibility clause said breaking changes "require a MAJOR-intent note in the PR" — i.e. they
    were **mergeable**, conditionally. Under the v1.0.0 commitment they are not mergeable into
    v1.x at all; ADR-0030's bucket 2 has been deferring them for a milestone. So it took the ADR
    branch: **[ADR-0042](../../../adr/0042-post-1.0-compatibility-contract.md)** states the
    post-1.0 contract, retires the MAJOR-intent note, and makes the `/v2` ledger the only
    sanctioned destination for a breaking change. **The original clause is struck in place, not
    rewritten** — overwriting it would erase the evidence that a promise to consumers had changed,
    inside the one document whose purpose is to be the frozen contract.
  - The spec now carries a dated **Amendments** block that keeps the two branches visibly
    distinct, so the next divergence has a precedent to follow rather than a judgment call to
    re-make.

## Where the project stands

**Milestone 11 complete (2/2)**, documentation only — no code, no version bump, no consumer-visible
change, so `version.go` stays at 1.1.0 and `CHANGELOG.md` gets no entry (the 10.1 precedent: pure
governance items do not appear in a Keep-a-Changelog file). v1.1.0 remains the released version.

## How the next session resumes

Two things were found during this session and deliberately **not** fixed in it (a third, the stale
1.24 language floor, was fixed as part of 11.2):

- **The EADOS bundle's Go profile still asserts the tree.**
  `.eados-core/orchestrator/profiles/go.yaml` says the module is placed "under
  `src/main/go/<group>/<slug>`". `.eados-core/` is gitignored — it is factory tooling copied in,
  not this project's source — so the correction cannot be committed here and must be carried
  upstream to the framework. Until it is, a regeneration from a *fresh* bundle can still
  reintroduce the claim; ADR-0041 and the amended manifest are what a future agent should trust.
- **`egl-util-cpp` ships `it/d4np/util` (singular)** where ADR-0041's contract says `utils`. Under
  the component-name rule one of the two must move. It affects another repository, so it is a
  series-level call and was not taken here.
The one open question 11.2 leaves for the maintainer is **how much else of the frozen spec has
drifted**. Four divergences surfaced from a sweep that was only looking for one number, and nothing
mechanically checks the spec against reality — `consistency_lint.py`'s spec-map check verifies that
every §ction has a fulfilling roadmap item, not that the §ction's *claims* are still true. A
deliberate read-through of `01_spec_utils.md` against as-built behaviour is the obvious next
governance item; the Amendments block is now the place its findings would land.

Otherwise the carry-overs from the v1.1.0 cut are unchanged: the ADR-0030 `/v2` ledger, the NFR-01
spec amendment, the `middleware.HeaderName` canonicalisation decision, and first tags for the two
contrib modules (still unreleased, so nobody can `go get` them at a version yet).
