# 2026-07-15 — Milestone 8 opens: validator.Struct

## What got done

- **Roadmap 8.1 `validator.Struct`** (branch `feat/validator-struct`, draft PR #27, ADR-0023): the first of
  Milestone 8 and the largest single surface in the project (reflection tag grammar). Spec §5 froze
  `Struct(v any) error` with the rules `required, email, min, max, oneof`. **Hand-rolled** — no
  `go-playground/validator` — as ADR-0004 already mandated.
- **The shaping decision: two failure channels, kept strictly separate.** A **data** violation (a
  value breaking a valid rule) is *returned*; a **definition** error (a tag that cannot apply —
  `email` on a non-string, `min` on a bool, an unknown rule, a non-numeric bound) *panics*
  (ADR-0005). The returned error is therefore only ever about the data under validation; a bad tag
  surfaces loudly at first use, pointing at the struct-definition bug.
- **Rules apply literally, no implicit optional.** `min=3` must hold whether or not the field is
  `required`; optionality is expressed by making the field a pointer (nil is its own zero value,
  only `required` reacts). `omitempty`/`dive` deliberately deferred (additive). `min`/`max` measure
  **rune length** for strings and collections, **value** for numbers; `oneof` matches the scalar
  rendering for strings/numbers/bool; `email` is a pragmatic regexp, not RFC 5322.
- **Nested structs recurse** with dotted paths (`Address.Zip`); a non-nil pointer-to-struct is
  descended into, a nil one is not (optional section = nil pointer). Unexported / untagged fields and
  `validate:"-"` are skipped. **All** failures aggregate into a `ValidationErrors` (`[]*FieldError`)
  whose `Unwrap() []error` lets `errors.As` reach a specific `*FieldError`.
- Tests are comprehensive (100% coverage): per-rule pass/fail, rune-length, uint/float/bool branches,
  nested value + pointer structs, aggregation, `errors.As`, unexported/untagged/`-` skips, a
  `pgregory.net/rapid` property for string min/max, a runnable `ExampleStruct`, and a table of every
  panic path (nil/non-struct/nil-pointer input, email-on-int, min-on-bool, unknown rule, bad
  int/uint/float params, oneof-on-slice).
- Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, **100% validator
  coverage**, gofumpt clean, golangci-lint v2 0 issues, govulncheck clean, `consistency_lint.py` OK.

## Where the project stands

M1–M7 complete and merged; **M8 in progress (1 of 2)**. 8.1 validator.Struct drafted on
`feat/validator-struct` (draft PR #27), awaiting the maintainer to open and merge (one PR at a time). README
milestone table: M8 → in progress. **Six completed milestones remain unreleased** (M2→v0.2.0 …
M7→v0.7.0); the maintainer keeps deferring the cut.

## How the next session resumes

Wait for the 8.1 PR to merge. Then **8.2 `hash.HashPassword` / `hash.CheckPassword`** — bcrypt via
**`golang.org/x/crypto/bcrypt`** (ring 2, ADR-0004-permitted; the first new runtime dependency since
yaml.v3). This is **security-relevant** → it needs its own ADR **and the security-auditor's sign-off**
(AGENTS.md §10), and likely a new compliance control (password hashing at rest). The bcrypt **cost
factor** (default vs configurable, and the DoS trade-off of a high cost) is the design/review point;
CheckPassword must use `bcrypt.CompareHashAndPassword` (constant-time) and never leak whether the
user or the password was wrong. That completes Milestone 8. Portable Go under `%TEMP%\go-portable`;
`/v2` golangci-lint path; `-race` CI-only.
