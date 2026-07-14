# ADR-0018: config.Loader design — generic Load, extension-driven format, gopkg.in/yaml.v3 selected

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 13, §5 (`Load[T any](path string, opts ...Option) (T, error)`);
  ROADMAP 5.1 (opens Milestone 5); ADR-0004 (runtime dependency policy — the YAML-parser budget
  entry, "selected and pinned when Milestone 5 is implemented"); ADR-0005 (functional options,
  loud-by-default); compliance control C-1

## Context

Feature 13 is "load configuration from JSON/YAML or environment variables, supporting
validation", with the signature frozen at intake as the generic
`Load[T any](path string, opts ...Option) (T, error)`. Two decisions are open: how the loader
handles the three sources and validation, and — the review point ADR-0004 deferred to this PR —
**which YAML parser** to adopt as the module's first configuration runtime dependency.

## Decision

**Select `gopkg.in/yaml.v3` as the YAML parser.** ADR-0004 budgeted exactly one YAML runtime
dependency and named `gopkg.in/yaml.v3` "or its maintained successor, subject to review at that
PR". yaml.v3 is the de-facto standard, already present in the module as an indirect dependency
(via testify), so adopting it promotes an existing, `go.sum`-pinned, `govulncheck`-clean module
from indirect to direct — no new supply-chain surface. It stays within ADR-0004's ring 3 and
compliance control C-1; `go.mod`/`go.sum` review and the `govulncheck` gate remain its evidence.

**Format is chosen by file extension:** `.json` → `encoding/json`, `.yaml`/`.yml` → yaml.v3.
Anything else fails with the sentinel `ErrUnsupportedFormat` (wrapped, `errors.Is`-testable).
Extension dispatch is explicit and predictable; content sniffing was rejected as ambiguous.

**Generic decode straight into `T`.** `Load[T]` decodes into the consumer's own struct with no
intermediate `map[string]any`, returning the zero `T` alongside any error. The consumer tags
fields with the standard `json:`/`yaml:` tags it already knows.

**Environment variables via `${VAR}`/`$VAR` expansion**, on by default, applied to the file bytes
before parsing (`os.Expand` with `os.Getenv`; an unset variable expands to empty). This keeps
secrets and per-environment values out of the committed file — the common 12-factor pattern —
without a bespoke overlay mechanism. `WithoutEnvExpansion()` disables it for configs that
legitimately contain `$`. Direct typed environment reads are the separate concern of `env.GetDefault`
(5.2); field-level env **overlay by struct tag** is deliberately deferred (additive, non-breaking).

**Validation via a `Validator` interface.** If the decoded value implements `Validator`
(`Validate() error`), `Load` calls it (through `*T`, so a pointer-receiver method is found) and
wraps any error. An interface keeps validation type-safe and generic-friendly, avoiding a
generic-typed functional option; the richer tag-driven `validator.Struct` (feature 19, M8) composes
on top later by having `Validate` call it.

**Functional options** (`Option func(*options)`), the workerpool/pubsub idiom (ADR-0005), leave the
frozen signature stable while future options (strict-missing-env, overlay) arrive additively.

## Alternatives Considered

- **`github.com/goccy/go-yaml`** — faster and actively developed, but adds a genuinely new module
  where yaml.v3 is already vendored transitively and sufficient. Rejected for the zero-new-surface
  option; revisit only if yaml.v3 becomes unmaintained (ADR-0004's "maintained successor" clause).
- **Hand-rolled or JSON-only config** (no YAML dep) — honors strict-stdlib, but the maintainer chose
  the vetted-few posture at intake (ADR-0004) precisely so config can accept YAML. Rejected.
- **Field-level env overlay by struct tag** (`env:"ADDR"`) — the fuller 12-factor overlay, but a
  reflection-heavy surface overlapping M8's validator. Deferred as an additive option; `${VAR}`
  expansion covers the common need now.
- **A generic validation option `WithValidate(func(T) error)`** — flexible, but a generic-typed
  option complicates the non-generic `Option` type. Rejected for the `Validator` interface.
- **Content sniffing instead of extension** — tolerant, but ambiguous (JSON is valid YAML); a
  wrong guess is a confusing failure. Rejected for explicit extension dispatch.

## Consequences

- The public surface gains the `config` package: `Load[T]`, `Option`, `WithoutEnvExpansion`,
  `Validator`, `ErrUnsupportedFormat`. `gopkg.in/yaml.v3` becomes a direct runtime dependency.
- ADR-0004's deferred YAML-parser selection is now closed; control C-1's dependency ring is
  unchanged (yaml.v3 was already in the budget).
- Milestone 5 opens; `env.GetDefault` (5.2) provides the typed direct-env reads that complement
  `Load`'s file-plus-expansion model.
- Deferred surface (env overlay by tag, strict-missing-env, options-based validation) is additive.

## References

- `docs/specs/01_spec_utils.md` §2.13, §5.
- ADR-0004 (runtime dependency policy — YAML-parser budget), ADR-0005 (functional options).
- `docs/compliance/README.md` — control C-1.
- `gopkg.in/yaml.v3`, `encoding/json`, `os.Expand`.
