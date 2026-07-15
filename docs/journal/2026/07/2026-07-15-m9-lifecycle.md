# 2026-07-15 — Milestone 9 opens: lifecycle.GracefulShutdown

## What got done

- **Roadmap 9.1 `lifecycle`** (branch `feat/lifecycle-shutdown`, draft PR #29, ADR-0025): signal-coordinated
  graceful shutdown — the first and hardest item of **Milestone 9, the final milestone** (Fable 5 ·
  xhigh per the ROADMAP tag). Spec §5 froze three package-level functions:
  `Register(fn func(ctx) error)`, `WaitForSignals(sig ...os.Signal)`, `Shutdown(ctx) error`; the
  spec's own §3 example fixes the call shape.
- **Ordering: LIFO (reverse registration).** Wiring order is dependency order (DB before the server
  using it), so shutdown is the reverse — stop traffic first, close storage last: `defer` intuition
  at process scope. Hooks run **sequentially** ("ordered" is the spec's word); a failing hook never
  skips the rest, and `Shutdown` returns the `errors.Join` of all failures. A cancelled context does
  not skip hooks either — each hook receives it and aborts on its own terms.
- **Exactly-once, convergent.** The first `Shutdown` runs the hooks; later or **concurrent** calls
  (a SIGTERM racing a programmatic shutdown — the realistic case) block until that run finishes and
  return its result. Mutex-guarded `started` flag + a `finished` channel whose close publishes the
  result (write-before-close = the happens-before edge). Register-after-shutdown and nil hooks
  panic (ADR-0005).
- **`WaitForSignals` blocks in place — zero owned goroutines** (spec §1). Subscribes the given
  signals (zero-arg default: `os.Interrupt` + `syscall.SIGTERM`; on Windows only Interrupt ever
  fires — documented), blocks, runs `Shutdown(context.Background())`, logs any joined error at
  Error on `slog.Default` (ADR-0016 wiring). **No hidden timeout**: the platform's kill escalation
  (systemd/K8s grace → SIGKILL) is the real bound; a consumer wanting its own uses a deadline
  context with `Shutdown` (ADR-0025 records the alternatives).
- **Testability of a spec-frozen singleton:** the state lives in an internal `coordinator` swapped
  per-test, and `signal.Notify`/`signal.Stop` are injectable seams — the fake delivers the signal
  synchronously, so WaitForSignals tests are deterministic **on Windows** (no `kill(2)` exists) and
  never start os/signal's process-wide goroutine (goleak stays exact). Tests cover LIFO order,
  context passthrough, cancelled-context behaviour, error joining, exactly-once, 8-caller
  convergence (the `-race` CI target), both panics, signal defaults, stop-on-exit, and the
  Error-level log on a failing shutdown. **100% coverage.**
- Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, 100% lifecycle
  coverage, gofumpt clean, golangci-lint v2 0 issues, `consistency_lint.py` OK. `-race` is CI-only
  locally (no cgo compiler) — the Linux race job gates the convergence path.

## Where the project stands

M1–M8 complete and merged; **M9 in progress (1 of 5)**. 9.1 lifecycle drafted on
`feat/lifecycle-shutdown` (draft PR #29), awaiting the maintainer to open and merge (one PR at a time). README
milestone table: M9 → in progress. **Seven completed milestones remain unreleased** (M2→v0.2.0 …
M8→v0.8.0); after M9 the library is feature-complete — the natural moment for the big release
decision (possibly v1.0.0).

## How the next session resumes

Wait for the 9.1 PR to merge. Then, in roadmap order: **9.2 `health.Handler`**
(`Handler(checks ...Check) http.Handler`, `Check{Name, Probe func(ctx) error}` — concurrent probes
with per-check timeouts; it is an HTTP surface, so it extends the threat model's public-edge rows;
Opus · medium), 9.3 `metrics.Prometheus` (**adds `prometheus/client_golang`, the last ADR-0004
ring-3 dependency — check its `go` directive before pinning, x/crypto lesson**), 9.4
`syncpool.BufferPool` (zero-alloc proof via `testing.AllocsPerRun`; patterns row: Object Pool
becomes Implemented), 9.5 `errors.Wrap`. Portable Go under `%TEMP%\go-portable`; `/v2`
golangci-lint path; `-race` CI-only.
