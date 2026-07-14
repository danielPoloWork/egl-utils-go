# ADR-0019: logger.Structured design — slog JSON handler, functional options, default keys kept

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 15, §5 (`NewStructured(opts ...Option) *slog.Logger`); ROADMAP 6.1
  (opens Milestone 6); ADR-0005 (functional options, loud-by-default); ADR-0014 (middleware.Logger
  consumes a `*slog.Logger`)

## Context

Feature 15 is "a JSON logger for easy ElasticSearch / Grafana Loki ingestion", frozen at intake as
`NewStructured(opts ...Option) *slog.Logger`. The return type ties the decision to `log/slog`
(already the module's logging substrate — ADR-0014's `middleware.Logger` takes a `*slog.Logger`).
The open questions are which handler to build, what "tuned for aggregation" means concretely, and
which knobs the options expose. No security or concurrency subtlety here; the decision is about the
public surface, so it is recorded per the ADR policy.

## Decision

**Build slog's `JSONHandler`.** One JSON object per line is exactly what ElasticSearch and Loki
ingest; the stdlib handler is battle-tested and zero-dependency. `NewStructured` returns
`slog.New(handler)` — an ordinary `*slog.Logger`, so it drops straight into `middleware.Logger` and
any slog-aware code.

**Keep slog's default field keys (`time`, `level`, `msg`).** These are the lingua franca aggregators
already understand; renaming them (`@timestamp`, `severity`, …) would chase one backend's convention
at another's expense. A consumer needing a specific schema applies its own `ReplaceAttr` — exposing
that is a deferred, additive option.

**Functional options** (ADR-0005 idiom), the four that matter for aggregation:
- `WithWriter(io.Writer)` — destination, default `os.Stdout` (the 12-factor sink a collector tails);
  a nil writer is ignored, keeping the default.
- `WithLevel(slog.Leveler)` — minimum level, default Info. Taking a `Leveler` (not a `Level`) lets a
  caller pass a `*slog.LevelVar` for runtime-adjustable verbosity; a nil level is ignored.
- `WithSource()` — add source file:line (`AddSource`), off by default for its runtime cost.
- `WithAttrs(...slog.Attr)` — base attributes stamped on every record via `Handler.WithAttrs`, the
  right place for stable identifiers (service, version, environment) an aggregator groups by.

Defaults are safe and useful with no options: Info-and-above JSON to stdout.

## Alternatives Considered

- **A hand-rolled JSON handler / custom logger type** — full control over the schema, but reinvents
  a solved stdlib problem and yields a non-slog type that `middleware.Logger` and consumers could not
  use directly. Rejected for `slog.JSONHandler` + the `*slog.Logger` return.
- **A third-party structured logger (zap, zerolog)** — faster in micro-benchmarks, but a new runtime
  dependency outside ADR-0004's budget for a need slog covers, and it would not satisfy the
  spec-frozen `*slog.Logger` return. Rejected.
- **Rename keys to a specific backend schema** (`@timestamp` for ES / ECS) — convenient for one
  target, wrong for others; locks the library to a house schema. Rejected; keep slog defaults, defer
  a `WithReplaceAttr` escape hatch.
- **A config struct instead of options** — the config package (5.1) uses a struct because its shape
  was spec-frozen; here the spec froze `opts ...Option`, so functional options it is, matching the
  workerpool/pubsub/config-loader idiom.

## Consequences

- The public surface gains the `logger` package's `NewStructured`, `Option`, and the four `With*`
  options; the return is a plain `*slog.Logger`.
- The structured logger and `middleware.Logger` compose directly: `middleware.Logger(logger.NewStructured(...))`.
- No new runtime dependency; no new pattern (functional options already catalogued, row 2).
- Deferred, additive surface: `WithReplaceAttr` (custom schema), a text-handler variant, sampling.
- `logger.Context` (6.2) builds on this — carrying per-request attrs through `context.Context` and
  handing back a `*slog.Logger` derived from one of these.

## References

- `docs/specs/01_spec_utils.md` §2.15, §5.
- ADR-0005 (functional options), ADR-0014 (middleware.Logger consumes `*slog.Logger`), ADR-0004
  (why no third-party logger dependency).
- `log/slog` — `JSONHandler`, `HandlerOptions`, `Leveler`/`LevelVar`.
