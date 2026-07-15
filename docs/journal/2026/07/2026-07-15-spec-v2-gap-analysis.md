# 2026-07-15 — Spec v2.0 discovered: gap analysis

## What got done

- **Discovered `.spec/` at the repo root** (untracked): spec **v2.0** of `d4np-go` ("Reviewed
  draft", 2026-07-14, resolving spec-review issue #8) plus three maintainer ADRs (001 logger-on-slog,
  002 wrap-x-packages, 003 zero-dep-core + contrib submodules). The repo was generated and built
  against the spec **frozen at intake** (`docs/specs/01_spec_utils.md`, from the v1 brief); v2 never
  entered the governed tree, so v1.0.0 shipped without ever reconciling it.
- **Wrote the gap analysis** at [`docs/specs/02_spec_v2_gap_analysis.md`](../../specs/02_spec_v2_gap_analysis.md)
  (maintainer's request: "a new file listing the differences and the gaps"). Two-frame verdict:
  **fully conformant to the frozen spec** (what v1.0.0 certifies); **substantial deltas vs v2.0** —
  all 25 items classified (✅ conformant / 🟢 convergent / 🟡 additive / 🟠 behavioral / 🔴 breaking).
- **Headline deltas:** 🔴 `errors` vs v2's `errx` (v2 names our exact design as the v1 mistake it
  corrects: shadowing + implicit stack capture); 🔴 metrics exposes `prometheus.Registerer` and the
  SDK (v2 ADR-003 forbids it — the text exposition format suffices); 🔴 `cache.Get (V, error)` vs
  `(V, bool)`; 🔴 `workerpool.Stop` vs `Close`; 🔴 pubsub API shape; 🔴 `WaitForSignals` without the
  bounded-deadline first argument (+ missing `Trigger()`); 🟠 bcrypt default cost 10 vs 12; 🟠
  proportional vs full jitter and verbatim vs attempt-wrapped last error; 🟡 no fuzz targets, only
  NFR-05 of six NFRs verified, no depguard/`go mod graph` CI, no `contrib/` submodules, no `State()`.
- **Notable finding:** v2 is internally contradictory on YAML — item 13 requires YAML config but
  ADR-003 allows stdlib+`x/` only, and no YAML parser exists there; our ADR-0004 two-dep budget was
  the pragmatic fix for that half. The prometheus half, by contrast, is a genuine divergence (v2
  shows the SDK is avoidable). Also: several v2 corrections we made **independently** (slog,
  ErrPasswordTooLong/no truncation, errors.Join in lifecycle, re-panic in db.Transaction, x/sync
  wrap, driver-free health probes).
- The document also maps **adoption paths**: what fits v1.x additively vs what needs a `/v2` major
  (v1.0.0's fresh SemVer commitment makes the 🔴 set breaking), vs deliberate deviations already
  argued in our ADRs that may simply be recorded. **No disposition decided** — the file is
  informational by request.

## Where the project stands

v1.0.0 shipped (tag pushed; GitHub Release draft awaiting the maintainer's Publish). No feature
work pending. The spec-v2 gap analysis is drafted on `docs/spec-v2-gap-analysis`, awaiting the
maintainer to open and merge. The open strategic question — which spec governs post-1.0 (adopt v2
deltas in v1.x/v2, or record deviations) — remains with the maintainer; the analysis is the
decision's input.

## How the next session resumes

If the maintainer picks a disposition: (a) v1.x additive adoption → new roadmap items (State(),
Trigger(), Middleware()/ErrLimited, configurable bcrypt cost, fuzzing, NFR bench suite, depguard,
contrib/); (b) `/v2` planning → new major roadmap; (c) deviations-recorded → one reconciliation ADR.
In all cases, first **import the v2 draft into `docs/specs/` under version control** (the gap doc's
§7 recommendation) so future spec revisions land inside governance. Portable Go under
`%TEMP%\go-portable`; `/v2` golangci-lint path; `-race` CI-only.
