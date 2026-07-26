# ADR-0033: `config.WithStructValidation` — opt-in tag validation, and the module's first internal package edge

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0018 (`config.Load` — the `Validator` interface), ADR-0023 (`validator.Struct` — tag grammar, panic on tag misuse), ADR-0004 (dependency policy), ADR-0030 (spec v2 reconciliation: 10.6 adopted additively), spec v2.0 item 13 ("configuration with struct validation (via item 19)") and §3 (layered import graph), roadmap 10.6, roadmap 10.8 (depguard enforcement)

## Context

Spec v2 item 13 describes `config.Loader` as "JSON/YAML/env configuration **with struct validation
(via item 19)**" — item 19 being `validator.Struct`. Today the two packages are unaware of each other:
`config.Load` offers the `Validator` interface (ADR-0018), a config type checking its own invariants
via a `Validate() error` method, while `validator.Struct` (ADR-0023) applies the hand-rolled
`validate:"required,email,min,max,oneof"` tag grammar. A consumer wanting both writes the glue.

Two decisions follow, and the second is larger than the roadmap's *low* effort tag suggests.

**How the two mechanisms relate.** They are not alternatives: tags are declarative and per-field,
`Validate` is imperative and can see the whole struct. Their interaction — both, either, or ordered —
has to be a documented contract rather than an accident of implementation order.

**The import graph.** `config` importing `validator` would be the **first internal package edge in
the module**: to date no feature package imports another (verified — every internal import is
test-only). Spec §3 places both at L2 with "arrows point downward only", which is written about
crossing layers and says nothing explicit about a same-layer edge. Roadmap 10.8 will encode the §3
rules as depguard rules, so whatever precedent this sets has to be one 10.8 can express.

## Decision

`WithStructValidation()` is an opt-in `Option` that runs `validator.Struct(&cfg)` after decoding and
**before** the `Validator` interface, with a tag failure returning immediately so `Validate` never sees
a struct whose individual fields are already known bad. `config` therefore takes a direct dependency
on `validator`, establishing a **same-layer (L2 → L2) internal edge as legal**, on the specific grounds
that spec item 13 mandates this exact composition.

Off by default, tags first, `Validate` second, both able to run in one load.

## Alternatives Considered

- **Enabling tag validation implicitly** whenever the struct has `validate` tags — no option needed,
  and arguably what a reader expects. Rejected on the failure mode: a struct with *no* tags passes
  vacuously, so implicit enablement would silently imply a guarantee that does not exist, and adding a
  tag to a struct would retroactively change `Load`'s behaviour. It is also a behaviour change to an
  existing function, which the v1.0.0 stability commitment does not allow.
- **`Validate` before tags** — lets a config type normalise fields (fill defaults, trim strings)
  before the declarative rules judge them. Genuinely useful, and rejected because it inverts the
  responsibility: `Validate` would then have to defend against malformed per-field data that the tag
  layer exists to reject, and the cheap declarative check would run only after arbitrary user code.
  Tags first keeps each layer's contract narrow. A consumer needing normalisation-then-validation can
  decode without the option and call `validator.Struct` itself.
- **Running both and joining the errors** (`errors.Join`) so one load reports every problem — better
  diagnostics in principle. Rejected because `Validate` would then run against data already known to
  violate its field rules, and a `Validate` written against the documented guarantee (fields are
  individually valid) could panic or produce a nonsense second error. Note that the tag layer *does*
  report every field violation in one pass — the fail-fast boundary is only *between* the two layers.
- **A `WithValidator(func(T) error)` general hook** instead of a validator-specific option — no
  internal edge at all, and the consumer wires `validator.Struct` in themselves. Rejected because it
  declines what item 13 actually asks for: the composition would remain the consumer's boilerplate,
  which is the thing the spec wants removed. It is also a strictly larger API surface (a generic
  function-valued option) than a flag.
- **Putting the glue in a third package** to keep `config` and `validator` mutually unaware. Rejected
  as ceremony: a package existing only to call one function from another, and a worse import story for
  consumers than the direct edge.
- **Converting the validator's panics into `Load` errors** — a malformed tag or a non-struct `T` would
  become a returned error rather than a panic. Rejected as overriding ADR-0023 from the outside: tag
  misuse is a programming error there by deliberate decision, and re-classifying it here would mean the
  same mistake behaves differently depending on which entry point found it. `Load` documents the panic
  instead; it surfaces on the first load, which is where a wiring mistake belongs.

## Consequences

- **API:** `config` gains `WithStructValidation()`. Additive — a `Load` call without it behaves exactly
  as before, tags inert, which a test asserts directly against a file that fails *with* the option.
- **The internal import graph now has one edge, `config → validator` (L2 → L2), and it is a
  precedent.** Roadmap **10.8 must encode this explicitly** when it writes the depguard rules, or the
  build breaks: the rule to express is that same-layer edges are permitted only where the spec mandates
  the composition, not that L2 is a free-for-all. This is the one architectural consequence of an
  otherwise small item, and it is why this ADR exists.
- No new module dependency; `validator` is hand-rolled in-repo (ADR-0004), so the edge costs nothing in
  dependency budget and cannot introduce a version conflict.
- Errors stay inspectable: `Load` wraps with `%w`, so `validator.ValidationErrors` (and, through its
  `Unwrap() []error`, each `*validator.FieldError`) is reachable with `errors.As` — asserted rather
  than assumed.
- `config` stays at 100% coverage. The ordering contract needs a counter outside the struct to observe,
  because `Load` decodes into a value it owns; those tests are documented as non-parallel.
- Deferred, additive: a general `WithValidator` hook, and normalise-then-validate ordering, if a
  consumer presents the need.

## References

- Spec v2.0 item 13 (config with struct validation via item 19), item 19, §3 (layered import graph).
- ADR-0018 (`Validator` interface, `any(&cfg).(Validator)` so a pointer receiver is found), ADR-0023
  (tag grammar; panic on malformed tag, incompatible type, or non-struct), ADR-0030 (adoption bucket).
- `config/structvalidation_test.go`; roadmap 10.8 (depguard) — the follow-up this ADR constrains.
