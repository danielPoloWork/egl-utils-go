# examples — runnable programs that compose the module

Each directory here is a **separate Go module** with its own `go.mod`. They are
documentation, not library code: nothing in the core imports them, and they are
never released or tagged.

| Module | Shows |
|---|---|
| [`examples/service`](service) | An HTTP service composed from eight packages: the middleware chain in the order it has to be composed, operational endpoints deliberately kept outside it, a readiness probe that reports something real, bounded background work, and an ordered shutdown on a signal. |

## Why these are separate modules

The same reason `contrib/*` is, arrived at from the other direction. There the
boundary keeps a driver dependency *out* of the core; here it keeps a showcase's
imports out — and the mechanism is identical, because it is the only mechanism
there is.

**A directory of `.go` files with no `go.mod` of its own silently joins the root
module.** Its imports become the core module's imports, its dependencies enter
`go.mod` on the next `go mod tidy`, and every `go build ./...`, `go list ./...`
and `go test ./...` at the repository root keeps passing — because from the
toolchain's point of view nothing is wrong. That is worse here than in `contrib`:
a contrib directory losing its `go.mod` at least breaks resolution on the driver
import, while everything an example service imports is *already* in the core's
graph, so the boundary can disappear with no error at all.

So it is asserted from the filesystem, on every CI run, by
[`tools/import_graph_lint.py`](../tools/import_graph_lint.py) — which also refuses
a `replace` directive, a committed `go.work`, and a core requirement pinned to
anything but a released version. The full reasoning is in
[ADR-0054](../docs/adr/0054-examples-service-module.md).

## Running one

```bash
cd examples/service
go run .
```

`./...` from the repository root does not descend into a nested module, so these
are built, tested and linted from their own directories. CI does the same, in an
`examples / <module>` job per module.

## What an example module may and may not do

- **It requires the core at a released version** — `require github.com/danielPoloWork/egl-utils-go/v2 vX.Y.Z`
  — with **no `replace` and no `go.work`**. An example is a consumer, and the
  point of it is to be built exactly the way a consumer builds it. A `replace`
  pointing at `../..` would be green in CI while leaving the `require` line that
  actually resolves for a reader completely unexercised.
- **It imports as many feature packages as its composition needs.** The core's
  own rule — feature packages do not import each other (spec §3, ADR-0033) — is
  about the *library's* internal graph. Composing many packages at once is what
  a consumer does, and showing that is the whole job.
  [`examples/.golangci.yml`](.golangci.yml) omits depguard for exactly this
  reason, the way [`contrib/.golangci.yml`](../contrib/.golangci.yml) does.
- **It carries tests, and they run in CI.** A program that only compiles proves
  nothing — the same trap
  [ADR-0053](../docs/adr/0053-runnable-examples-convention.md) closes for
  `Example` functions with its verified `// Output:` rule. Every claim an example
  makes about composition should be asserted by a test, so a rearrangement that
  breaks the advice fails the build instead of shipping.
- **It is not held to the per-package coverage floor.** Deliberately, and the
  reasoning is in [ADR-0054](../docs/adr/0054-examples-service-module.md) and in
  [`tools/coverage_gate.py`](../tools/coverage_gate.py): a `main()` that binds a
  port and blocks on a signal is not reachable from a test, and lowering the
  library's floor to accommodate a demo would weaken the gate that matters.

## Adding another example module

1. `examples/<name>/go.mod` declaring
   `github.com/danielPoloWork/egl-utils-go/examples/<name>` — the path must match
   the directory, which `import_graph_lint.py` checks. No major-version suffix:
   a nested module carries its own version line, not the core's.
2. Require the core at a released version. No `replace`, no `go.work`.
3. Add an `examples` matrix entry in
   [`.github/workflows/ci.yml`](../.github/workflows/ci.yml) and a `gomod` entry
   in [`.github/dependabot.yml`](../.github/dependabot.yml) — neither is
   discovered automatically. Ask the maintainer to add the new job to `master`'s
   required status checks; a job that is not required is a job a merge can skip.
4. The shared [`.golangci.yml`](.golangci.yml) in this directory applies.
5. Add a row to the table above, and give the module a `README.md` saying what it
   demonstrates and how to run it.
