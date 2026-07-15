# Architecture Decision Records

One numbered Markdown file per decision, in the lightweight
[Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
format. Numbering is sequential and never reused or renumbered. Template:
[`template.md`](template.md).

Open an ADR when a choice affects the public surface or compatibility, when two reasonable
options exist and the rationale is non-obvious, when a **design pattern** is adopted, or
when superseding a prior decision. Do **not** open one for routine implementation details
or trivially reversible choices.

Status transitions: `Proposed` → `Accepted` → (`Superseded by ADR-XXXX` | `Deprecated`).

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](0002-adopt-cross-language-source-layout.md) | Adopt the cross-language source layout | Superseded by ADR-0003 |
| [0003](0003-adopt-idiomatic-go-root-layout.md) | Adopt the idiomatic Go root layout | Accepted |
| [0004](0004-runtime-dependency-policy.md) | Runtime dependency policy | Accepted |
| [0005](0005-workerpool-design.md) | workerpool design — bounded pool, blocking-first admission, loud panics | Accepted |
| [0006](0006-pubsub-design.md) | pubsub design — at-most-once buffered delivery, no broker goroutines | Accepted |
| [0007](0007-fanin-design.md) | fanin design — forwarder-per-input, cancel-or-drain contract | Accepted |
| [0008](0008-fanout-design.md) | fanout design — forwarder-per-output, exactly-once distribution | Accepted |
| [0009](0009-semaphore-design.md) | semaphore design — thin adapter over x/sync, first runtime dependency | Accepted |
| [0010](0010-circuitbreaker-design.md) | circuitbreaker design — lazy timerless transitions, generation-guarded accounting | Accepted |
| [0011](0011-retry-design.md) | retry design — proportional jitter, hard cap, last error verbatim | Accepted |
| [0012](0012-ratelimit-design.md) | ratelimit design — hand-rolled lazy token bucket, reservation-model Wait | Accepted |
| [0013](0013-middleware-requestid-design.md) | HTTP middleware foundation — Decorator chain and RequestID design | Accepted |
| [0014](0014-middleware-logger-design.md) | middleware.Logger design — ResponseWriter capture, status-derived levels, path-only logging | Accepted |
| [0015](0015-enterprise-governance-posture.md) | Enterprise governance posture — a raised compliance bar orthogonal to the domain | Accepted |
| [0016](0016-middleware-recoverer-design.md) | middleware.Recoverer design — panic-to-500, no stack to the client, ErrAbortHandler passthrough | Accepted |
| [0017](0017-middleware-cors-design.md) | middleware.Cors design — CorsConfig shape, deny-by-default, loud credential/wildcard guard | Accepted |
| [0018](0018-config-loader-design.md) | config.Loader design — generic Load, extension-driven format, gopkg.in/yaml.v3 selected | Accepted |
| [0019](0019-logger-structured-design.md) | logger.Structured design — slog JSON handler, functional options, default keys kept | Accepted |
| [0020](0020-logger-context-design.md) | logger.Context design — Field alias, accumulating context fields, slog.Default base | Accepted |
| [0021](0021-cache-inmemory-design.md) | cache.InMemory design — lazy expiry on Get, one sweeper goroutine, deterministic Close | Accepted |
| [0022](0022-db-transaction-design.md) | db.Transaction design — rollback on error and panic, re-panic, joined rollback errors | Accepted |
| [0023](0023-validator-struct-design.md) | validator.Struct design — reflection tag grammar, literal rules, panic on tag misuse | Accepted |
| [0024](0024-hash-password-design.md) | hash password design — bcrypt at default cost, per-hash salt, constant-time verify | Accepted |
| [0025](0025-lifecycle-shutdown-design.md) | lifecycle.GracefulShutdown design — LIFO hooks, exactly-once convergent Shutdown, no hidden timeout | Accepted |
