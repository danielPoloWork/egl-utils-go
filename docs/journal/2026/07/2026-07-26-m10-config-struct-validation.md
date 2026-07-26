# 2026-07-26 — Milestone 10.6: config.WithStructValidation()

## What got done

- **Roadmap 10.6 `config.WithStructValidation()`** (branch `feat/config-struct-validation`): wires
  `validator.Struct` into `config.Load` as a functional option, which is what spec v2 item 13 means by
  "configuration with struct validation (via item 19)". **New
  [ADR-0033](../../../adr/0033-config-struct-validation.md)**.
- **The item was tagged *low* effort, and the code is small — but it turned out to carry the module's
  first architectural precedent.** `config → validator` is the **first internal package edge in the
  module**: verified that to date no feature package imports another (every internal import was
  test-only). Spec §3 puts both at L2 with "arrows point downward only", which is written about
  crossing layers and is silent on a same-layer edge. Since spec item 13 mandates exactly this
  composition, the edge is legal — and **roadmap 10.8 now carries an explicit note that its depguard
  rules must permit it or the build breaks**, with the rule to express being "same-layer edges only
  where the spec mandates the composition", not "L2 is a free-for-all".
- **Tags run before `Validator`, and a tag failure skips `Validate` entirely.** That ordering is the
  real design decision, and it is a contract rather than an implementation accident: the tag layer is
  declarative and per-field, so running it first lets a `Validate` method assume every field is
  individually well-formed and concern itself only with the cross-field invariants no tag can express.
  Rejected `Validate`-first (it would have to defend against malformed field data the tag layer exists
  to reject) and rejected joining both errors (`Validate` would then run against data already known
  bad). Note the fail-fast boundary is only *between* the layers — the tag pass itself still reports
  every violated field at once.
- **Opt-in, not implicit.** Enabling tag validation automatically whenever a struct has `validate` tags
  was tempting and is wrong: a struct with *no* tags passes vacuously, so implicit enablement would
  silently imply a guarantee that does not exist, and adding a tag later would retroactively change
  `Load`'s behaviour — a change the v1.0.0 commitment does not allow. A test asserts the same file that
  fails *with* the option loads cleanly without it.
- **The validator's panics are not softened into errors.** A malformed tag, a rule on an incompatible
  type, or a non-struct `T` panics per ADR-0023, and `Load` documents that rather than re-classifying
  it: tag misuse is a programming error there by deliberate decision, and converting it here would mean
  the same mistake behaves differently depending on which entry point found it.
- Tests: `config` stays at **100% coverage**. Coverage of the ordering contract needed a counter
  outside the struct — `Load` decodes into a value it owns, so an injected field is always nil; the
  first draft had exactly that bug and the assertion would have passed vacuously. Those tests are
  documented as non-parallel. Also covered: every violation reported in one pass, `errors.As` reaching
  `ValidationErrors` and an individual `*FieldError`, composition with `WithoutEnvExpansion` (expansion
  happens before validation, so a disabled expansion leaves a literal `${VAR}` to fail the email rule),
  and the non-struct panic.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...`, gofumpt clean,
  golangci-lint v2 0 issues, govulncheck 0 affecting, `consistency_lint.py` OK. No dependency change —
  `validator` is hand-rolled in-repo (ADR-0004), so the new edge costs nothing in dependency budget.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (6 of 13)**: 10.1 (#37), 10.2 (#38), 10.3 (#46), 10.4 (#47),
and 10.5 (#48) merged; 10.6 drafted on `feat/config-struct-validation`, awaiting the maintainer to open
and merge. M10 releases as v1.1.0.

## How the next session resumes

Wait for the 10.6 PR to merge. Then **10.7 fuzzing** (spec §7, roadmap tags it *high*):
`FuzzConfigLoader` (JSON/YAML inputs) and `FuzzValidatorTags`, committed seed corpora under
`testdata/fuzz/`, and a CI fuzz job with a 10-minute budget. Two things to think about before writing
code: `config.Load` reads a *file path*, so the fuzz target needs to write each input to a temp file (or
the loader's byte path needs extracting) — decide which without bending the public API; and
`validator.Struct` **panics by contract** on malformed tags (ADR-0023), so a naive tag fuzzer will
"find" those panics immediately — the target must distinguish a contract panic from a genuine crash,
probably by fuzzing tag *values* against a fixed valid grammar rather than fuzzing arbitrary tag text.
Standard footprint per PR (tests + goleak + coverage, CHANGELOG `[Unreleased]`, ROADMAP checkbox,
journal, lint). Portable Go under `%TEMP%\go-portable` — in the Bash tool add it as the *unix* path
`/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs the `/v2` module path;
`-race` is CI-only.
