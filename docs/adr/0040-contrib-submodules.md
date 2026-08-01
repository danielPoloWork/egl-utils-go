# ADR-0040: `contrib/*` nested submodules — require the released core, no `replace`, no workspace

- **Status:** Accepted
- **Date:** 2026-07-27
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0003 (idiomatic Go root layout; nested submodules for driver-backed probes), ADR-0004 (dependency rings — which do not govern a contrib module the same way), ADR-0026 (`health.Handler`: concurrent probes, status-only body, per-probe timeout left to the probe), ADR-0035 (import-graph enforcement, extended here), ADR-0036 (the coverage gate, extended here), ADR-0030 (spec v2 reconciliation: 10.13 adopted), ADR-0005 (loud-by-default), spec v2.0 item 22 and §3, roadmap 10.13

## Context

Spec v2 item 22 requires that "DB/Redis probes live in **separate submodules**
(`contrib/…`) so the core never imports driver dependencies", and ADR-0003 already
provided for nested modules with independent tags. Roadmap 10.13 is the last item of
Milestone 10: `contrib/redishealth` and `contrib/pgxhealth`, each supplying a
`health.Check`.

The code is small — a `PING`, a pool `Ping`. The decisions are all topological.

**How does a submodule reference the core?** It needs `health.Check`, which lives in
the root module. A multi-module repository has three conventional answers — require a
published version, add a `replace` pointing at `../..`, or commit a `go.work` — and
they differ in what they test and in what they do to the *root* module's tooling.

**Nothing discovers a nested module automatically.** `go build ./...`,
`go list ./...`, `go test ./...`, Dependabot's `/` entry, and the repo's own policy
tools all stop at the module boundary. That boundary is exactly what keeps the core
clean, and it is also exactly what makes new code invisible to every gate.

**The core's lint rules forbid what a contrib module is for.** ADR-0035's depguard
rules deny driver and cache-client SDKs outright, and deny one feature package
importing another. A contrib module does both on purpose.

## Decision

Each `contrib/*` directory is a separate module that **requires the core at a released
version** (`v1.0.0` when this ADR was written; **`/v2 v2.0.0` since roadmap 13.9** — see the
amendment below) with **no `replace` directive and no `go.work`**; its exported
constructor takes the **driver's own type** while its internals are written against a
one-method interface so the probe is testable without a live server; and the three
places that would otherwise silently ignore a nested module are extended —
`tools/import_graph_lint.py` asserts every contrib directory *has* a `go.mod`,
`tools/coverage_gate.py` measures the submodules, and CI grows a `contrib` matrix job
— with a shared `contrib/.golangci.yml` shadowing the root's depguard rules.

## Alternatives Considered

- **A `replace github.com/danielPoloWork/egl-utils-go => ../..` in each submodule.**
  The most common pattern in multi-module repositories, and it builds contrib against
  the working tree so a core change is caught immediately. Rejected because it tests
  the wrong thing: `replace` is ignored for anyone who *depends* on the module, so CI
  would validate a configuration no consumer ever gets, and the `require` line — the
  one that actually resolves for consumers — would never be exercised. Requiring the
  published core means CI builds contrib exactly as `go get` will.
- **A committed `go.work` listing all three modules.** The modern answer, and it makes
  local development seamless. Rejected for its effect on the *root* module: with a
  `go.work` present every root-level `go build`, `go vet`, `golangci-lint` and
  `govulncheck` invocation switches into workspace mode, which changes resolution and
  tooling behaviour for the eight CI jobs that currently work. Paying that for a
  submodule that needs one frozen struct from the core is a poor trade, and a
  committed workspace also surprises anyone who clones the repo. (The cost of not
  having it: a core change that breaks contrib is not caught until the core is
  released. Acceptable here — contrib depends only on `health.Check`, a two-field
  struct frozen by the v1 API commitment.)
- **Vendoring `health.Check` into each contrib module** to remove the core dependency
  entirely. Rejected: the whole value is that these produce the *core's* type, so a
  consumer passes them straight to `health.Handler`. A structurally identical copy
  would not be the same type.
- **Exported constructors taking a narrow interface** (`interface{ Ping(ctx) error }`)
  rather than `*pgxpool.Pool` / `redis.UniversalClient`. Tempting, and for pgx it would
  remove the driver dependency altogether — at which point the module would need no
  `go.mod` of its own and the submodule's reason for existing would evaporate.
  Rejected on both counts: a consumer already holds the driver's type and should not
  write an adapter, and a "driver glue" module that imports no driver is a
  contradiction. The narrow interface is kept **internally**, which is what makes the
  probe testable without a server; the exported signature stays concrete.
- **Testing against real servers** (testcontainers, a CI service container). Rejected
  as far more machinery than these probes justify: the logic under test is "call Ping,
  wrap the error, apply a timeout", and a fake exercises every branch of it to 100%.
  Each module additionally constructs a *real* client and pool to prove the exported
  signature accepts the driver's concrete types — neither dials eagerly, so no server
  is required.
- **Leaving the policy tools and CI alone**, since the root's `./...` legitimately
  excludes contrib. Rejected as the exact failure ADR-0036 was written against: the
  coverage gate would have silently stopped covering new code, and CI would never have
  built these modules at all. A gate that ignores new code is worse than no gate,
  because it reads as green.
- **Applying the root's depguard rules to contrib** for uniformity. Impossible in the
  literal sense: verified that with the root config in force, linting
  `contrib/redishealth` reports the go-redis import *and* the `health` import as
  violations. A single shared `contrib/.golangci.yml` keeps every other linter
  identical and omits only the rules that encode core-module policy.
- **Duplicating that lint config per submodule.** Rejected as drift-prone;
  golangci-lint searches upward from its working directory, so one file at
  `contrib/.golangci.yml` covers both modules.

## Consequences

- **The core's dependency graph is provably unchanged.** Verified after adding both
  modules: the root `go.mod`/`go.sum` are untouched, `go list ./...` returns no
  `contrib` package, and `go list -deps ./...` contains no `redis` or `jackc` path.
  A consumer of the core inherits nothing.
- **The topology is now enforced, not merely intended.** `import_graph_lint.py` fails
  if a `contrib/*` directory holds Go files without a `go.mod`, or if its module path
  does not match its directory. The first is the dangerous one and it is silent
  otherwise: those files join the root module and the driver enters the core's graph
  while every `./...`-based check keeps passing. That check runs **first** and
  short-circuits, because without a `go.mod` the later checks die on an opaque
  "no required module provides package" error instead of the real problem — verified by
  removing the file.
- The coverage gate now measures **23 packages** (21 core + 2 contrib), both new
  modules at **100%**. CI grows a `contrib` matrix job running build, vet, `-race`
  tests, gofumpt, golangci-lint and govulncheck per module, and Dependabot grows an
  entry per submodule — a nested module is invisible to the `/` entry.
- **ADR-0004's rings apply to the core, not to contrib.** These modules exist to take a
  driver dependency; the ring policy is what confines that dependency *to them*. Their
  own dependencies are governed by the same spirit — one driver each, nothing else
  beyond the test ring — and by govulncheck in their CI job. Recorded so a future
  reader does not read the rings as forbidding what this ADR sanctions.
- Independent release: each module tags as `contrib/<name>/vX.Y.Z`, so a driver bump
  never forces a core release. `consistency_lint.py`'s version-lockstep check compares
  `version.go`, the README badge and the newest changelog/release files — all core
  artifacts — so it is unaffected by submodule tags. **Neither contrib module is
  released by Milestone 10's v1.1.0 tag**; their first tags are a separate, later act.
- Both probes honour the request context and add their own optional bound via
  `WithTimeout`, which is the per-probe timeout ADR-0026 deliberately left to the probe
  rather than putting a field on `health.Check`.
- A wiring error is loud (ADR-0005): an empty name, a nil client or pool, or a
  non-positive timeout panics at construction rather than failing on the first request.
  The nil check catches an untyped nil; a typed nil in an interface is not detectable
  that way and is documented as failing on the first probe instead.
- **Milestone 10 is complete.** The release carry-through is v1.1.0 for the core alone.

**Amendment 2026-07-30 (roadmap 13.9) — the released core is now `/v2 v2.0.0`.** Both submodules
now `require github.com/danielPoloWork/egl-utils-go/v2 v2.0.0` and import `…/v2/pkg/health`.
**Nothing in this ADR is superseded; it was exercised.** Three of its decisions did real work here,
and it is worth recording which:

- **"Require the *released* core" set the schedule of an entire milestone.** Every other `/v2` item
  shipped inside the major; this one could not, because the version it must name did not exist until
  the core was tagged. That is why the ROADMAP orders it *after* the release (13.9 runs after 13.10)
  and why an attempt to do it earlier fails concretely rather than stylistically:
  `go list -m …/egl-utils-go/v2@v2.0.0` reported **"unknown revision"** while the published list
  stopped at v1.1.1.
- **The rejected `replace` would have hidden exactly this.** With `replace ../..` the migration could
  have been done at any point during the major and CI would have been green throughout — while the
  `require` line consumers resolve went unexercised. The constraint that made this item wait is the
  same one that makes it *verified* when it lands.
- **The per-module major-version suffix rule.** A contrib path carries its own major, not the core's,
  so `contrib/redishealth` keeps its path while depending on the core's v2 — the module paths and tag
  scheme are untouched, as this ADR anticipated.

One measured side effect, recorded because it is invisible in the diff: MVS raised `pgxhealth`'s
transitive `golang.org/x/sync` pin from **v0.17.0 to v0.22.0**, because the core v2 requires the
newer one. A major bump in a dependency moves a submodule's indirect graph even when the submodule's
own code does not change.

Their first `/v2`-based tags are, as before, **a separate act**: this amendment migrates the source,
it does not release it.

## References

- Spec v2.0 item 22, §3; ADR-0003 (nested submodules, independent tags), ADR-0026
  (`health.Handler` contract and the deferred per-probe timeout), ADR-0035 and ADR-0036
  (the two gates extended here), ADR-0030 (adoption bucket).
- `contrib/README.md` (topology and how to add another), `contrib/.golangci.yml`,
  `contrib/redishealth/`, `contrib/pgxhealth/`, `tools/import_graph_lint.py`,
  `tools/coverage_gate.py`, the `contrib` job in `.github/workflows/ci.yml`.
- Go's nested-module tagging convention (`<subdir>/vX.Y.Z`); `replace` semantics for
  dependent modules.
