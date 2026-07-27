# ADR-0045: Group feature packages under `pkg/` and open `/v2`

- **Status:** Accepted
- **Date:** 2026-07-27
- **Deciders:** Maintainer (Daniel Polo), with the architect agent
- **Related:** ADR-0003 (superseded), ADR-0030 (the `/v2` ledger), ADR-0040, ADR-0041, ADR-0042, ROADMAP 13.1

## Context

ADR-0003 put every feature package at the repository root, and for twelve milestones that was the
right call: it bought the short consumer import `…/egl-utils-go/workerpool` and it was the only shape
compatible with the short-import promise three governing documents already made.

The cost accumulated somewhere else. With twenty-one features shipped, the repository root holds
**forty entries** — twenty-one package directories, four other directories, and fifteen files — and
the maintainer's report was simply that the project had become hard to read. That is a real defect
even though nothing is broken: the root listing is the first thing a contributor sees, and ours
buries `README.md` among `fanin/`, `fanout/` and `syncpool/`.

The proposal on the table was the series' Maven tree, `src/main/go/it/d4np/utils/`. It was built and
measured before being rejected — see Alternatives — and the exercise produced the number that
decided this ADR: **the tree costs 34 characters per import, `pkg/` costs 7, and both declutter the
root identically.** The clutter was never caused by the packages being at the root; it was caused by
there being twenty-one of them. Any single parent directory fixes it. Only the depth differs.

Moving packages changes every import path, so this needs a major either way. [ADR-0042](0042-post-1.0-compatibility-contract.md)
made [ADR-0030](0030-spec-v2-reconciliation.md) §2's ledger the sole destination for breaking
changes, and [ADR-0041](0041-series-logical-namespace.md) recorded that deferred breakage is free at
a `/v2` boundary and only there. Seven items were waiting.

## Decision

Feature packages move from the repository root to **`pkg/<component>/`**, and the module opens its
second major: `go.mod` declares `module github.com/danielPoloWork/egl-utils-go/v2`.

```go
import "github.com/danielPoloWork/egl-utils-go/v2"            // package utils — Version
import "github.com/danielPoloWork/egl-utils-go/v2/pkg/cache"  // a feature package
```

Three parts of the shape are deliberate:

- **`/v2` is a module-path suffix, not a directory.** The major lives in `go.mod` and in the
  `v2.0.0` tag; the tree is not duplicated. v1 stays reachable at its own tags, which is what lets
  `contrib/*` keep requiring the released v1 until it migrates.
- **The module root keeps a package.** `doc.go`, `version.go` and `version_test.go` stay beside
  `go.mod`, so `…/v2` remains importable as `package utils` and pkg.go.dev has a landing page. They
  are module metadata, not features, and the one experiment that moved them into the tree left the
  module with no root package at all — legal, and worse.
- **`contrib/*` does not move and its tags do not change.** Those are separate modules versioning
  independently (ADR-0040), and their paths carry no major suffix of the core's.

This supersedes ADR-0003. **`/v2` also empties ADR-0030 §2's ledger in the same major** — all seven
deferred breaking items land in Milestone 13 alongside this move, because a boundary that is opened
and not used has to be opened again.

## Alternatives Considered

- **`src/main/go/it/d4np/utils/` — the series' Maven tree.** The maintainer's stated preference, and
  it was **built, verified green, and then reverted** rather than argued about: the move, the import
  rewrite across 45 files, the four policy tools, and depguard all passed. What the working version
  showed is that the consumer import runs to **86 characters** against `pkg/`'s 59 and v1's 52, for a
  root listing indistinguishable from `pkg/`'s. Rejected on that measurement. The series contract is
  the *logical* namespace `it.d4np.utils.<component>` (ADR-0041), which the module path already
  carries; rendering it a second time as directories buys nothing a consumer can use.
- **Keep the root layout (no major at all).** Cheapest — no import churn, no `/v2`, and v1.1.1 stays
  current. Rejected: it leaves the forty-entry root that prompted this, and it leaves the ledger
  where it has been since Milestone 10, which under ADR-0042 means those seven changes are simply
  never made.
- **Group by the concerns spec §4 already documents** (`concurrency/`, `resilience/`, `http/`, …).
  Attractive on paper and it would have matched the architecture section. Rejected on contact:
  two groups hold a single member whose name equals the group's, producing `config/config` and
  `lifecycle/lifecycle`.
- **A vanity module path** (`go.d4np.it/utils`), which ADR-0041 requires re-evaluating at exactly
  this boundary. Rejected again, and now on stronger evidence: with `pkg/` the import is already
  short, so the vanity path would buy only branding — in exchange for a domain that must resolve and
  a `go-import` tag that must be served for as long as anyone runs `go get`.

## Consequences

- **Every consumer import changes**, which is the definition of the major this ADR opens. The
  migration is mechanical: `…/egl-utils-go/<pkg>` → `…/egl-utils-go/v2/pkg/<pkg>`.
- **`contrib/redishealth` and `contrib/pgxhealth` stay on v1 until `v2.0.0` is tagged.** ADR-0040
  forbids a `replace` and requires the *released* core, so they cannot reference `/v2` before it
  exists. Their migration is a Milestone 13 item that must run **after** the tag — a sequencing
  constraint, not an oversight. Their own module paths and tag scheme are unaffected.
- **The four policy tools needed more than a path edit**, and the changes are worth naming because
  they encode the new shape:
  - `import_graph_lint.py` now separates `REPO` from `MODULE`. It built contrib module paths as
    `MODULE + "/contrib/" + name`, which with a major suffix would demand `…/v2/contrib/redishealth`
    — a module that does not exist.
  - A shared `short_pkg()` strips both the module path and `SRC_ROOT`, so `RUNTIME_DEPS`,
    `ALLOWED_INTERNAL_EDGES` and the coverage report stay written in short names (`cache`,
    `config`) instead of full import paths.
  - `consistency_lint.py`'s `src_main` becomes `pkg`; its `version_file` stays `version.go`,
    because the root package did not move.
- **depguard's `config-imports-only-validator` allowlist named a concrete path** and so rejected the
  one *sanctioned* edge after the move — a red build that looked like a violation and was a rule
  gone blind. Both depguard rules and `import_graph_lint.py` were re-verified by deliberate
  violation afterwards.
- **A verification note worth keeping.** The first violation test used a *blank* import and depguard
  reported nothing, which looked like a regression. It was not: ADR-0035 documents precisely that
  depguard cannot see `import _ "…/sibling"`, and that this is why `import_graph_lint.py` exists as
  a second layer — which caught it. Re-testing with a real import showed depguard firing correctly.
  The repository's own record answered the alarm.
- **The frozen v1 spec is amended, not replaced.** §4's layout and §5's import line describe v1 and
  are corrected under the document's own divergence rule with a dated Amendments entry; the spec
  stays one document. Each subsequent Milestone 13 item amends §5 again for its own API change, and
  `tools/spec_api_lint.py` makes that mechanical rather than remembered.
- The forty-entry root becomes nineteen.

## References

- ADR-0003 (superseded), ADR-0030 §2 (the ledger this major empties), ADR-0035 (why the blank-import
  gap exists), ADR-0040 (contrib versions independently), ADR-0041 (the logical namespace; the
  boundary rule), ADR-0042 (why this needs a major).
- Go modules reference — major version suffixes, `/v2` in the module path.
