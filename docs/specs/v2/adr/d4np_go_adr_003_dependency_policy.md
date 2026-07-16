# ADR-003: Dependency policy — zero-dependency core; driver probes as nested contrib submodules

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-07-14 |
| **Related spec** | [d4np-go.md](../d4np-go.md) (§1, §2 items 22–23, §3) |

## Context
v1 grouped eight domains into one module with no dependency policy. Two items force the issue: the health endpoint (item 22) probes DB/Redis connections, and the metrics middleware (item 23) suggests the Prometheus SDK. In Go, a module's dependencies are viral: if the core module imports `go-redis` and `pgx`, **every** consumer — including one who only wants `workerpool` — pulls those into their `go.sum`, inflating audit surface, download size, and `govulncheck` noise. This is the most common failure mode of "utilities grab-bag" modules.

## Options considered

**A. Zero-dependency core (stdlib + `golang.org/x` only) + nested contrib submodules** *(chosen)*
- ✅ `go get github.com/danielpolowork/d4np-go` never transitively drags database drivers, Redis clients, or the Prometheus SDK.
- ✅ Nested modules (`contrib/redishealth/go.mod`, `contrib/pgxhealth/go.mod`) live in the same repo — one codebase, one CI — but version and resolve independently; only consumers who import a probe pay for its driver.
- ✅ `health.Handler` in core accepts a plain `func(ctx) error` probe interface; contrib packages just supply implementations. `metrics.Prometheus` emits the exposition text format directly — the format is stable and tiny; no SDK needed for counters/histograms of this scope.
- ❌ Nested modules add release friction (tags like `contrib/redishealth/v0.2.0`) and are a known Go tooling sharp edge. Accepted: the tag discipline is documented in the release runbook, and the alternative (viral deps) is worse.

**B. Single module, all dependencies in**
- ✅ Simplest repo mechanics.
- ❌ The viral-dependency problem in full; core SemVer becomes hostage to driver major-version churn (pgx v5→v6 would ripple into the utilities library's go.mod).

**C. Separate repositories for contrib**
- ✅ Cleanest module boundaries.
- ❌ Multiplies repos, CI pipelines, and issue trackers for what is a page of glue code each; discovery suffers.

## Decision
**Option A.** Core module policy: imports limited to stdlib and `golang.org/x/*`, enforced in CI by `depguard` (lint) plus a `go mod graph` assertion job. Driver-dependent probes ship as nested submodules under `contrib/`, each with its own `go.mod`, versioned independently.

## Consequences
- The §3 import-rules diagram is enforceable, not aspirational; a PR adding a third-party import to core fails lint before review.
- `govulncheck` results for core stay actionable (only stdlib/`x/` advisories).
- Adding a new probe (e.g., `contrib/mongohealth`) is a documented recipe: implement the core probe interface, own go.mod, own tag — no core release required.
