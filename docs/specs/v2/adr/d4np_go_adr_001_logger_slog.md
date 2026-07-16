# ADR-001: Logger — build on stdlib `log/slog`, not a custom JSON logger

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-07-14 |
| **Related spec** | [d4np-go.md](../d4np-go.md) (§2 items 15–16, §2 item 10) |

## Context
v1 specified a custom JSON structured logger and never mentioned `log/slog`, which entered the standard library in Go 1.21 and is now the ecosystem's convergence point (handlers exist for every backend, and libraries increasingly accept `*slog.Logger`). A utilities library that ships its own logging *interface* forces an adapter onto every consumer already standardized on slog — the exact friction a utilities library exists to remove.

## Options considered

**A. Build on `log/slog`** *(chosen)*
- ✅ `logger.Structured` becomes configuration, not invention: a `slog.Handler` with opinionated JSON defaults (RFC 3339 UTC timestamps, `level`/`msg`/`source` keys tuned for Elastic/Loki ingestion), not a new API surface.
- ✅ `logger.Context` (attach fields via `context.Context`) implements the one thing slog deliberately left out — an idiomatic gap-filler rather than a competitor.
- ✅ Zero new logging dependency; consumers pass `*slog.Logger` everywhere, including into `middleware.Logger`.
- ❌ Requires Go ≥ 1.21 — already implied by the supported-versions policy (two most recent releases).
- ❌ slog's `Attr`-based API has known allocation costs in extreme hot paths; NFR-01's middleware allocation budget (≤ 3 allocs/op for the logging middleware) keeps this measured instead of assumed.

**B. Custom JSON logger (v1's implicit choice)**
- ✅ Full control over encoding performance (zerolog-style zero-alloc encoding).
- ❌ Reinvents an API the stdlib now owns; every consumer needs bridging to the slog ecosystem; maintenance of encoder correctness (escaping, UTF-8, time formats) lands on this library forever.

**C. Depend on zerolog/zap**
- ✅ Best-in-class raw performance.
- ❌ Violates the ADR-003 dependency policy (stdlib + `golang.org/x` only in core) and imposes a logging framework choice on consumers.

## Decision
**Option A.** `logger.Structured` returns a configured `*slog.Logger`; `logger.Context` stores/retrieves logger + accumulated attrs through `context.Context`; `middleware.Logger` accepts any `*slog.Logger`. No custom logging interface exists in the module.

## Consequences
- The library rides stdlib maintenance (new slog features arrive for free) and interoperates with every slog handler.
- If a profiling-proven hot path ever needs zero-alloc logging, the escape hatch is a custom `slog.Handler` — still inside the slog contract, no API break.
- Go < 1.21 support is explicitly out of scope (consistent with the supported-versions policy in spec §7).
