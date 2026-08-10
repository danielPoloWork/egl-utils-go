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

## State

Between milestones; no milestone on this PR, and none to set. 43 board findings, 2 closed. The
backlog is ordered chronologically and severity travels on the line, so the next pick is a reading
of `ISSUES.md`'s badges, not of its top.
