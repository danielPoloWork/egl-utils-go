# Session Journal

Dated end-of-session checkpoints — what got done, where the project stands, and how the
next session resumes. One file per session that changed the project's state, at
`docs/journal/<YYYY>/<MM>/<YYYY-MM-DD>-<short-slug>.md`. The journal is the dated trail;
`ROADMAP.md` is the forward plan — checkpoints never live inline in the roadmap.

At the close of a state-changing session, the agent:

1. Creates the dated file under `docs/journal/<YYYY>/<MM>/`.
2. Adds a link row to this index (newest first, grouped by year/month).
3. Updates the *Latest checkpoint* pointer in `ROADMAP.md`.

## Index

### 2026

_(newest first)_

#### 07 — July

- [2026-07-15 — M9.3: metrics.Prometheus](2026/07/2026-07-15-m9-metrics.md) — roadmap 9.3 (ADR-0027);
  request counter + latency histogram labelled (method, code) — no path label, method normalized
  (cardinality-DoS mitigation); adds prometheus/client_golang v1.23.2 (floor-preserving); one
  uncalled x/sys advisory kept to preserve the 1.24 floor.
- [2026-07-15 — M9.2: health.Handler](2026/07/2026-07-15-m9-health.md) — roadmap 9.2 (ADR-0026, PR #30);
  concurrent dependency probes on the request context, 200/503, status-only JSON body (never the
  probe error — info-disclosure); loud panic on empty/dup name or nil probe.
- [2026-07-15 — M9 opens: lifecycle.GracefulShutdown](2026/07/2026-07-15-m9-lifecycle.md) — roadmap
  9.1 (ADR-0025, PR #29); LIFO shutdown hooks, run-all + errors.Join, exactly-once convergent Shutdown, no
  hidden timeout, zero owned goroutines; injected signal seam for Windows-deterministic tests.
- [2026-07-15 — M8.2: hash (bcrypt) — Milestone 8 complete](2026/07/2026-07-15-m8-hash.md) — roadmap
  8.2 (ADR-0024, PR #28); bcrypt hashing/verify — default cost 10, per-hash salt, ErrPasswordTooLong,
  constant-time ErrMismatch; adds x/crypto v0.48.0 (floor-preserving); control C-4 + auditor sign-off.
  Milestone 8 complete.
- [2026-07-15 — M8 opens: validator.Struct](2026/07/2026-07-15-m8-validator.md) — roadmap 8.1
  (ADR-0023, PR #27); hand-rolled reflection validator (required/email/min/max/oneof), literal rules, nested
  recursion with dotted paths, full aggregation; data violations returned, tag misuse panics.
- [2026-07-15 — M7.2: db.Transaction — Milestone 7 complete](2026/07/2026-07-15-m7-db.md) — roadmap
  7.2 (ADR-0022, PR #26); auto-rollback transaction helper — commit on nil, rollback+return on error
  (errors.Join if rollback fails), rollback+re-panic on panic; fake sql driver in tests. M7 complete.
- [2026-07-15 — M7 opens: cache.InMemory](2026/07/2026-07-15-m7-cache.md) — roadmap 7.1
  (ADR-0021, PR #25); generic TTL cache — expiry enforced by Get (stale reads impossible), one sweeper
  goroutine with sync.Once Close (goleak-gated), fake-clock boundary tests, 0 allocs/op hot paths.
- [2026-07-15 — M6.2: logger.Context — Milestone 6 complete](2026/07/2026-07-15-m6-logger-context.md)
  — roadmap 6.2 (ADR-0020, PR #24); `WithFields`/`FromContext` carry accumulating logger fields through
  context (Field = slog.Attr alias), `FromContext` enriches slog.Default. Milestone 6 complete.
- [2026-07-15 — M6 opens: logger.Structured](2026/07/2026-07-15-m6-logger-structured.md) — roadmap
  6.1 (ADR-0019, PR #23); `NewStructured` returns a slog JSON-handler `*slog.Logger` tuned for ES/Loki, with
  WithWriter/WithLevel/WithSource/WithAttrs; composes with `middleware.Logger`.
- [2026-07-15 — M5.2: env.GetDefault — Milestone 5 complete](2026/07/2026-07-15-m5-env.md) —
  roadmap 5.2 (PR #22); typed env reads (`GetDefault`/`GetInt`/`GetBool`/`GetDuration`) with safe fallbacks;
  no ADR (routine). Milestone 5 complete.
- [2026-07-15 — M5 opens: config.Loader](2026/07/2026-07-15-m5-config.md) — roadmap 5.1
  (ADR-0018, PR #21); generic `Load[T]` for JSON/YAML with `${VAR}` env expansion and a `Validator` hook;
  selects + pins `gopkg.in/yaml.v3` (already indirect) under ADR-0004's budget.
- [2026-07-15 — M4.4: HTTP middleware (Cors) — Milestone 4 complete](2026/07/2026-07-15-m4-cors.md)
  — roadmap 4.4 (ADR-0017, PR #20); fourth/last M4 middleware — CORS preflight (terminal 204), deny-by-default
  origins, exact-origin echo + Vary, loud panic on the Fetch-forbidden credentials+`*` combo (new
  control C-3). Milestone 4 complete.
- [2026-07-15 — M4.3: HTTP middleware (Recoverer) + ADR-0015 backfill](2026/07/2026-07-15-m4-recoverer.md)
  — roadmap 4.3 (ADR-0016, PR #19); third HTTP middleware — panic→clean 500, no stack/panic leaked to
  the client (info-disclosure, C-2), server-side Error log, `http.ErrAbortHandler` passthrough;
  also backfills ADR-0015 (enterprise posture) to close the referenced-but-unwritten record.
- [2026-07-14 — M4.2: HTTP middleware (Logger)](2026/07/2026-07-14-m4-logger.md) — roadmap
  4.2 (ADR-0014, PR #18); second HTTP middleware — one structured `slog` line per request,
  Unwrap-aware status/bytes capture, status-derived levels, path-only logging (extends the
  threat model's Info-disclosure row + compliance C-2).
- [2026-07-14 — M4 opens: HTTP middleware (RequestID)](2026/07/2026-07-14-m4-middleware.md)
  — roadmap 4.1 (ADR-0013, PR #17); first HTTP middleware — adopts Decorator, crosses the
  first untrusted-input boundary (threat model + compliance C-2), `crypto/rand.Text` IDs.
- [2026-07-14 — M3 opens: circuitbreaker](2026/07/2026-07-14-m3-circuitbreaker.md) —
  roadmap 3.1 (ADR-0010, PR #14) + addenda 3.2 retry (ADR-0011, PR #15) and 3.3 ratelimit
  (ADR-0012, PR #16 — **Milestone 3 complete**, first benchmark report); healed the red
  master (2.6 go.sum handoff) via a portable Go toolchain; first local verification.
- [2026-07-12 — M1 bootstrap](2026/07/2026-07-12-m1-bootstrap.md) — Go module + quality
  configs; ADR-0003 (root layout) + ADR-0004 (dependency policy); Milestone 1 complete.
  Addenda 1–7 carry the whole of Milestone 2 (2.1–2.6, same-day sessions).
