# ADR-0029: errors.Wrap design — %w-transparent wrapping, one-time origin stack, errors package name

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 25, §5 (`Wrap(err error, msg string) error`, `Wrapf(err, format,
  args...) error` — errors.Is/As/Unwrap compatible); ROADMAP 9.5 (completes Milestone 9 and the
  roadmap); ADR-0005 (loud-by-default)

## Context

Feature 25 is the final one: attach context and a traceable stack to an error "without losing the
original call stack". The two functions are frozen at intake and the spec's own wording requires
`errors.Is`/`As`/`Unwrap` compatibility. The open decisions: how the stack is captured and exposed,
what "one-time" capture means precisely, how a nil error is handled, and how to live with a package
literally named `errors`.

## Decision

**`%w`-transparent wrapping.** `Wrap`/`Wrapf` return a `*wrapped` whose `Unwrap()` returns the
cause, so `errors.Is`, `errors.As`, and `errors.Unwrap` all see straight through to the wrapped
error (and every error below it). `Error()` renders `"message: cause"`. This is the whole
compatibility contract, verified by tests against both a sentinel (`Is`) and a typed error (`As`).

**One captured stack per chain, at the origin.** The first time a chain *without* a stack is wrapped,
`runtime.Callers` records the call site; every later wrap **reuses that same stack by reference**
(via the `StackTracer` already present below it) rather than capturing a new one. So `runtime.Callers`
runs exactly once per chain and the recorded trace always points at the **original** failure site,
not at some outer re-wrap — which is what "without losing the original call stack" means. Each
`wrapped` stores the (inherited or freshly captured) origin stack, so `StackTrace()` is a trivial
accessor with no chain walking on the read path.

**The stack is exposed, not hidden.** A `StackTracer` interface (`StackTrace() []uintptr`) is
exported; a consumer reaches it with `errors.As` and expands the PCs with `runtime.CallersFrames`.
The `*wrapped` also implements `fmt.Formatter`: `%v`/`%s` print `"message: cause"`, `%+v` additionally
prints the stack frames (the familiar pkg/errors ergonomics), `%q` prints the quoted message. Returning
raw `[]uintptr` (rather than a bespoke frame type) keeps the surface minimal and composes directly
with the standard library.

**Wrapping nil returns nil.** `Wrap(nil, …)`/`Wrapf(nil, …)` return `nil` — a helper must not
manufacture an error out of the absence of one, so `if err := do(); err != nil { return errors.Wrap(err,
…) }` and an unconditional `return errors.Wrap(err, …)` both behave correctly.

**The package is named `errors` and imports the standard `errors` under an alias.** The spec froze
the package path `.../egl-utils-go/errors`, so the package name is `errors`. Internally it needs
`errors.As`, so it imports the standard library as `stderrors` — unambiguous, and the godoc tells a
consumer who needs both to alias one on import. No loud-nil surface here beyond the nil-returns-nil
rule; there is no misuse a panic would clarify.

## Alternatives Considered

- **Capture a stack on every wrap** — simpler code (no inheritance, no dead read-path branch), but
  each wrap would point at its own site and the *origin* trace would be one of several — losing the
  "original call stack" the spec asks for, and paying `runtime.Callers` on every wrap. Rejected for
  one-time origin capture (inherited by reference).
- **A bespoke `StackTrace`/`Frame` type (pkg/errors style)** — richer formatting out of the box, but
  a wider surface than the stdlib-composable `[]uintptr` + `runtime.CallersFrames`. Rejected for the
  minimal raw-PC form; `%+v` covers the common formatting need.
- **No `fmt.Formatter` (only `StackTrace()`)** — smaller, but forfeits the near-universal `%+v`
  stack-print idiom for no real saving. Kept the Formatter.
- **Return a non-nil error when wrapping nil** — matches a naive reading of "always wrap", but breaks
  the unconditional-return pattern and manufactures phantom errors. Rejected.
- **Rename the package to avoid shadowing (`errs`, `xerrors`)** — friendlier imports, but the package
  path/name is spec-frozen and `errors` is the honest name; the stdlib alias handles the collision
  cleanly. Rejected.

## Consequences

- Wrapping is fully `errors.Is`/`As`/`Unwrap`-transparent and carries a single origin stack, reachable
  via `errors.As` + `StackTracer` or printable with `%+v`.
- `runtime.Callers` runs once per chain; later wraps are allocation-light (message + reused stack ref).
- **Milestone 9 and the entire feature roadmap are complete — all 25 spec features are implemented.**
- Deferred, additive: a `Cause`/root accessor, a bespoke frame type, configurable stack depth.

## References

- `docs/specs/01_spec_utils.md` §2.25, §5 (error model — Is/As/Unwrap transparency).
- ADR-0005 (loud-by-default). `errors` (stdlib), `runtime.Callers`/`CallersFrames`, `fmt.Formatter`.
