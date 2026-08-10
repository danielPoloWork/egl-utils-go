# 2026-08-10 — the quickstart is production code, and so is a doc comment

First fix out of the release review board's backlog:
[#107](https://github.com/danielPoloWork/egl-utils-go/issues/107), `MAJOR`, filed independently by
three of the seven reviewers. The README's quickstart discarded the `ListenAndServe` error and
constructed an `http.Server` with no timeouts.

## What changed

- **`README.md`'s quickstart** — the `http.Server` literal now states all four timeouts, and the
  listener goroutine checks its error against `http.ErrServerClosed`, logs anything else and calls
  `lifecycle.Trigger()`. The two `lifecycle.Register` calls moved above the goroutine, so the hooks
  exist before anything can `Trigger` them.
- **`pkg/lifecycle`'s package doc comment** — the same defect at a second and larger site. Fixed the
  same way. Not filed by the board; found while confirming #107's blast radius.
- `ISSUES.md` — `#107` flipped, and `#106` flipped, which
  [#150](https://github.com/danielPoloWork/egl-utils-go/pull/150) owed after merging behind #149.

## Three reviewers, three arguments, one line

The board's convergence on `README.md:90-91` is the part worth keeping. Site Reliability read it as
an availability defect: a process that survives a bind failure passes liveness while serving
nothing, which is the worst shape an alert can have — the page never fires and the traffic never
lands. API review read it as a contradiction: `examples/service/main.go:95-108`, which the README
links to two lines below the block, says *in writing* that a failed listener "must not leave the
process alive and silent". Product Security read it as Slowloris exposure with gosec's G112 enabled
repo-wide.

None of the three needed the other two. That is the argument for the rule the board wrote down:
**a quickstart is a production template people paste, and it earns the same review as production
code.** It is not illustrative pseudocode with the error handling elided for clarity — it is the
most-copied fifty lines the project ships.

## The site nobody filed is the one that reaches consumers

`pkg/lifecycle/lifecycle.go:13` taught `go func() { _ = server.ListenAndServe() }()` in the package
doc comment. Verified against the tags rather than assumed: the line has been there since
**`v1.0.0`** — at `lifecycle/lifecycle.go` before the `/v2` layout move, at
`pkg/lifecycle/lifecycle.go` after — so it is in all five published versions of this module.

That makes it the more consequential of the two, on this board's own key finding: **pkg.go.dev
renders Go doc comments and does not render Markdown.** The README correction reaches a reader who
visits GitHub; the doc-comment correction is what `godoc`, pkg.go.dev and every IDE hover show. A
finding filed against the README had its larger half outside the README.

The fix also repays a debt inside that same doc. Its `Trigger` paragraph says the function exists
for "code that must start the shutdown itself — a fatal background error, an admin endpoint, a
supervisor command", and the example above it demonstrated the fatal background error being thrown
away. `Trigger` now has the use it was documented for.

## Deliberately not touched

- **`/healthz` returning a constant 200, and the missing readiness endpoint** — that is
  [#108](https://github.com/danielPoloWork/egl-utils-go/issues/108), a separate `MAJOR` on the same
  forty lines. One issue per PR; folding it in would have made the diff unreviewable against either.
- **The usage guide's shutdown recipe** (`docs/usage/README.md:412-417`) — checked, and clean. It
  shows `Register` and `WaitForSignals` with no listener at all, so there is no discarded error to
  fix. Recorded because "the same defect is probably everywhere" is a guess, and the repository was
  searched instead: `ListenAndServe` appears in exactly two `.go` files, and the other —
  `examples/service/main.go` — was already correct and is what both fixes were modelled on.
- **The Italian source specification** — `d4np-go.md:79` (`go server.ListenAndServe()`) and its two
  copies under `.spec/` and `docs/specs/v2/`. That is the inbound brief this library was built
  from, and `docs/specs/v2/d4np-go.md:124-149` already carries a corrected, `go build`-clean
  rewrite beside the original. A frozen inbound document is a dated record, not a page to edit into
  agreement with the present.
- **No `Fixed` entry for the README.** The quickstart landed in #105, after `v2.0.1`, so the broken
  version never reached a consumer through a release; the `[Unreleased]` entry describing it is
  corrected in place, exactly as #150 handled the import-graph sentence. The `pkg/lifecycle` fix
  does get a `Fixed` entry, because that one shipped in five published versions.

## Second pass, same forty lines: #108

[#108](https://github.com/danielPoloWork/egl-utils-go/issues/108) landed on the same block right
after #107 merged, and it is worth recording that the issue's *remedy* and its *diagnosis* pointed
in different directions.

The finding: the quickstart mounted `health.Handler()` with no checks and exposed no readiness
endpoint, "in a service whose stated point is load shedding". The remedy as written: "**register at
least one real check**, and add the readiness endpoint the usage guide already documents."

Read as "put checks on `/healthz`", the first half of that remedy would have been wrong, and wrong
against this module's own documented design in three places:
`examples/service/service.go:127-132` explains that dependency probes on liveness let one blip
restart every instance at once; `docs/usage/README.md:251-253` states the same distinction; and
`pkg/health/health.go:44-45` documents a checkless handler as "a bare liveness endpoint", which is a
sanctioned configuration rather than an omission.

So the defect is **not** that `/healthz` has no checks. It is that `/readyz` did not exist, and that
nothing on the page said the checkless liveness endpoint was a decision — a reader saw "health
endpoint" and had no way to tell deliberate from unfinished. Both are now fixed:
`/readyz` submits a no-op task through the same `pool.Submit` the request handler uses, so a
saturated instance is taken out of the load balancer instead of being kept in it by a probe that
verifies nothing, and the comment above `/healthz` says why it stays bare.

**A board finding is evidence, not a work order.** Three disciplines converging on #107 made that
finding stronger than any one report; here a single reviewer's remedy line, followed literally,
would have contradicted the design the same reviewer was measuring against. The finding was right
and the prescription was loose — which is the ordinary case, and the reason the fix gets designed
from the code rather than transcribed from the issue.

## Something to watch

The quickstart is now 97 lines in the fence, 62 of them code, having grown by half across two
`MAJOR` fixes — and every line of that growth was earned. It is nonetheless converging on
`examples/service`, which already exists, is compiled by CI, and is linked from three lines below
the block. There is a point at which the honest front page is a short hello-world plus "the
production shape is one link away", and the next `MAJOR` on these lines is the moment to decide
whether that point has arrived. Recorded, not acted on: it is a design question, not a defect.

## Third pass: the guide's own claim was the tell, on #109

[#109](https://github.com/danielPoloWork/egl-utils-go/issues/109) was found by two reviewers
independently: the retry recipe (`docs/usage/README.md:118-124`) showed
`retry.Policy{MaxAttempts: 3}` — no `BaseDelay`, no `Jitter` — directly beneath prose promising
"exponential backoff and jitter" and, six lines further down, a paragraph titled **"Jitter is not
decoration"** warning that its absence turns a failure into a synchronised retry storm. The snippet
was the storm the paragraph beside it warns against.

The root cause was traceable rather than sloppy: `pkg/retry/example_test.go:14-21` uses the same
zero-delay policy deliberately, so `ExampleBackoff` stays clock-free and instant, and the comment
above it says so and gives the production shape as a comment. The usage guide recipe carried the
*code* faithfully from that test and dropped the *comment* that made it safe to show — the guide's
own `[Unreleased]` claim is "every snippet is derived from code CI compiles and runs", which was
true of the code and silent about the one line of context that changed its meaning.

**The fix states the production policy directly** (`MaxAttempts: 5, BaseDelay: 100ms, MaxDelay: 2s,
Jitter: 0.2`) rather than reproducing the test's zero-delay values with a comment, which is the
shape every other recipe in this guide already takes — `workerpool.New(4, 64, ...)`,
`ratelimit.NewLimiter(20, 40)`, `cache.New[...](5 * time.Minute)` are all real numbers, not
test-clock values annotated with what production would use. A recipe answers "how do I", and the
answer is now copy-paste correct instead of copy-paste-then-remember-to-change-four-fields.

## State

Between milestones; no milestone on any of these PRs, and none to set. 43 board findings,
**4 closed** (#106, #107, #108, #109). The backlog is ordered chronologically and severity travels
on the line, so the next pick is a reading of `ISSUES.md`'s badges, not of its top.

**Standing order added mid-session:** the maintainer instructed that the agent opens pull requests
itself from now on, rather than pushing the branch and reporting the `gh pr create` command.
Everything else in the §6.1 boundary is unchanged — merging, `master` and release publication stay
human. `AGENTS.md` §6.1/§6.4 and `CLAUDE.md`'s TL;DR still say the opposite in writing and need a
governance PR to catch up; flagged, not yet filed.
