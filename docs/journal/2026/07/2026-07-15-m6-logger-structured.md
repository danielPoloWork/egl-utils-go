# 2026-07-15 — Milestone 6 opens: logger.Structured

## What got done

- **Roadmap 6.1 `logger.NewStructured`** (branch `feat/logger-structured`, draft PR #23, ADR-0019): the first of
  Milestone 6. Spec §5 froze `NewStructured(opts ...Option) *slog.Logger`. Returns a `*slog.Logger`
  backed by slog's **JSON handler** — one JSON object per line, the format ElasticSearch and Grafana
  Loki ingest directly — so it drops straight into `middleware.Logger` (4.2).
- **Four functional options** (ADR-0005 idiom): `WithWriter(io.Writer)` (default `os.Stdout`; nil
  ignored), `WithLevel(slog.Leveler)` (default Info; takes a `Leveler` so a `*slog.LevelVar` gives
  runtime-adjustable verbosity; nil ignored), `WithSource()` (off by default), `WithAttrs(...slog.Attr)`
  (base fields like service/version/env stamped on every record via `Handler.WithAttrs`). Safe,
  useful defaults with zero options: Info-and-above JSON to stdout.
- **slog's default keys kept** (`time`/`level`/`msg`) as the aggregator lingua franca — renaming to
  one backend's schema (ES `@timestamp`, etc.) would break others; a `WithReplaceAttr` escape hatch
  is deferred (additive). No new runtime dependency, no new pattern (functional options, row 2).
- Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, **100% logger
  coverage**, gofumpt clean, golangci-lint v2 (`/v2` path) 0 issues, govulncheck clean,
  `consistency_lint.py` OK. `-race` is CI-only locally (no cgo compiler).

## Where the project stands

M1–M5 complete and merged; **M6 in progress (1 of 2)**. 6.1 logger.Structured drafted on
`feat/logger-structured` (draft PR #23), awaiting the maintainer to open and merge (one PR at a time). README
milestone table: M6 → in progress. **Four completed milestones remain unreleased** (M2→v0.2.0 …
M5→v0.5.0); the maintainer has been deferring the release cut deliberately in favor of forward
progress.

## How the next session resumes

Wait for the 6.1 PR to merge. Then **6.2 `logger.Context`** — `WithFields(ctx, ...Field)
context.Context` + `FromContext(ctx) *slog.Logger` (spec §5 line 110), carrying per-request logger
fields through the context; the one subtle bit (per ROADMAP) is `slog.Handler` `WithAttrs`/`WithGroup`
propagation. Tier Sonnet 5 · high; likely its own ADR (the `Field` API + handler-wrapping design).
That completes Milestone 6. The portable Go toolchain under `%TEMP%\go-portable` remains the local
path; use the `/v2` golangci-lint module path; `-race` is CI-only locally.
