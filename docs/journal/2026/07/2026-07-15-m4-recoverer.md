# 2026-07-15 — Milestone 4.3: HTTP middleware (Recoverer) + ADR-0015 backfill

## What got done

- **Roadmap 4.3 `middleware.Recoverer`** (branch `feat/middleware-recoverer`, ADR-0016,
  patterns row 9): the third HTTP middleware. Spec §5 unconfigured decorator
  `Recoverer(next http.Handler) http.Handler`. Runs `next` under a deferred `recover()`;
  on a panic it writes a **generic `500 Internal Server Error`** and contains the failure.
- **Security decisions (why it carries an ADR under the enterprise posture):**
  - **No stack or panic value ever reaches the client** — a leaked stack exposes source
    paths/symbols/structure and a leaked value can carry secrets in flight; the client sees
    only the generic status text. The detail is logged **server-side** at Error on
    `slog.Default` (method, path, panic value, `runtime/debug.Stack`, `request_id` when the
    chain seeded one). Path only — never the query string (same rule as Logger, ADR-0014).
  - **`http.ErrAbortHandler` is re-panicked unchanged** — it is net/http's silent-abort
    sentinel; swallowing it would mislabel an intentional abort as a 500. Not logged, not a 500.
  - **An already-committed response is left intact** — detected via the Logger
    `responseRecorder` (ADR-0014), which tracks `wroteHeader` and exposes `Unwrap` so
    `http.ResponseController` still reaches the underlying Flusher/Hijacker. The 500 is written
    only when nothing was committed; the panic is logged either way. **Reuses** the existing
    recorder rather than adding a second wrapper.
  - Recommended chain `RequestID → Logger → Recoverer → handler` (Recoverer innermost, so the
    recovered 500 is the status Logger observes). Loud nil (ADR-0013).
- **Backfilled ADR-0015 (enterprise governance posture).** The posture (init-phase Q0.5,
  `governance.posture: enterprise`) was referenced by name as "ADR-0015" in five artifacts
  (AGENTS.md §3, compliance README, docs/README, project.yaml, consistency_lint.py) but its
  record was never written. `consistency_lint.py` enforces gap-free sequential ADR numbering,
  so the next physical ADR had to be 0015 — and 0015 was semantically reserved for the posture.
  Maintainer chose (batched question) to **backfill 0015** (records the already-made decision,
  no new choice) and number **Recoverer 0016**. Both land in this PR; index gap-free through 0016.
- Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, **100% middleware
  coverage**, gofumpt clean, golangci-lint v2 (`/v2` module path) 0 issues, govulncheck clean,
  `consistency_lint.py` OK. `-race` is CI-only locally (no cgo compiler).

## Where the project stands

M1–M3 complete and merged; M4 in progress (3 of 4 — 4.1 RequestID #17, 4.2 Logger #18 merged).
4.3 Recoverer drafted on `feat/middleware-recoverer`, awaiting the maintainer to open and merge
(one PR at a time). **Two completed milestones remain unreleased** — M2 (v0.2.0) and M3 (v0.3.0),
one MINOR each per §11; still open with the maintainer whether to cut separately, combined, or defer.

## How the next session resumes

Wait for the 4.3 PR to merge. Then either address the release question (v0.2.0 / v0.3.0) or finish
Milestone 4 with **4.4 middleware.Cors** — `Cors(cfg) func(http.Handler) http.Handler` (constructor
form, configured; spec §5). Cors is the CORS-preflight edge-case item flagged in the M4 guidance
(OPTIONS handling, origin allow-listing, header reflection) and will carry its own ADR. The portable
Go toolchain under `%TEMP%\go-portable` remains the local path; use the `/v2` golangci-lint module path.
