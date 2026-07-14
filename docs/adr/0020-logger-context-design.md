# ADR-0020: logger.Context design — Field alias, accumulating context fields, slog.Default base

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 16, §5 (`WithFields(ctx, ...Field) context.Context`,
  `FromContext(ctx) *slog.Logger`); ROADMAP 6.2 (completes Milestone 6); ADR-0013 (unexported
  context-key idiom); ADR-0016 (Recoverer's `slog.Default` precedent); ADR-0019 (logger.Structured)

## Context

Feature 16 is "attach logger fields to a `context.Context`", frozen at intake as
`WithFields(ctx, ...Field) context.Context` and `FromContext(ctx) *slog.Logger`. It completes
Milestone 6 alongside `logger.NewStructured` (6.1). The open decisions: what a `Field` is, how
fields behave when `WithFields` is called more than once down a call chain, and where the logger
`FromContext` returns gets its base configuration. The subtlety flagged in the ROADMAP is
field propagation — an inner scope must see the outer scope's fields.

## Decision

**`Field` is a type alias for `slog.Attr`.** `type Field = slog.Attr` means a value from slog
(`slog.String`, `slog.Group`, …) *is* a `Field` and vice versa — no conversion, no parallel type
system. Thin constructors (`String`, `Int`, `Bool`, `Duration`, `Any`) wrap the slog equivalents so
a caller can use `logger.String(...)` without importing slog, while power users pass any `slog.Attr`.

**Fields accumulate down the call chain.** `WithFields` reads the field slice already in the context
(if any), copies it, appends the new fields, and stores the result under an **unexported context
key** (`fieldsKey struct{}`, the ADR-0013 idiom — no collision with other packages). So a
request-id field set by an outer middleware is still present when an inner handler adds a
user-id field. The parent context's slice is never mutated (a fresh backing array is allocated), so
sibling call chains that branch from the same parent stay independent. `WithFields` with no fields
returns the context unchanged.

**`FromContext` derives from `slog.Default`.** The frozen signature takes no base logger, so
`FromContext` enriches `slog.Default()` with the accumulated fields (via `Logger.With`) and returns
it; with no fields it returns `slog.Default()` untouched. This mirrors Recoverer's use of the default
logger (ADR-0016) and gives a clean wiring story: a service sets
`slog.SetDefault(logger.NewStructured(...))` once (6.1), and every `FromContext` call then produces a
structured, context-enriched logger. No global state of our own; the consumer owns the base.

## Alternatives Considered

- **A distinct `Field` struct** (not a slog alias) — a house vocabulary independent of slog, but it
  would need converting at every slog boundary and duplicate slog's constructors for no gain. Rejected
  for the alias; the module is slog-native throughout (ADR-0014/0019).
- **Replace-on-`WithFields` (last call wins)** — simpler, but breaks the whole point of context
  propagation: an inner scope would lose the outer scope's request-scoped fields. Rejected for
  accumulation.
- **Mutate the existing slice in place** — avoids an allocation, but corrupts sibling contexts that
  share the parent's backing array, a classic aliasing bug. Rejected; copy-on-write.
- **`FromContext` takes or stores a base logger** — more explicit than `slog.Default`, but the spec
  froze the single-argument signature, and threading a base logger through the context duplicates what
  `slog.SetDefault` already provides. Rejected; use the default, document the wiring.
- **Store a `*slog.Logger` in the context instead of fields** — fewer moving parts, but bakes in the
  base logger at `WithFields` time and can't pick up a later `SetDefault`; storing plain data (fields)
  and resolving the logger lazily in `FromContext` is more flexible. Rejected.

## Consequences

- The public surface gains `Field`, the five constructors, `WithFields`, and `FromContext`; Milestone
  6 (structured logging) is complete.
- Context-scoped fields propagate correctly and immutably; `FromContext` composes with
  `NewStructured` through `slog.SetDefault`, closing the logging story: structured JSON base +
  per-request fields.
- No new runtime dependency; no new pattern.
- Deferred/additive: a variant taking an explicit base logger, or a `Fields(ctx) []Field` accessor,
  can arrive without breaking these signatures.

## References

- `docs/specs/01_spec_utils.md` §2.16, §5.
- ADR-0013 (unexported context-key idiom), ADR-0016 (`slog.Default` precedent), ADR-0019
  (logger.Structured — the base logger this pairs with).
- `log/slog` — `Attr`, `Logger.With`.
