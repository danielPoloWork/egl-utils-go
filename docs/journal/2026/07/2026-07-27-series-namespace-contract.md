# 2026-07-27 — Series namespace contract (Milestone 11)

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

## Where the project stands

**Milestone 11 complete (1/1)**, documentation only — no code, no version bump, no consumer-visible
change, so `version.go` stays at 1.1.0 and `CHANGELOG.md` gets no entry (the 10.1 precedent: pure
governance items do not appear in a Keep-a-Changelog file). v1.1.0 remains the released version.

## How the next session resumes

Three things were found during this session and deliberately **not** fixed in it:

- **The EADOS bundle's Go profile still asserts the tree.**
  `.eados-core/orchestrator/profiles/go.yaml` says the module is placed "under
  `src/main/go/<group>/<slug>`". `.eados-core/` is gitignored — it is factory tooling copied in,
  not this project's source — so the correction cannot be committed here and must be carried
  upstream to the framework. Until it is, a regeneration from a *fresh* bundle can still
  reintroduce the claim; ADR-0041 and the amended manifest are what a future agent should trust.
- **`egl-util-cpp` ships `it/d4np/util` (singular)** where ADR-0041's contract says `utils`. Under
  the component-name rule one of the two must move. It affects another repository, so it is a
  series-level call and was not taken here.
- **AGENTS.md contradicts itself on the language floor.** `go.mod` declares `go 1.25.0` and §10's
  build-matrix row says "module floor 1.25" (corrected by 10.9/ADR-0036), but three other places
  still say 1.24 — §1 line 20, the §5 layout block, and §10's "Language standard" line. A separate
  one-item `docs:` PR, kept out of this one to preserve the one-item-per-PR rule.

Otherwise the carry-overs from the v1.1.0 cut are unchanged: the ADR-0030 `/v2` ledger, the NFR-01
spec amendment, the `middleware.HeaderName` canonicalisation decision, and first tags for the two
contrib modules (still unreleased, so nobody can `go get` them at a version yet).
