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
