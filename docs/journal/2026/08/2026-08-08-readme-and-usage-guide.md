# 2026-08-08 — the README proved the project was well-run and never sold it

An evaluation of the front door and the usage documentation, from a product rather than a
governance standpoint, and the rewrite that followed.

## The finding

The README was a **governance artifact wearing a product README's clothes**. It proved the project
is well-run and did nothing to get anyone to use it. Measured rather than asserted:

| | Before |
|---|---|
| Lines of Go showing the library in use | **0** (`grep -c '```go'`) |
| Position of `go get` | **absent** — the first actionable command was `go build ./...` |
| Lines spent on internal milestones | 20 of 165 |
| Statement of why to choose this over assembling four libraries | none |

Three specific faults were worth naming:

**The first actionable section addressed the wrong audience.** "Build, test, run" opened with
`go build ./...` and `go test ./...` — what a *contributor* runs after cloning. A consumer needs
`go get`, and the import path was a sub-bullet twenty lines further down.

**Internal voice reached the public page.** "Provide a production-ready Go utilities module"
(imperative, straight out of a requirements document), "Design philosophy *(imported from the
brief)*", "the *frozen specification*". None of that is wrong; all of it is written for the people
building the thing.

**The M1–M14 milestone table was an internal tracker on the front door.** No adopter cares that
Milestone 6 was "Structured logging ✅".

## What was kept

The evaluation was not a demolition. The `Packages` section added earlier the same day is the
strongest thing on the page and survives verbatim; so does the dependency-discipline story, the
`contrib/` and `examples/` separation, and the governance depth — which is a genuine enterprise
asset and simply needed *summarising and linking* rather than dumping.

`examples/service/README.md` (192 lines) is, honestly, better written than the README was.

## Two constraints found before proposing anything

The README is not free-form: two lint checks read it.

- `version-lockstep` parses the literal `Status-vX.Y.Z` badge. Any badge redesign has to keep it.
- `check_milestones` **fails outright if no milestone rows parse from README.md**. The table could
  not simply be deleted.

That second one shaped the answer. Wrapping the table in `<details>` removes it from the visible
page while leaving the rows parseable — **all 14 still parse, and no tool changed.** Checking the
constraint first turned what looked like "delete the section, then relax the gate" into a
presentation change with no gate change at all, which is the better outcome: a gate should not be
loosened to accommodate a layout preference.

## The quickstart, and the rule that made it safe

The single biggest gap was that no code appeared anywhere. The fix is a complete ~40-line service —
`workerpool` for bounded background work, `ratelimit`, the `middleware` chain, `health`, and
`lifecycle` for ordered shutdown.

**There is no Go toolchain on this machine, so a snippet written from memory would be a guess on the
front page.** Every construct was therefore lifted from code CI actually compiles, vets and
race-tests — `examples/service/*.go` — and every signature in the usage guide from the package
`example_test.go` files, which CI additionally *runs* with verified output.

That discipline caught a real error. The guide's first draft had:

```go
outs := fanout.Split(ctx, in, 4)
```

`Split` takes **variadic output channels and returns nothing**:
`func Split[T any](ctx context.Context, in <-chan T, outs ...chan<- T)`. It would not have compiled,
in the document whose purpose is showing people how to use the library. Reading the signature is what
found it; nothing else would have.

## The usage guide

`docs/usage/README.md` fills the layer that was missing: between `go get` and per-identifier
reference documentation there was nothing showing the packages doing a job. It is organised by
question — *how do I run background work without unbounded goroutines, retry safely, shed load, shut
down in the right order* — with the smallest code that answers each, and a link onward to the
reference.

It deliberately carries the *reasoning* the reference cannot: why non-blocking submission is the only
defensible mode on a request path, why jitter is not decoration, why liveness and readiness are
different questions, why a readiness probe that returns `nil` verifies nothing.

## Gating the new document

Check 12 (`readme-packages`) was extended to the guide — but **one way only**. The README is held to
a full bijection: the front door must name every package. The guide is checked against dead links
only, because a task-oriented document is entitled to omit a package it has no recipe for, and must
never point at one that does not exist.

Verified by sabotaging a link to `…/pkg/retrie`, which failed as intended. The first attempt at that
sabotage did not run at all — a `sed` expression where `|` served as both address separator and
delimiter — and the run came back green, which would have read as a pass. Second time today that a
violation test needed checking as carefully as the thing it tests.

## Result

| | Before | After |
|---|---|---|
| Go examples on the front page | 0 | a complete service |
| First actionable command | `go build ./...` | `go get …/v2` |
| Visible internal-process content | milestones + document index | collapsed under *Project governance* |
| Task-oriented usage documentation | none | `docs/usage/README.md`, all 21 packages |
| Lint changes required | — | none for the restructure; one extension for the new file |
