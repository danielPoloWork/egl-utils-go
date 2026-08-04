# 2026-08-04 — 14.5: the boundary that fails silently, and the floor that was not applied

Roadmap 14.5 → [ADR-0054](../../../adr/0054-examples-service-module.md). `examples/service`: one HTTP
service composed from eight packages, as a module of its own, plus the enforcement that keeps it
outside the library's dependency graph.

Two things turned out to be more interesting than the service itself — how the module boundary fails
when it fails, and what the coverage gate should do with a demo. Both were settled by measuring
rather than by reasoning, and both changed the answer I expected.

## The boundary fails with no error at all

The roadmap line inherited 10.13's warning: a directory of `.go` files with no `go.mod` silently joins
the root module. It is worth knowing exactly *how* silent, so the go.mod was moved aside and the tools
asked:

```
$ mv examples/service/go.mod examples/service/go.mod.bak
$ go list ./... | grep -c examples
1
$ go list -deps ./... && echo OK
OK
```

That is a different failure from the one `contrib` has. ADR-0040 recorded that a contrib directory
without its `go.mod` makes `go list ./...` die on the driver import — an opaque error, but *an*
error, which is why the filesystem check runs first and short-circuits. Here there is nothing to
die on: everything a composition example imports is already in the core's graph, so the module
boundary can disappear and every gate stays green. The first sign would arrive later, as a line
nobody put there in the library's own `go.mod` after someone's `go mod tidy`.

**depguard is a second net, and it names the wrong problem.** With the file still hidden:

```
examples\service\main.go:44:2: import 'github.com/danielPoloWork/egl-utils-go/v2' is not allowed
  from list 'internal-edges-only-where-mandated': feature packages do not import each other …
```

Accurate as a rule application, useless as a diagnosis: nobody violated the internal-edge policy, and
the fix it points at is to change the imports rather than to restore the boundary. A gate that fires
correctly and diagnoses wrongly costs debugging time, which is the whole argument for short-circuiting
on the cause with a message that names it.

## The check was generalized, and it grew two teeth

Rather than copy `check_contrib_is_separately_moduled`, it became `check_nested_modules` over a
`NESTED_MODULE_PARENTS` map, and it now walks **recursively** — which closed a hole the original had
without anyone noticing: the old check listed the top level of each directory, so
`examples/foo/cmd/server/main.go` with no `examples/foo/go.mod` above it would have passed. Same
failure, one directory deeper.

Then scope grew on the item's own wording. "No `replace`, no `go.work`" is stated in 14.5's brief and
was decided by ADR-0040 on 2026-07-27 — and had been **prose ever since**, enforced by nothing. Three
assertions now cover every nested module, contrib included:

- no `replace` directive (it is ignored for dependent modules, so it validates a configuration no
  consumer receives — ADR-0040's central argument, previously unguarded);
- no committed `go.work` in the repository root (it switches every root-level `go`,
  `golangci-lint` and `govulncheck` invocation into workspace mode);
- the core requirement is a released tag, not a pseudo-version from `go get …@master`.

None of the three breaks a build, which is precisely why they need a gate. All five new failure modes
were confirmed **by deliberate violation** — hidden `go.mod`, Go files only in a subdirectory, an
added `replace … => ../..`, a `go.work` in the root — not by reading the code and believing it.

## A new CI job, and the reason is outside the YAML

Extending the `contrib` matrix with an `examples/service` entry was the obvious tidy move and it is a
trap: `contrib / redishealth` and `contrib / pgxhealth` are two of `master`'s thirteen **required
status checks**, and generalizing the job renames its contexts. A required context that no longer
reports does not fail a pull request — it blocks it forever, waiting for a check that will never
arrive. Duplicating fifteen lines of YAML is the cheaper cost.

The mirror of that: adding a job does not make it required. **`examples / service` is not yet in
branch protection**, so until the maintainer adds the context a PR can merge with it red. Governance
state lives outside the repository and no gate in here can assert it — the same class of drift as the
missing labels and merge-mode findings of 2026-08-01.

## The coverage decision: no floor, and the number is the argument

14.5 asked for this to be decided explicitly. It was measured first:

| | statements | covered |
|---|---|---|
| `main.go` | 17 | **0.0%** |
| `service.go` | 31 | **87.1%** |
| package | 48 | **56.2%** |

So the two halves say opposite things, and that is what makes the decision easy. `service.go` holds
every decision the module exists to demonstrate and clears the 85% floor on its own. `main()` is a
third of the statements and **none of them is reachable from a test** — it binds a port and blocks in
`lifecycle.WaitForSignals` on that package's process-wide singleton. Adding `examples/*` to
`coverage_gate.py` would therefore mean lowering the repository's floor to accommodate a demo, or
introducing a second per-module threshold into a tool whose entire argument is that there is *one*
floor applied *per package* so the weakest package binds it.

What replaces it is not nothing, and it is the bar that actually matters for documentation: CI
**runs** the module, `go test -race` included. That is ADR-0053 rule 2's mechanism transferred — an
`Example` with no `// Output:` compiles and never runs while looking identical in a green run, and a
demo with no tests is the same failure wearing the same clothes. The exclusion is recorded in
`coverage_gate.py`'s own docstring with these numbers, so a future reader finds a decision rather
than an absence.

The four uncovered statements in `service.go` are named rather than papered over: the `default`
branch of the `Submit` switch (unreachable — a non-blocking `Submit` returns only `nil`,
`ErrQueueFull` or `ErrClosed`) and the `io.WriteString` failure path (needs a `ResponseWriter` that
fails, which `httptest`'s recorder never does).

## What the service shows, and what it deliberately does not

Eight packages, not twenty-one. A demo importing everything makes every package look mandatory to a
reader forming a first impression and teaches nothing about composition; the packages left out have
their own runnable examples. `env` rather than `config.Load`, because a demo whose first step is
"write a YAML file" is one the reader has to set up before it runs.

Four decisions are the content, each asserted by a test rather than described:

- **Chain order** — `Recoverer → RequestID → Logger → metrics → ratelimit → Cors → mux`, with the
  logger and the recorder *outside* the limiter so a shed request is logged and counted. The rate of
  429s is exactly what tells an operator the limit is wrong; a limiter outside the recorder makes its
  own effect invisible. The chain wraps the mux rather than each route, so the mux's own 404s and
  405s are logged too.
- **Operational endpoints outside the application chain** — `/healthz`, `/readyz` and `/metrics`
  behind `Recoverer` and nothing else. A limiter answering 429 to a readiness probe gets a healthy
  instance killed; a 15-second scrape would add ~5 800 requests a day to `http_requests_total` that
  the service never served for anyone.
- **Liveness and readiness are different questions** — `/healthz` carries no checks at all, so one
  dependency's blip cannot restart every instance at once. And the readiness probe **exercises the
  real admission path**: it submits a no-op through the same `Submit` the handler uses, so the pool's
  three errors already are the three answers an orchestrator needs. A probe that returns `nil`
  documents the wiring and verifies nothing.
- **Registration order is dependency order** — `pool.Close` registered first so it runs last, the
  server's `Shutdown` registered last so it runs first. Stopping the listener first means no request
  can enqueue work while the pool drains.

Twelve tests, and the pointed ones are gates: `/healthz` 200 while `/readyz` is 503 with the pool
closed — **and the 503 body names which check failed and never why**, so an unauthenticated endpoint
leaks no internals; the 429 in the exposition while the scrape is absent from it; a single worker
surviving a panicking task and running the next one. No test sleeps: where ordering matters the test
waits on a channel the task itself signals, ADR-0053 rule 4 carried over to a place it was not
written for.

## One number worth quoting, and one gap worth logging

`examples/service`'s `go.mod` has **one `require` line and no indirect requirements at all**, and
`go list -deps .` resolves **203 packages of which zero are third-party**. ADR-0004 has claimed that
in prose since Milestone 1; this is the smallest program that demonstrates it, and it will keep
demonstrating it or fail CI. Its tests are stdlib-only for the same reason — a module whose point is
what a single dependency buys should not need a second one to test itself.

And writing a consumer surfaced something reading the surface had not: **`env` has no float getter.**
`GetDefault`, `GetInt`, `GetBool`, `GetDuration` — so the service's rate limit is an `int` converted
at the call site even though `ratelimit.NewLimiter` takes a `float64`. Not proposed as a change (M14
adds no identifier), but it is exactly the shape of evidence 14.10's ledger wants in its trigger
column, and it is logged here so 14.10 does not have to rediscover it.

## State

Milestone 14 is **5 of 12**. `master` is at `51b9310` (14.4, #88). Core `go.mod`/`go.sum` provably
untouched, `go list ./...` returns no `examples` package, surface unchanged at 141, coverage of the
core unmoved. 0 golangci-lint issues in both modules, gofumpt clean, four policy tools green.

Next: **14.6**, `contrib-release.yml` — the release act for the submodules, closing the hole
`release.md` recorded on 2026-08-01 (`release.yml` matches `v*.*.*`, which a `contrib/…` tag does
not, so the first two submodule releases were verified by hand).

Still open for the maintainer: `examples / service` as a required status check, and the two
branch-protection flags (`required_linear_history`, `required_conversation_resolution`).
