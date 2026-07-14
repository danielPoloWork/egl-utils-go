# 2026-07-14 — Milestone 4 opens: HTTP middleware (RequestID)

## What got done

- **Roadmap 4.1 `middleware.RequestID`** (branch `feat/middleware-requestid`, draft PR #17,
  ADR-0013, patterns row 9): the first HTTP middleware and the first of Milestone 4. Stands
  up the `middleware` package (`doc.go` + `requestid.go`) with the spec §5 surface —
  `RequestID(next http.Handler) http.Handler` and `RequestIDFrom(ctx) string`. Adopts a
  valid inbound `X-Request-ID`, else generates one with **`crypto/rand.Text`** (Go 1.24,
  dependency-free, ≥128-bit base32); stores it in the request context under an unexported
  key type; echoes the resolved ID in the response header. 100% statement coverage.
- **First HTTP trust boundary handled, not deferred.** RequestID reads a client-supplied
  header — the module's first untrusted input. Inbound IDs are sanitized (visible-ASCII
  `0x21–0x7e`, ≤128 bytes) so CR/LF/control bytes can never reach logs or the reflected
  header (log/header-injection). Because this is security-relevant under the enterprise
  posture, the PR carries three artifacts beyond the code: **ADR-0013** (also the
  Decorator-pattern adoption record), the **threat model**'s first real content (public
  HTTP edge boundary + a full STRIDE pass scoped to RequestID), and **compliance control
  C-2** (untrusted HTTP input handling).
- **Package conventions set for all of M4** (ADR-0013): decorator shape
  `func(http.Handler) http.Handler` (direct where unconfigured — RequestID, Recoverer;
  constructor where configured — Logger, Cors, per spec §5); unexported context-key types
  with exported accessors; loud panic on a nil handler (ADR-0005 lineage). Logger (4.2),
  Recoverer (4.3), Cors (4.4) inherit these.
- Local gauntlet green (portable Go 1.26.5): build, vet, full `go test ./...`, gofumpt,
  100% middleware coverage. `crypto/rand.Text` confirmed present in the 1.24 floor before use.

## Where the project stands

M1–M3 complete and merged; M4 in progress (1 of 4). PR #17 (draft) awaits review — note it
is the first PR to touch the security surface, so the reviewer should apply the
security-auditor sign-off the enterprise bar asks for (AGENTS.md §10). **Two completed
milestones remain unreleased** — M2 (v0.2.0) and M3 (v0.3.0), one MINOR each per §11; the
question is still open with the maintainer whether to cut them separately, combined, or
defer.

## How the next session resumes

Wait for PR #17 to merge (one PR at a time). Then either address the standing release
question (v0.2.0 / v0.3.0) or continue Milestone 4 with **4.2 middleware.Logger**
(`Logger(l *slog.Logger) func(http.Handler) http.Handler` — logs method, path, status,
duration, bytes; roadmap tier Opus 4.8 · high). 4.2 will consume the request ID this item
put in context, and will extend the threat model's Info-disclosure row (what gets logged).
The portable Go toolchain under `%TEMP%\go-portable` remains the local-verification path.
