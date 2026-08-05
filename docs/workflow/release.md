# Release Process

The mechanical step-by-step for cutting a release of `egl-utils-go`. The governance
(which SemVer level, how a fix flows, deprecation/security) is in
[`maintenance.md`](maintenance.md); the agent-vs-human boundary is
[`AGENTS.md`](../../AGENTS.md) §11.

## Versioning

**Semantic Versioning 2.0.0**, annotated tags `vMAJOR.MINOR.PATCH`. Start point:
pre-1.0 milestone-driven.

- Pre-1.0: `MINOR` bumps on each completed roadmap milestone; `PATCH` for hotfixes.
- Post-1.0: `MAJOR` for incompatible changes, `MINOR` for additions, `PATCH` for fixes.

## Cutting a release (the steps)

1. **Bump the version constant** (const Version = "X.Y.Z") in `version.go`; update any
   version-check test.
2. **Roll the changelog** — move the `[Unreleased]` entries into a new per-version file
   `docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and add an index row to `CHANGELOG.md`.
3. **Refresh the README** status badge (and milestone table on a MINOR that closes a
   milestone).
4. **Draft release notes** under `docs/releases/v<X.Y.Z>.md`.
5. **Run the consistency lint** (`python tools/consistency_lint.py`) — version lockstep must
   pass.
6. **Open the release PR** — *the maintainer does this*. The agent prepares it.
7. **Merge** — *the maintainer*.
8. **Tag + draft (carry-through)** — the agent runs `git tag -a v<X.Y.Z> -m "<headline>"` and
   `git push origin v<X.Y.Z>` immediately after merge; the tag push lets CI open the GitHub Release
   as a **draft**. The agent always carries the release this far — only **Publish** is the human's.
9. **Publish** the GitHub Release — *the maintainer* (the deliberate human checkpoint).
10. **CI builds & attaches artifacts** on the tag push.


## Releasing a `contrib/*` submodule

Each `contrib/*` directory is a module of its own with its own tags ([ADR-0040](../adr/0040-contrib-submodules.md)),
so releasing one is a **separate act** from releasing the core: a driver bump never forces a core
release, and a core release never publishes a submodule.

- **Tag scheme:** `contrib/<name>/vX.Y.Z` — the directory path, then the version, per Go's
  nested-module convention. That tag is what the proxy serves as the module's version.
- **The version is the submodule's own.** It tracks that module's API and its driver, not the core's:
  `contrib/redishealth/v0.2.0` requires the core's `/v2` while carrying no major suffix itself,
  because a module path carries its own major.
- **`release.yml` does not fire, and does not need to.** It triggers on `tags: ["v*.*.*"]`, which a
  `contrib/…` ref does not match. `contrib-release.yml` covers that space instead
  ([ADR-0055](../adr/0055-contrib-release-workflow.md)), firing on `tags: ["contrib/*/v*.*.*"]` — so
  a submodule tag is now verified mechanically rather than by hand. It checks, and fails loudly on
  each:
  1. the tag names **one** directory under `contrib/`, followed by a SemVer version;
  2. that directory **has a `go.mod`** — a tag naming a directory that is not a module fails instead
     of falling back to the root module and reporting a green release for the core;
  3. the `go.mod`'s `module` line is exactly `github.com/danielPoloWork/egl-utils-go/<dir>`, with the
     **`/vN` suffix** Go requires from `v2` upward;
  4. the tagged commit is **reachable from `master`**, which is how the tag inherits the
     `contrib / <module>` required status check;
  5. `go build`, `go vet`, `go test -race` and `go mod verify` from the module directory.
- **A red run means: delete the tag, fix, re-tag.** Verification necessarily happens after the push,
  so that is the remedy, and it is legal because nothing has been published (see the boundary rule at
  the bottom of this page). The run's summary records the module path, version and commit that were
  verified.
- **No GitHub Release is published for a submodule**, and the workflow deliberately drafts none
  (ADR-0055 (b)): the annotated tag message is the record, the proxy is the distribution, and keeping
  the tag unpublished is what keeps delete-and-repush available. Every `contrib/*` tag so far follows
  this.
- **The core's release artifacts are untouched.** `version.go`, the README badge, `CHANGELOG.md`,
  `docs/changelog/` and `docs/releases/` are core-only, and `consistency_lint.py`'s version-lockstep
  check compares just those — a submodule tag can neither satisfy nor break it. `spec_api_lint.py`
  likewise holds `contrib/*` outside the versioned surface.
- **Verify what a consumer resolves** once the tag is pushed:
  `go list -m github.com/danielPoloWork/egl-utils-go/contrib/<name>@<version>`. The proxy caches its
  version *list* for a few minutes, so `@latest` can lag an explicit version briefly.

The agent tags and pushes; the version choice is the maintainer's, and there is no Publish step to
reach. Precedent from v2.0.0: both submodules went to **v0.2.0** — a minor bump inside `v0`, because
moving to the core's `/v2` changed the `health.Check` type they return (breaking for consumers) while
their own identifiers did not change, and `v0` still commits to no stability on an API pinned to a
driver major.


## Boundary

| Action | Who |
|---|---|
| Bump version, roll changelog, draft notes | Agent |
| Open / merge the release PR | **Human** |
| Create & push the annotated tag, then the **draft** release (CI drafts it on tag-push) | Agent |
| Publish the GitHub Release (click **Publish**) | **Human** |
| Build & attach artifacts | CI |
| Choose a `contrib/*` submodule version | **Human** |
| Create & push a `contrib/<name>/vX.Y.Z` tag (no Release is drafted) | Agent |
| Verify a `contrib/*` tag — parse, module identity, reachability, build/vet/race/verify | CI (`contrib-release.yml`) |


Agents never publish releases, never amend or delete published tags, and only delete-and-
repush an *unpublished* tag whose release run visibly failed.
