# ADR-0023: validator.Struct design — reflection tag grammar, literal rules, panic on tag misuse

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 19, §5 (`Struct(v any) error` — tag grammar `required, email, min,
  max, oneof`); ROADMAP 8.1 (opens Milestone 8); ADR-0004 (runtime dependency policy — the validator
  is hand-rolled, no framework); ADR-0005 (loud-by-default)

## Context

Feature 19 is a tag-driven struct validator, frozen at intake as `Struct(v any) error` with the
grammar `required, email, min, max, oneof`. ADR-0004 already decided this is built in-repo, not
imported (no `go-playground/validator` dependency). The open decisions are the semantics that the
one-line grammar leaves unstated: what "required" means across types, how `min`/`max` read on
different kinds, whether unlisted rules are optional, how failures are reported, how nesting works,
and — the decision that shapes the whole feel of the API — what happens when a *tag itself* is wrong
versus when *data* is wrong.

## Decision

**Two failure channels, kept strictly separate.** A **data** violation (a value breaking a valid
rule) is returned as an error. A **definition** error (a tag that cannot apply — `email` on a
non-string, `min` on a bool, an unknown rule, a non-numeric `min`/`max` parameter) is a bug in the
struct declaration and **panics** (ADR-0005 loud-by-default). This keeps the returned
`error` purely about the data under validation; a malformed tag can never masquerade as a validation
failure, and it surfaces loudly the first time the struct is validated, pointing at the bad tag.

**Rules apply literally, in order, with no implicit "optional".** Each comma-separated rule is
checked as written; a field carrying `min=3` must satisfy it regardless of whether it is also
`required`. There is no `omitempty`-style skip — the frozen grammar has none, and a literal reading
is the most predictable. A field that should be optional-but-constrained is expressed by making it a
pointer (a nil pointer is its own zero value and only `required` reacts to it). `omitempty` is a
clean additive extension for later.

**Rule semantics by kind:**
- `required` — the field is not its zero value (`reflect.Value.IsZero`); works on every kind
  (empty string, 0, nil pointer/slice/map, false, zero struct).
- `email` — a pragmatic regexp (`local@domain.tld`, no spaces/control chars, a dotted domain), not an
  exhaustive RFC 5322 grammar; string-only.
- `min` / `max` — **length** for strings (counted in **runes**, not bytes) and for
  slices/maps/arrays; **value** for signed, unsigned, and floating numbers.
- `oneof` — the field's scalar rendering equals one of the space-separated options; strings,
  numbers, and bool.

**Nested structs are validated recursively**, with a dotted field path (`Address.Zip`). A non-nil
pointer-to-struct is descended into; a nil one is not (nothing behind it), so an optional nested
section is a nil pointer. Unexported and untagged fields are skipped; `validate:"-"` skips a field
explicitly.

**Aggregate, don't fail-fast.** `Struct` collects **every** failure into a `ValidationErrors`
(`[]*FieldError`), so one call reports all problems — the behaviour a form or config wants. Each
`FieldError` carries the dotted field, the failed rule, and its parameter; `ValidationErrors.Unwrap()
[]error` lets `errors.As` pull out a specific `*FieldError`.

**Input contract.** `v` must be a struct or a non-nil pointer to a struct; nil, a nil pointer, or a
non-struct panics — misuse, not a validation outcome.

## Alternatives Considered

- **Import `go-playground/validator`** — batteries-included and battle-tested, but a substantial new
  runtime dependency ADR-0004 explicitly rules out for this feature ("implemented in-repo against the
  spec's tag grammar"). Rejected by prior decision.
- **Return an error for tag misuse instead of panicking** — friendlier in theory, but it forces every
  caller to handle an error only a developer bug can produce and blurs the data/definition line. A
  bad tag is a compile-time-ish mistake surfaced at first validation; a panic is the honest signal.
  Rejected for the panic.
- **Fail-fast on the first violation** — simpler, but a validator that reports one error at a time is
  a poor form/config experience. Rejected for full aggregation.
- **Skip constraints on zero values (implicit omitempty)** — more convenient for optional fields, but
  surprising for `min` on numbers (a 0 that should be rejected would be skipped) and not in the frozen
  grammar. Rejected; literal rules + pointer-for-optional, with `omitempty` deferred.
- **Byte length for `min`/`max` on strings** — cheaper, but rune length is what a human means by
  "at most 20 characters". Rejected for rune counting.
- **`dive` into slice elements** — powerful, but beyond the frozen grammar and a larger surface.
  Deferred as additive.

## Consequences

- The public surface gains the `validator` package: `Struct`, `FieldError`, `ValidationErrors`. No
  runtime dependency; the reflection walk is self-contained.
- Data errors are returned and fully enumerated; definition errors panic — the two never mix.
- Nested structs and pointers validate with dotted paths; optional sections are nil pointers.
- Milestone 8 opens; `hash.HashPassword`/`CheckPassword` (8.2) — the security-relevant half —
  completes it under the security-auditor's sign-off.
- Deferred, additive surface: `omitempty`, `dive`, custom rules, JSON-tag field names in errors — all
  reachable without breaking `Struct(v any) error`.

## References

- `docs/specs/01_spec_utils.md` §2.19, §5.
- ADR-0004 (hand-rolled, no validation framework), ADR-0005 (loud-by-default).
- `reflect`, `regexp`, `unicode/utf8`, `errors` (multi-`Unwrap`).
