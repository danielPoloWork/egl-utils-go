# 2026-08-01 — 14.3: the examples that assert a security property, and the one that had to say less

Twelve examples across `middleware`, `health`, `metrics`, `logger` and `lifecycle`, written under
[ADR-0053](../../adr/0053-runnable-examples-convention.md) with nothing re-decided — which is what
the ADR was for. The interesting part was not writing them; it was that three of its four rules bit
harder here than in 14.2, and one package could only be documented by writing *less* code.

## Rule 3 turned out to be the design constraint

"Print shape, not strings" sounded like a style rule. In this set it decided the shape of half the
examples, because these packages emit values that move every run:

- `slog` and the `Logger` middleware both write a `time` field and a `duration`. So those examples
  **decode the JSON line and print only the stable fields** — and that reads *better* than dumping
  the line would, because printing `line["level"], line["method"], line["path"], line["status"]`
  documents the schema explicitly instead of leaving a reader to infer it from one sample.
- `RequestID` generates a random id, so its example prints a **client-supplied** id for the
  deterministic half and `!= ""` for the generated half. The split is the documentation: it shows the
  extract-or-generate contract in the only way that can be asserted.
- `metrics` serves a histogram whose buckets depend on how long the requests actually took, so the
  example **filters the scrape to the counter family**. The comment says why, which also puts the
  no-path-label cardinality decision in front of the reader at the moment they are looking at labels.

The `Logger` example gained an accidental bonus: the request URL is `/orders?token=secret` and the
printed `path` is `/orders`. The query string never appears, so the example *demonstrates* the
path-only rule rather than asserting it in a comment.

## Two examples assert a security property, which is where they beat prose

`Recoverer`'s example panics with `"connect to db-primary.internal failed: password=hunter2"` and
prints `500 false` — the `false` being `strings.Contains(body, "hunter2")`. A sentence saying "the
panic value never reaches the client" is a promise; this is a test that fails if the promise breaks,
sitting in the documentation where a reader evaluating the middleware will see it.

`health`'s does the same for information disclosure: a failing Redis probe returns
`dial tcp 10.0.0.7:6379: connection refused`, and the 503 body is
`{"status":"unavailable","checks":{"cache":"fail","database":"ok"}}` — the *name* of the failing check
and nothing else. An unauthenticated caller learns that the cache is down, not where it lives.

## `lifecycle`: the hard case the ADR named in advance, answered by writing less

`WaitForSignals` blocks until a signal that will never arrive in a test binary, and calling it would
install a real process-wide signal handler for the rest of the run. So it appears as **prose inside
the example** — with the timeout argument and the fact that `0` means no deadline, the parameter most
likely to be copied wrong — while the runnable part is `Register` + `Shutdown` proving the LIFO
order. Documenting the blocking call by *not* calling it is the honest version.

The package is also the module's only **process-wide singleton**, and this example shuts the real one
down. That is safe for one reason: all 21 internal tests swap the coordinator behind an unexported
seam, so nothing else touches the live one. But the safety is invisible and easy to destroy — a
second example here would panic on `Register` after this one's shutdown, depending on order — so the
file carries a note telling the next contributor to keep it the only example in the package. I
checked the coupling with `-shuffle` on four seeds instead of trusting the argument, and the two
`slog.Default` swaps (Recoverer's log sink, `FromContext`'s base logger, both restored) were checked
the same way.

## The lint finding worth keeping

revive flagged an unused `ctx` in a health probe, and the fix was not to hide the parameter. A probe
that ignores its context is exactly the bug the example should not model, so the database probe now
`return ctx.Err()` — nil while the request is alive — with a comment pointing at what a real one does
(`db.PingContext(ctx)`). The linter asked for a rename and the right answer was to use the parameter.

## State

Milestone 14 is **3 of 12**; **26 examples** in the module now — 13 from 14.2, 12 here, and
`validator.ExampleStruct`, which was the whole set a week ago. `master` is at `2c44c45` (14.2). No
surface, behaviour or dependency change: 0 golangci-lint issues, gofumpt clean, whole module green
including shuffled runs, four policy tools green.

Next: **14.4**, the config/storage/validation/core set, which inherits the two cases ADR-0053 already
named — `config.Load` needs a file written to a temp dir, `hash` costs ~230 ms per call at the new
default so an example may hash exactly once, and `db.Transaction` needs a `*sql.DB` the module
deliberately has no driver for, which is a decision to make in the item rather than a problem to
discover in it.

Still open for the maintainer: the two branch-protection flags
(`required_linear_history`, `required_conversation_resolution`).
