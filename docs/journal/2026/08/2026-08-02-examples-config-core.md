# 2026-08-02 — 14.4: the driver that is not there, and the set that finishes

Twenty-nine examples across `config`, `env`, `cache`, `db`, `validator`, `hash`, `syncpool` and
`errx`, under [ADR-0053](../../adr/0053-runnable-examples-convention.md). That completes the set:
**55 examples across all 21 packages**, where a week ago there was one. Every exported surface a
consumer meets now has a call site that the toolchain executes.

The ADR named three traps for this item in advance. All three turned out to be real, and one of them
was a decision rather than an obstacle.

## `db`: the stub connector, and why ADR-0004 was never in the way

The roadmap put it as a fork — run the examples over a fake `driver.Connector`, or give `db`
prose-only godoc — and the fork dissolves once you notice that **`database/sql/driver` is standard
library**. A stub costs no dependency, no `go.sum` line and nothing for `import_graph_lint` to
object to. ADR-0004 forbids a *driver*; it does not forbid the interface every driver implements.
Against that, prose-only would have left the module's most safety-critical helper as the one thing a
reader cannot watch run.

What the stub buys is not "the example compiles". It counts commits and rollbacks, so the three
examples **assert the finalization contract** instead of describing it:

- commit path — `commits: 1 rollbacks: 0`
- error path — `commits: 0 rollbacks: 1`, with `errors.Is` proving the callback's own error comes
  back unwrapped
- panic path — `recovered: nil pointer in the row mapper` *and* `commits: 0 rollbacks: 1`

The third is the reason the helper exists. It is the case a hand-written `defer` gets wrong, and it
is now documentation that fails if the behaviour does.

The stub is deliberately **its own**, not the `fakeConn` that `db_test.go` has owned since ADR-0022.
Reuse would have saved thirty-five lines and put `sdb := newDB(conn)` in the rendered example body —
and pkg.go.dev renders the *function*, not the file, so a reader would meet an opaque helper with no
way to see what it was. `sql.OpenDB(&stubConnector{conn: conn})` says what it is on the line where it
appears, with the real call (`sql.Open("pgx", dsn)`) in the comment above it.

## `hash`: budgeted, not avoided

The instruction was "hash once, never in a loop". The whole package's example set does five bcrypt
operations and measures **0.74 s**, of which `ExampleHashPassword` is 0.67 s — a hash and two
verifications at the default cost 12, which is the register-then-login pair and the wrong-password
case, and there is no fourth operation to cut.

`ExampleCost` uses an explicit cost 10, and that is not a time-saving dodge: cost 10 is honestly what
a store written by an older deployment — or by v1 of this module — contains, so the upgrade example
needs a legacy hash to read the factor out of. It costs 0.07 s, which is the whole point of the
adaptive parameter and worth a reader seeing in the timings.

The other two examples are free. Both the 72-byte limit and the out-of-range cost are rejected before
any work happens, which is itself worth documenting.

## The gosec finding worth keeping

`golangci-lint` flagged `G101` on the `config` example's

```go
fmt.Println(cfg.DSN == "postgres://app:s3cret@localhost/app")
```

A `//nolint` would have been available and wrong. The example now prints

```go
fmt.Println(strings.Contains(cfg.DSN, "${"), strings.Contains(cfg.DSN, "s3cret"))
// false true
```

which is a *better* demonstration — it shows the two halves of what expansion did, the reference
gone and the environment's value in place — and leaves one less credential-shaped literal in a
documentation file. The same pattern as 14.3's revive finding: the linter pointed at a real thing,
and the fix improved the example rather than silencing the tool.

`config` also writes into a fresh `os.MkdirTemp` per example rather than a shared `os.TempDir()`
path, so the examples stay independent under `-shuffle` and under a parallel run of the package.

## Two rule-3 judgements, recorded because they look like violations

Rule 3 says print shape, not strings. Two examples print strings on purpose.

`errx.ExampleWrap` prints `handling GET /profile: loading user: not found`. Every word of that is the
example's own — the sentinel text and both wrap messages are declared three lines above — so the
assertion pins no `errx` wording and cannot be broken by a reviewer improving a message in the
package. The composed message *is* the documentation here; printing `err != nil` would document
nothing.

`errx.ExampleWithStack` asserts `strings.HasSuffix(frames[0].Function, "loadConfig")`. That proves
the property ADR-0046 made structural — the trace points at the capture site, and wrapping afterwards
cannot move it — without pinning a file path or a line number, both of which move.

## The one file that moved

`validator.ExampleStruct`, the module's only example a week ago, lived in `validator_test.go`. It is
now in `validator/example_test.go` with three siblings, so the pre-existing example finally sits
where the ADR it inspired says examples live.

## State

Milestone 14 is **4 of 12**. `master` is at `0f07922` (14.3, #87). Coverage: all eight packages were
**already at 100%** and stayed there — examples document, tests verify, and no example here stands in
for a missing test. 0 golangci-lint issues, gofumpt clean, whole module green including shuffled
runs, four policy tools green; no exported identifier, behaviour or dependency change.

Next: **14.5**, `examples/service` — the composition no package doc can show, as a module of its own,
with the trap 10.13 documented (a directory of `.go` files with no `go.mod` silently joins the root
module) and `import_graph_lint.py` extended to fail on it.

Still open for the maintainer: the two branch-protection flags
(`required_linear_history`, `required_conversation_resolution`).
