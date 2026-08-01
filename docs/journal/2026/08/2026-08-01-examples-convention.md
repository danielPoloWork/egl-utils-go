# 2026-08-01 — 14.2: documentation that runs, and the rule the unexported clocks forced

Milestone 14's first substantive item. Thirteen examples across eight packages, and the deliverable
that outlives them is [ADR-0053](../../adr/0053-runnable-examples-convention.md): four rules, each
written down with the reason it exists, because 14.3 and 14.4 are meant to follow them rather than
re-decide.

## The rule I did not get to choose

The plan said "deterministic without sleeping" and I expected that to be about discipline. It is
about reach. The seams that make the *tests* deterministic — `now func() time.Time` in `cache`,
`circuitbreaker`, `ratelimit`, `retry` — are **unexported fields**, and rule 1 puts examples in the
external test package, so they cannot touch them. Two rules I chose independently turned out to
collide, and the collision is the interesting part of the item.

Exporting a clock option would have dissolved it in one line. It was refused twice over: Milestone 14
promises no new exported identifier, and **a knob whose only caller is documentation ships forever** —
it would sit in every consumer's field of view looking like a feature.

What is left is better than the seam would have been, because it forced each example to be honest
about what the component actually promises:

- the breaker takes a **one-minute** open timeout, so the lazy half-open transition *provably* cannot
  fire mid-example — the state machine is shown without a clock rather than in spite of one;
- `retry.Policy{MaxAttempts: 3}` leaves `BaseDelay` at zero, which ADR-0011 already defines as
  immediate retries, so the attempt budget is demonstrated in microseconds;
- the limiter's bucket **starts full**, so burst-then-refuse needs no refill at all;
- `workerpool`'s queue-full path waits on a `started` channel the example owns, which turns a timing
  assumption into an **ordering** guarantee: the worker has provably dequeued the first task, so the
  queue is empty and the next submit fills it;
- and `semaphore`'s bound is shown by handing capacity between two goroutines, where **the example
  terminating is the proof** that the blocked acquire completed only after the release.

`time.Sleep` appears in none of them, which matters beyond flakiness: a reader copies what the
documentation does, and this module exists partly to remove the goroutine-timing bugs that habit
produces.

## The trap that makes rule 2 load-bearing

**`go test` compiles an `Example` with no `// Output:` comment and never runs it.** So the difference
between a verified example and a decorative one is one comment, and there is no signal either way —
the package still prints `ok`. That is why the ADR requires the comment on every example, and why I
checked with `go test -v` instead of trusting the package total: all thirteen appear as run tests at
0.00–0.01s. Had one silently not run, nothing would have said so.

The companion rule came from the one example that already existed. `validator.ExampleStruct` prints
`err != nil`, not the error text — and generalised, that is: **whatever an example prints, `go test`
enforces forever.** Printing an error message would quietly make the message's *wording* part of the
module's observable behaviour, breakable by a reviewer improving it. So: booleans, counts, sorted
slices, status codes. It is also why `fanin` sorts before printing (Merge preserves order *within* an
input, not across inputs) and `fanout` prints a total rather than which output received what.

## Two traps found by writing the code

**pubsub's default subscriber buffer is 16, and that is the only reason a sequential example works.**
ADR-0049's `Publish` never waits for a subscriber, so publish-three-then-read-three is only safe
because the buffer holds them; with an unbuffered subscription every message would have been dropped
and the example would have hung on its first read — a demonstration of the drop path masquerading as
a broken example. The comment in the example says so, because a consumer who assumes rendezvous
delivery will hit exactly this.

**`fmt.Println` of an empty header value leaves a trailing space, and the example comparator does not
trim per line** — it trims the whole output's ends. So printing `rec.Code` alongside an absent
`Retry-After` produces `"200 "` against a `// Output:` line of `200`, and the example fails for a
reason invisible in the diff. The middleware example issues two explicit requests instead of looping,
which reads better anyway.

## What the item deliberately did not do

**No new identifier, no behaviour change, no dependency.** And coverage did not move — I measured
instead of assuming, expecting a small rise, and got eight numbers identical to the baseline (four at
100%, `ratelimit` 98.1, `retry` 97.7, `fanin` 95.7, `fanout` 93.3). Every path these examples touch was
already covered by a test, which is the relationship I want: examples document, tests verify. It also
means no example is quietly standing in for a missing test, so ADR-0036's gate still measures what it
measured.

**Examples are not leak-gated** — `goleak` runs per test via `defer goleak.VerifyNone(t)` and there is
no `TestMain` — so a leaked goroutine in an example would pass unnoticed. Each one therefore closes
its pool, closes its broker and cancels its subscription context, which is what a consumer should
copy in any case: the constraint and the pedagogy point the same way.

And the cost is recorded rather than papered over: **a behaviour that is nondeterministic by nature
can be described but not shown.** Cache expiry is the clearest case — the honest example will store
and read a live entry, with the TTL behaviour left to prose godoc in 14.4. An example that is
*usually* right is worse than one that is narrower.

## State

Milestone 14 is 2 of 12. `master` is at `c1bb07d` (14.1); this is draft PR on
`feat/examples-concurrency-resilience`. Next: **14.3**, the HTTP and observability set, which inherits
two named hard cases from this ADR — `lifecycle` is the one package with no external test file *and*
its `WaitForSignals` blocks until a signal arrives, and `slog` output carries timestamps that rule 3
forbids printing.

Still open from the 2026-08-01 audit: the two branch-protection flags
(`required_linear_history`, `required_conversation_resolution`) need a whole-object
`gh api -X PUT` the maintainer has to run.
