# 2026-07-27 — Milestone 10.13: contrib/ submodules — **Milestone 10 complete**

## What got done

- **Roadmap 10.13** (branch `feat/contrib-submodules`): `contrib/redishealth` and
  `contrib/pgxhealth`, each a separate module supplying a `health.Check` probe so the core never
  imports a driver. **New [ADR-0040](../../../adr/0040-contrib-submodules.md)**. This closes
  Milestone 10.
- **The code is small; the decisions were all topological.** A `PING` and a pool `Ping` — the work
  was in how a nested module references the core, and in the fact that *nothing discovers a nested
  module automatically*.
- **Each submodule requires the released core (v1.0.0), with no `replace` and no `go.work`.**
  - A `replace ../..` is the common pattern and I rejected it because it tests the wrong thing:
    `replace` is ignored for anyone who *depends* on the module, so CI would validate a
    configuration no consumer ever gets, while the `require` line that actually resolves for them
    would never be exercised. Requiring the published core means CI builds contrib exactly as
    `go get` will.
  - A committed `go.work` is the modern answer and I rejected it for its effect on the *root*
    module: every root-level `go build`, `go vet`, `golangci-lint` and `govulncheck` would switch
    into workspace mode, changing behaviour for the eight CI jobs that currently work — a poor
    trade for a submodule that needs one frozen two-field struct. The accepted cost is that a core
    change breaking contrib is not caught until the core is released; acceptable because
    `health.Check` is frozen by the v1 commitment.
- **Exported constructors take the driver's own type; internals use a one-method interface.** The
  tempting alternative — export the narrow interface — would have removed pgx as a dependency
  altogether, at which point the module needs no `go.mod` and its reason for existing evaporates.
  A "driver glue" module that imports no driver is a contradiction, and a consumer already holding
  a `*pgxpool.Pool` should not have to write an adapter. Keeping the interface internal is what
  makes every branch testable to 100% without a live server; each module additionally constructs a
  **real** client and pool to prove the exported signature accepts the concrete types (neither
  dials eagerly, so no server is needed).
- **Verified the core is genuinely untouched**, rather than assuming it: root `go.mod`/`go.sum`
  unchanged, `go list ./...` returns no `contrib` package, and `go list -deps ./...` contains no
  `redis` or `jackc` path. A consumer of the core inherits nothing.

## The three things that silently ignore a nested module — all three closed

This was the substance of the item, and the reason the journal note for 10.13 said to check first.

- **`tools/import_graph_lint.py` now fails if a `contrib/*` directory holds Go files without a
  `go.mod`.** This is the dangerous failure and it is otherwise *silent*: those files simply join
  the root module, the driver enters the core's graph, and every other check keeps passing because
  they all ask `go list ./...` what the module contains. Verified by deleting the file — and that
  exposed a second problem: the later checks die first with an opaque "no required module provides
  package github.com/redis/go-redis/v9" instead of the real cause. So the contrib check now runs
  **first and short-circuits**, and the diagnostic names the actual problem. It also checks each
  module path matches its directory (a mismatch breaks `go get`).
- **`tools/coverage_gate.py` now measures the submodules** — 23 packages (21 core + 2 contrib),
  both new modules at 100%. Without this the gate would have silently stopped covering new code,
  which is precisely the failure ADR-0036 was written against.
- **CI grows a `contrib` matrix job** (build, vet, `-race` tests, gofumpt, golangci-lint,
  govulncheck per module), because every existing job runs `go test ./...` from the root and would
  never have built these modules at all. **Dependabot** grows an entry per submodule for the same
  reason — a nested module is invisible to the `/` entry, and these carry the driver dependencies
  most in need of watching.

## One interaction I had to discover by running it

The root's depguard rules **forbid what a contrib module exists to do**. Linting
`contrib/redishealth` with the root config in force reports the `go-redis` import *and* the
`health` import as violations — golangci-lint searches upward from its working directory, so it
finds the root config across the module boundary. Fixed with a single shared
`contrib/.golangci.yml` that keeps every other linter identical and omits only depguard, with the
reasoning in the file: what keeps the core clean is the module boundary, asserted by
`import_graph_lint.py`, not a lint rule applied to code that is supposed to import a driver.

That lint run also caught a genuinely dead field in one of my fakes, which I removed.

## Two toolchain problems fixed along the way

- **The portable Go's stdlib was incomplete.** `pgx` failed to build with
  `crypto/pbkdf2: package crypto/pbkdf2 is not in std` — the directory existed but was **empty**,
  another casualty of the same partial extraction that cost `pkg/include` and `src/runtime/cgo`
  during 10.7. Rather than patch package by package, I re-extracted the whole `go/src` tree from
  the archive (11 476 entries, ~9 s) and re-verified the repo still builds and tests.
- **`sum.golang.org` returned a 500** on the first fetch of the core module. Transient — the retry
  succeeded, and the repo is public. Worth noting only because the tempting "fix" is
  `GOSUMDB=off`, which would be a real supply-chain regression for a transient failure.

## Where the project stands

v1.0.0 shipped. **MILESTONE 10 IS COMPLETE (13 of 13)**: 10.1–10.12 merged (#37, #38, #46–#55);
10.13 drafted on `feat/contrib-submodules`, awaiting the maintainer to open and merge.

Local gauntlet green: build, `go vet ./...`, full `go test ./...`, gofumpt clean, golangci-lint v2
0 issues (root **and** both submodules), govulncheck 0 affecting, `consistency_lint.py`,
`import_graph_lint.py`, `coverage_gate.py` all OK. The core gained **no** dependency.

## How the next session resumes

Once 10.13 merges, Milestone 10 is done and the next act is **the v1.1.0 release cut** — the same
carry-through as v1.0.0, and note that **the contrib modules are not part of it** (they tag
separately, as `contrib/<name>/vX.Y.Z`, whenever they are first released):

1. Move the `[Unreleased]` entries into `docs/changelog/v1/v1.1.0.md` — **moved, not duplicated** —
   leaving `CHANGELOG.md` with the empty skeleton plus a new index row.
2. Bump `version.go` to 1.1.0, update the README status badge, write
   `docs/releases/v1.1.0.md` plus its index row.
3. `python tools/consistency_lint.py` verifies the lockstep it enforces: `version.go` ↔ README
   badge ↔ newest `docs/changelog/v1/` ↔ newest `docs/releases/`. Run all three policy tools.
4. The maintainer merges the release PR. **Then the agent tags**: `git tag -a v1.1.0` and
   `git push origin v1.1.0` → CI drafts the GitHub Release → the maintainer clicks Publish. Never
   tag before the merge, and never publish.

Worth putting in the release notes as known-and-deliberate, since both came out of 10.10 and are
still open for the maintainer to decide on:

- **NFR-01's 0-allocs/op target is unachievable** and is enforced as a ratchet budget at the
  measured floor instead (ADR-0037); the spec target needs amending.
- **`middleware.HeaderName` is `"X-Request-ID"`, not Go's canonical `"X-Request-Id"`**, which costs
  2 allocations per request in `CanonicalMIMEHeaderKey` for no wire-format difference. Measured but
  deliberately not changed: it is an API-visible constant under the v1 commitment.
