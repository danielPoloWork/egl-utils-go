# ADR-0053: runnable examples are documentation that runs — external package, verified output, and no clock

- **Status:** Accepted
- **Date:** 2026-08-01
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** roadmap 14.2 (this item) and 14.3–14.4 (the remaining packages, which follow this ADR
  rather than re-deciding); spec §6 (verification strategy — examples are tests); AGENTS.md §10
  (`godoc / pkg.go.dev documented`); [ADR-0010](0010-circuitbreaker-design.md) and
  [ADR-0021](0021-cache-inmemory-design.md) (the unexported clock seams that rule 4 cannot reach);
  [ADR-0011](0011-retry-backoff-design.md) (jitter, and why `BaseDelay: 0` is a legitimate policy);
  [ADR-0049](0049-pubsub-reshape.md) (`Publish` never blocks — the reason a naive pubsub example
  drops every message); [ADR-0031](0031-ratelimit-middleware-design.md) (the 429 path an example can
  show without waiting); [ADR-0036](0036-coverage-gate.md) (what examples do and do not mean for the
  gate); [ADR-0054](0054-examples-service-module.md) (the composition a package example cannot show)

## Context

pkg.go.dev is where a consumer meets this module, and until this item it showed **twenty-one packages
and one runnable example between them** — `validator.ExampleStruct`. Every package has a package
doc and every exported identifier has godoc, so the gap is not prose: prose says what a function
does, an example shows what a call site looks like, and it is **the only documentation the toolchain
can execute**. Milestone 14 adds no exported identifier, which makes examples the milestone's entire
user-visible payload.

So the question was never "write some examples". It is *under what rules*, because an example is
documentation that runs, and it can fail in three ways prose cannot:

- it can **rot** — the code still compiles while the surrounding contract has moved;
- it can **lie** — it demonstrates a call a consumer is not able to write;
- and in a concurrency library it can **teach the wrong habit** — the most expensive failure of the
  three, because a reader copies what the documentation does.

Eight packages are in scope here, and six of them have time in their contract (`workerpool`,
`pubsub`, `circuitbreaker`, `retry`, `ratelimit`, and `fanin`/`fanout`'s draining discipline). That
is what forced a written convention instead of eight independently-styled functions.

## Decision

Examples live in the package's **external** test package (`package <pkg>_test`), one file per package
named `example_test.go`. **Every example carries a verified `// Output:` comment**, and what it prints
is the *shape* of a result — a boolean, a count, a sorted slice, a status code — never an incidental
string such as an error's text, a duration, or a timestamp. **No example may depend on elapsed
wall-clock time**: `time.Sleep` appears in none of them, and where a component's behaviour is
time-based the example either configures it so no time needs to pass, or synchronises through a
channel the example itself owns.

Four rules, each with the reason it exists:

**1. The external test package, so an example cannot document what a consumer cannot write.** In
`package cache` an example compiles against unexported identifiers; in `package cache_test` that is a
compile error. This is not a new convention — **20 of 21 packages already carry an external test
file**, several alongside an in-package one for internal tests, so examples simply join the half that
was already restricted to the public surface. (`lifecycle` is the single exception and gets its first
external file in 14.3.)

**2. A verified `// Output:`, because the alternative silently does nothing.** `go test`
**compiles an `Example` without an output comment and never runs it.** An example without one
therefore proves only that the code builds: it cannot catch a panic, a wrong result or a changed
contract, and it looks identical in a green CI run. That is the trap this rule exists to close. Where
a value is genuinely unprintable — a generated request ID, a live duration — the example prints a
derived property (a length, a boolean, a count) rather than dropping the comment.

**3. The output is a contract, so print shape, not strings.** `validator.ExampleStruct` prints
`err != nil` rather than the error text, and that is the rule generalised: whatever an example
prints, `go test` enforces from then on. Printing an error message would quietly make the *wording*
of that message part of the module's observable behaviour, breakable by a reviewer improving it. Also
excluded for the same reason: anything Go does not order (map iteration), anything the machine
decides (addresses, timings), and anything a concurrent component is explicitly not promising — which
is why the `fanin` example sorts before printing and the `fanout` example prints a total rather than
which output received what.

**4. Determinism without sleeping — the rule that shaped every example in this item.** The fake
clocks that make the *tests* deterministic (`now func() time.Time` in `cache`, `circuitbreaker`,
`ratelimit`, `retry`) are **unexported fields**: reachable from an in-package test, and by rule 1 not
reachable from an example. What is left turned out to be better than the seam would have been —
configure the component so **no time needs to pass**:

- the breaker takes a **one-minute open timeout**, so its lazy half-open transition provably cannot
  fire mid-example, and the open→refuse behaviour is shown with zero elapsed time;
- `retry.Policy` takes **`BaseDelay: 0`**, which ADR-0011 defines as immediate retries, so the
  example demonstrates the attempt budget in microseconds;
- the limiter's **bucket starts full**, so burst-then-refuse needs no refill;
- `workerpool`'s queue-full path is reached by **waiting on a `started` channel the example owns**,
  which guarantees the worker has dequeued the first task before the queue is filled — an ordering
  claim, not a timing one;
- and `semaphore`'s bound is shown by handing capacity between two goroutines, where the example
  terminating *is* the proof that the blocked acquire completed only after the release.

## Alternatives Considered

- **In-package examples (`package cache`)** — the smaller diff, and it would have let the timing
  examples inject the fake clock the tests use. Rejected because it removes the one guarantee that
  matters: nothing would stop an example from demonstrating a call built on unexported state, and a
  lying example is worse than a missing one. The cost — no access to the test seams — is what rule 4
  converts into a virtue.
- **An `examples/` directory of runnable programs instead of `Example` functions** — rejected *as a
  substitute*: a program in a directory is attached to no identifier, so pkg.go.dev shows nothing
  beside `Merge`, and nothing executes it in CI. It is a real complement, though, which is why 14.5
  adds one as its own module for the cross-package composition no single package doc can show.
- **Output-free examples everywhere, for freedom in what they demonstrate** — rejected: by rule 2's
  mechanism that converts the entire set into compile-only checks. A documentation suite that cannot
  fail is one that will drift, and it would drift invisibly.
- **Exporting the clock seams so examples could be deterministic the way tests are** — rejected on
  two independent grounds: Milestone 14 promises no new identifier, and a knob whose only caller is
  documentation is a knob that ships forever and appears in every consumer's field of view as though
  it were a feature.
- **Sleeping where determinism is hard** — rejected twice over. It is flaky on a loaded CI runner,
  and it is *exemplary*: this module exists partly to remove goroutine-timing bugs, so an example
  that sleeps to work teaches precisely the habit the library replaces.

## Consequences

- **Thirteen examples across the eight packages** in this item, all with verified output, and every
  one of them confirmed to *run* rather than merely compile — `go test -v` lists all thirteen, each at
  0.00–0.01s, which is the check rule 2 exists to make possible. No API change, no behaviour change, no new dependency: these are
  `_test.go` files a consumer never compiles.
- **They reach consumers only when a version is tagged.** pkg.go.dev renders the tagged tree, so this
  item's output is invisible on `master`; that is the argument 14.12 makes for cutting v2.0.1.
- **Coverage did not move, and that is the honest result** — measured after the fact rather than
  predicted. Examples with `// Output:` do run as tests, so they *could* raise a number, but the eight
  packages read exactly as before (four at 100%, `ratelimit` 98.1%, `retry` 97.7%, `fanin` 95.7%,
  `fanout` 93.3%): every path these examples touch was already covered. That is the desired
  relationship — examples document, tests verify — and it means ADR-0036's gate keeps measuring what it
  measured, with no example standing in for a missing test.
- **Examples are not leak-gated, so each one closes what it opens.** `goleak` runs per test via
  `defer goleak.VerifyNone(t)` and there is no `TestMain`, so a goroutine leaked by an example is
  *not* caught. Every example therefore closes its pool, closes its broker and cancels its
  subscription context — which is what a consumer should copy in any case, so the constraint and the
  pedagogy point the same way.
- **The rules bind the rest of the milestone.** 14.3 and 14.4 follow this ADR instead of re-opening
  it, and they inherit two hard cases named here in advance: `lifecycle.WaitForSignals` blocks until
  a signal arrives, and `db.Transaction` needs a `*sql.DB` for which this module deliberately has no
  driver (ADR-0004).
- **The accepted cost: a behaviour that is nondeterministic by nature can be described but not
  shown.** Cache expiry is the clearest instance — the honest example stores and reads a live entry,
  and the TTL behaviour stays in prose godoc rather than being demonstrated with a sleep. Recording
  that as a limitation is better than an example that is *usually* right.

## References

- Spec §6 (verification strategy); AGENTS.md §10 (the `godoc / pkg.go.dev` bar); ROADMAP 14.2.
- `testing` package documentation: example naming (`Example`, `ExampleT`, `ExampleT_M`,
  `ExampleT_suffix`) and the output-comparison rule (leading and trailing space is trimmed; an
  example with no output comment is compiled but not run).
- ADR-0010, ADR-0021 (unexported clock seams); ADR-0011 (`BaseDelay: 0`); ADR-0049 (`Publish` never
  blocks, and the 16-message default subscriber buffer that lets a sequential example work);
  ADR-0031 (the 429 admission path); ADR-0036 (the coverage gate); ADR-0007, ADR-0008 (the
  drain-or-cancel contract the `fanin`/`fanout` examples have to honour).
- `pkg/validator/validator_test.go` — `ExampleStruct`, the pre-existing example this convention
  generalises.
