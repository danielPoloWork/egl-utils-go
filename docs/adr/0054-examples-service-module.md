# ADR-0054: `examples/service` — a composition showcase as its own module, and no coverage floor on it

- **Status:** Accepted
- **Date:** 2026-08-04
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** roadmap 14.5 (this item); [ADR-0053](0053-runnable-examples-convention.md) (the
  package-level `Example` convention, which named this module as its complement);
  [ADR-0040](0040-contrib-submodules.md) (the nested-module topology this repeats and now
  enforces); [ADR-0003](0003-adopt-idiomatic-go-root-layout.md) and
  [ADR-0045](0045-pkg-layout-and-v2.md) (layout, and nested modules with independent tags);
  [ADR-0035](0035-import-graph-enforcement.md) and [ADR-0036](0036-coverage-gate.md) (the two
  gates extended here); [ADR-0004](0004-runtime-dependency-policy.md) (the dependency rings this
  module makes visible); [ADR-0051](0051-lifecycle-shutdown-timeout.md) (the shutdown deadline the
  service configures); [ADR-0026](0026-health-handler-design.md) (the probe contract the readiness
  endpoint honours); [ADR-0031](0031-ratelimit-middleware-design.md) (the 429 path kept off the
  operational endpoints)

## Context

ADR-0053 gave every package runnable `Example` functions — 55 of them across 21 packages — and
recorded, in its own alternatives, the thing they cannot do:

> **An `examples/` directory of runnable programs instead of `Example` functions** — rejected *as a
> substitute*: a program in a directory is attached to no identifier, so pkg.go.dev shows nothing
> beside `Merge`, and nothing executes it in CI. It is a real complement, though, which is why 14.5
> adds one as its own module for the cross-package composition no single package doc can show.

That is the gap. An `Example` is attached to one identifier and is therefore, structurally, about one
package. But a substantial share of the mistakes a consumer of this module can make are **decisions
about two packages at once**, and each of them is correct or wrong only in composition:

- `middleware.Logger` is documented; whether it belongs outside or inside `ratelimit.Middleware` is
  not, and the answer decides whether shed traffic is visible at all;
- `health.Handler` is documented; that it should be mounted **twice**, once with no checks for
  liveness and once with the dependency probes for readiness, is a wiring decision that appears in no
  package's godoc — and getting it wrong turns a dependency blip into a restart storm;
- `metrics.Recorder.Middleware` is documented; that the scrape endpoint must not be behind it is
  arithmetic about the consumer's deployment, not about the package;
- `lifecycle.Register` documents LIFO; *which* order that has to be for a listener and a worker pool
  is a fact about the pair.

There is also a hazard specific to *where* such a showcase lives, and this repository has already
paid for the lesson once. ADR-0040 §Consequences records it for `contrib`:

> Delete or rename one of those `go.mod` files and the failure is silent: the `.go` files underneath
> simply join the root module, the driver joins the core's dependency graph, and every other check
> here keeps passing.

A composition example is the same shape of risk with a worse failure mode, and that was **verified
rather than assumed**. With `examples/service/go.mod` moved aside:

- `go list ./...` from the repository root reports the package — it is part of the core module;
- and `go list -deps ./...` **still succeeds**, because everything this service imports is already in
  the core's own graph.

So for `examples/*` the boundary can disappear with **no error of any kind**. In `contrib` the driver
import at least breaks resolution; here there is nothing to break, and the next `go mod tidy` would
quietly write whatever a future example acquired into the library's `go.mod`.

## Decision

`examples/service` is a **separate Go module** at
`github.com/danielPoloWork/egl-utils-go/examples/service`, requiring the core at a **released**
version (`/v2 v2.0.0`) with **no `replace` and no `go.work`** — ADR-0040's reasoning applied a second
time, unchanged. It composes **eight** packages into a service skeleton (`env`, `logger`,
`middleware`, `ratelimit`, `metrics`, `health`, `workerpool`, `lifecycle`) rather than all
twenty-one; **carries tests that assert every composition claim its comments make**, stdlib-only; and
is **deliberately excluded from the per-package coverage gate**, which is stated here as a decision
with its measurement rather than left as an absence.

Four mechanisms make the topology enforced rather than intended:

1. **`tools/import_graph_lint.py`'s contrib check is generalized** over a
   `NESTED_MODULE_PARENTS` map covering `contrib` and `examples`, still running **first and
   short-circuiting** for ADR-0040's original reason. It now also walks **recursively**: a
   directory whose only Go files sit in `cmd/server/` with no `go.mod` above them is the same
   failure one level deeper, and the previous top-level-listing check would have missed it.
2. **The same tool now refuses a `replace`, a committed `go.work`, and a core requirement pinned to
   a pseudo-version** — for every nested module, so ADR-0040's "no `replace`, no workspace" stops
   being prose it had been since 2026-07-27 and becomes a gate.
3. **`examples/.golangci.yml` shadows the root config, minus depguard**, exactly as
   `contrib/.golangci.yml` does and for a structurally identical reason.
4. **A new `examples / <module>` CI job** builds, vets, runs `go test -race`, checks
   `go mod tidy -diff` and `go mod verify`, and runs gofumpt, golangci-lint and govulncheck.

## Alternatives Considered

- **A directory of `.go` files inside the root module** — no `go.mod`, the smallest possible diff,
  and `go run ./examples/service` would work immediately. Rejected on the verification above: the
  root module would own the showcase, and because nothing about that is an error, the only signal
  would arrive later as an unexplained line in the library's `go.mod`. The whole point of a
  showcase is that it is free to grow an import; the module boundary is what makes that free.
- **A `replace github.com/danielPoloWork/egl-utils-go/v2 => ../..`** — the common pattern, and it
  would build the demo against the working tree so a core change breaks it immediately. Rejected on
  ADR-0040's grounds, which apply more sharply here: an example's job is to be built the way a
  reader builds it, and `replace` is ignored for anyone who depends on the module. It would also
  hide the one thing this module proves — that the `require` line resolves. The accepted cost is
  the same one ADR-0040 accepted: a core change that breaks the example is not caught until the
  core is released, and Dependabot's bump is where it surfaces.
- **A committed `go.work`** — rejected verbatim on ADR-0040's reasoning: it switches every
  root-level `go`, `golangci-lint` and `govulncheck` invocation into workspace mode, changing
  resolution for jobs that currently pass, to serve a module that needs one released dependency.
- **Extending the existing `contrib` CI matrix with an `examples/service` entry** instead of adding
  a job — tempting, since the steps are nearly identical. Rejected for a reason outside the YAML:
  `contrib / redishealth` and `contrib / pgxhealth` are two of `master`'s thirteen **required
  status checks**, and generalizing the job renames its contexts. A required context that no longer
  reports does not fail a pull request — it blocks it forever, waiting. The duplication is the
  cheaper of the two costs.
- **Applying the 85% per-package coverage floor to `examples/*`** — the consistent-looking choice,
  and ADR-0036's own warning ("a gate that ignores new code is worse than no gate, because it reads
  as green") argues for it. Rejected on the measurement, which was taken before deciding:
  `examples/service` is at **56.2%**, because `main()` is **17 of its 48 statements** and not one of
  them is reachable from a test — it binds a port and blocks in `lifecycle.WaitForSignals` on that
  package's process-wide singleton. Meanwhile `service.go`, which holds every decision the module
  exists to demonstrate, is at **87.1%** and clears the floor on its own. Including the module
  would therefore force lowering the repository's floor to accommodate a demo, which weakens the
  gate that protects the library.
- **A second, lower per-module threshold in `coverage_gate.py`** — rejected on the tool's own
  argument. Its entire case is that there is *one* floor applied *per package*, so that the
  threshold binds the weakest package instead of being absorbed by an average; a table of
  per-module exceptions is the beginning of the averaging it was written against. What replaces it
  is not nothing: CI builds, vets and **runs** the module under the race detector, and ADR-0036's
  concern — new code silently uncovered — does not apply to code no consumer imports.
- **No tests at all, on the grounds that it is a demo** — rejected by ADR-0053's rule 2, whose
  mechanism transfers exactly. An `Example` with no `// Output:` compiles and never runs, which
  looks identical in a green CI run; a demo with no tests is the same failure with the same
  appearance. Every claim its comments make about ordering — that the 429 is counted, that the
  probe is not rate-limited, that liveness ignores what readiness reports — is now an assertion,
  so a rearrangement that breaks the advice fails the build instead of shipping wrong advice with a
  green badge.
- **Importing all twenty-one packages, for completeness** — rejected on two counts. It would make
  every package look mandatory to a reader forming a first impression, and it would teach nothing
  about composition, because a `cache` and a `retry` in a demo service sit beside the skeleton
  rather than inside a decision. The packages left out each have their own runnable examples, which
  is the division of labour this ADR and ADR-0053 are two halves of.
- **`config.Load` instead of `env` for configuration** — `config` is one of the module's
  most-reached-for packages, so showing it is tempting. Rejected because it would make the demo's
  first step "write a YAML file": the reader has to set something up before `go run .` works, and
  `config.Load`'s four runnable examples already cover the file path. `env`'s per-field safe
  fallback is also the more honest fit for a 12-factor service.
- **Reusing the library's test dependencies (`testify`, `goleak`, `rapid`) in the demo's tests** —
  rejected for what the module is *for*. Its `go.mod` has one `require` line, which is the most
  direct statement of what adopting this library costs; a test dependency in the same file would
  blunt exactly that. The tests are stdlib-only, and the one thing `testify` would have bought
  (terser assertions) is not worth the sentence it would cost the README.
- **Leaving the whole composition in `main()`** — the shortest readable program. Rejected because
  it is untestable end to end, and the alternative costs almost nothing: the composition lives in
  `newService`/`handler` and `main()` keeps only the process-level wiring that genuinely cannot be
  tested. That split is also what makes the coverage measurement above *legible* rather than a
  single unhelpful number.

## Consequences

- **The core's dependency graph is provably unchanged.** The root `go.mod` and `go.sum` are
  untouched by this item, `go list ./...` returns no `examples` package, and the module has no
  effect on `spec_api_lint.py` (it exports nothing) or on the surface count, which stays at 141.
- **The demo makes ADR-0004's dependency policy visible, and the number is the argument.** Its
  `go.mod` carries **one `require` line and no indirect requirements at all**, and `go list -deps .`
  resolves **203 packages of which zero are third-party** — every one is the standard library or
  `egl-utils-go`. That claim has been made in prose since ADR-0004; this is the smallest program
  that demonstrates it, and it will keep demonstrating it or fail CI.
- **The import-graph gate now catches strictly more than before, in two directions.** Recursively
  (the `cmd/server/`-only case the old top-level listing missed), and for `contrib` as well as
  `examples`, since the `replace`/workspace/pseudo-version assertions are shared. All five new
  failures were verified by deliberate violation rather than by reading the code: a hidden
  `go.mod` at the top level, Go files only in a nested subdirectory, an added
  `replace … => ../..`, a `go.work` in the root, and — by construction of the regex — a
  pseudo-version require.
- **depguard is a second net that names the wrong problem, and that is why the filesystem check
  runs first.** Verified: with the `go.mod` hidden, `golangci-lint` reports every
  `…/v2/pkg/…` import as an unsanctioned internal edge — "feature packages do not import each
  other", a rule nobody violated. Its suggested fix is to change the imports; the actual fix is to
  restore the module boundary. A gate that fires accurately and diagnoses wrongly costs debugging
  time, which is the argument for short-circuiting on the cause.
- **`examples / service` is not yet a required status check.** Adding a job does not add it to
  branch protection, so until the maintainer adds the context, a pull request can merge while this
  job is red. Flagged in the pull request rather than assumed, because the repository has been
  bitten by exactly this class of drift before (the labels and merge-mode findings of 2026-08-01):
  governance state lives outside the repository and no gate here can assert it.

  > *Amended 2026-08-08: **it is required now** — `master` went from thirteen contexts to fourteen.
  > The warning above was not theoretical and it is worth recording what it cost: between being
  > written and being acted on, this job went red on **two** pull requests and both merged anyway,
  > the second being the `v2.0.1` **release PR**. The defect behind it
  > ([BUG-0002](../bugs/2026/08/BUG-0002-unbuffered-started-channel-deadlocks-two-examples-service-tests.md))
  > was fixed first and the context added second, which is the right order: making a flaky job
  > required converts an ignorable red into a blocked repository. **The trap survives one level
  > down** — the job is a matrix over `module`, so a second entry produces `examples / <name>`,
  > which is a new context and equally not required by default.*
- **The demo pins `v2.0.0`, and something has to keep it current.** That is the accepted cost of
  forbidding `replace`: the version is written down, so it goes stale. Dependabot now watches
  `/examples/service`, and after 14.12 tags `v2.0.1` the bump should arrive as an ordinary
  dependency pull request. It is labelled `build` only — there is no `examples` label in
  `.github/labels.yml`, and Dependabot fails its whole labelling step on a label that does not
  exist on the repository.
- **ADR-0053's rules govern `Example` functions, not this module's tests.** Its rule 1 (the external
  test package) exists so an example cannot demonstrate a call built on unexported state; a
  `package main` with no exported surface has nothing for that rule to protect, so
  `service_test.go` is an ordinary in-package test file. Rule 4 (no dependence on the clock) does
  carry over and is honoured: no test sleeps, and where ordering matters the test waits on a channel
  the task itself signals.
- **One demand data point for 14.10's ledger, found by writing a consumer rather than by
  reviewing the surface:** `env` has `GetDefault`, `GetInt`, `GetBool` and `GetDuration` but **no
  float getter**, so the service's rate limit is an `int` converted at the call site even though
  `ratelimit.NewLimiter` takes a `float64`. This is not proposed as a change here — M14 adds no
  identifier — but it is the shape of evidence 14.10's trigger column is meant to collect, and it
  is worth noting that composing the module surfaced it when reading the module had not.
- **Nothing is released by this item.** `examples/*` modules are never tagged: they are
  documentation, nobody imports them, and a version would imply a compatibility promise about a
  demo. The `examples/service` path carries no major-version suffix for the same per-module reason
  ADR-0040 records for `contrib` — a nested module carries its own version line, not the core's.
- **The accepted cost, stated rather than discovered:** `main()`'s 17 statements are never executed
  by any gate. A `go build` proves they compile and `go vet` proves they are not obviously wrong,
  but nothing proves the process actually starts, binds and shuts down cleanly. Testing that needs
  a spawned binary, a real port and a delivered signal — an integration harness for a demo, which
  is more machinery than the risk justifies. If it ever breaks, the reader running `go run .` is
  the gate.

## References

- ROADMAP 14.5; ADR-0053 (which named this module in its own alternatives); ADR-0040 (the topology,
  the rejected `replace` and `go.work`, and the per-module major-version rule); ADR-0036 (the
  coverage gate and the reasoning this ADR declines to extend); ADR-0035 (the import-graph gate).
- `examples/README.md` (the topology and how to add another), `examples/.golangci.yml`,
  `examples/service/README.md` (the four decisions, in the form a reader copies),
  `tools/import_graph_lint.py` (`NESTED_MODULE_PARENTS`, `unmoduled_go_file`,
  `check_nested_modules`, `check_nested_modules_resolve_like_a_consumer`),
  `tools/coverage_gate.py` (`module_dirs`, where the exclusion is recorded), the `examples` job in
  `.github/workflows/ci.yml`, `.github/dependabot.yml`.
- Go's nested-module conventions: a module boundary stops `./...`; `replace` is ignored for
  dependent modules; a nested module's path carries its own major version.
