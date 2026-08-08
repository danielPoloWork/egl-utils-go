# 2026-08-08 — BUG-0002: the signal that was dropped because nobody was listening yet

The first work after Milestone 14 closed, and it is not a roadmap item: a defect found by the
release PR's own CI.

## What shipped

- **The fix** — `started := make(chan struct{}, 1)` at both sites in
  `examples/service/service_test.go`, and the comment above it corrected to say why the buffer is
  load-bearing.
- **[BUG-0002](../../../bugs/2026/08/BUG-0002-unbuffered-started-channel-deadlocks-two-examples-service-tests.md)**
  in the ledger, `status: fixed`.
- A `CHANGELOG` **Fixed** entry.

## The defect

`TestReadinessFailsWhileTheQueueIsFull` and `TestOrderIsShedWhenTheQueueIsFull` both did this:

```go
started := make(chan struct{})        // unbuffered
blocker := func(context.Context) {
    select {
    case started <- struct{}{}:       // non-blocking
    default:
    }
    <-release
}
```

**A non-blocking send on an unbuffered channel only delivers if a receiver is already parked.** If
the worker goroutine reached the send before the test goroutine reached `<-started`, `default` was
taken and the announcement vanished. The test then waited on `<-started` forever while the single
worker waited on `<-release`, which only the test's `defer` closes. Ten minutes later, `panic: test
timed out`.

The panic trace named both halves precisely — goroutine 21 parked at `service_test.go:183`
(`<-started`), goroutine 22 parked at `:176` (`<-release`), *past* the send it should have completed.
That second stack is the proof: the only way to reach line 176 is through `default`.

The `select`/`default` was not itself the error. `blocker` is submitted twice, and the second
invocation must not park on a channel nobody reads. The error was pairing that guard with **zero
buffer**, which converts "don't block on the second send" into "drop the first send if the receiver
is late". One character of capacity separates the two.

## The comment asserted the opposite of what the code did

> `// No sleeping and no timing assumption: the first task announces that a worker has dequeued it`

`select`/`default` **is** a timing assumption — it assumes the receiver arrives first — and it is
precisely the assumption that failed. The intent was right and worth keeping: an ordering guarantee
with no `time.Sleep`, which is what [ADR-0053](../../../adr/0053-runnable-examples-convention.md)
demands of this code. The mechanism did not implement the intent, and the comment's confidence is
part of why it survived review: it told the reader the hard thing had already been thought about.

A sleep would have been the wrong repair for the same reason — it converts a deadlock into a slow
flake, which is worse, because it stops failing loudly.

## Why it took two runs to notice

It failed on **#97** and then on **#98**, both times on a feature branch, both times passing on
`master`. Two things made the first one invisible.

**`examples / service` is still not a required status check.** Roadmap 14.5 recorded the trap when it
added the job — *adding a job does not add a required context* — and left it as an open hand-off. So
both pull requests merged with the job red, and the second was the `v2.0.1` **release PR**.

**And the watch I set on #97 timed out without reporting**, so I said the checks were running and
never confirmed the result. A monitor that times out is not a green result. The failure went
unremarked until the identical one recurred, which is the more useful half of the lesson: an
intermittent job that blocks nothing trains everyone to stop reading it.

Severity is `medium` rather than `low` for that reason alone. No consumer was ever affected —
`examples/service` is a separate module, never tagged, absent from the published `…/v2`.

## Verified how, and how far

A repository-wide sweep found **no third instance**: every other `select`/`default` in a `_test.go`
file is a non-blocking *receive* (`case <-stop:`, `case <-ch:`), which cannot lose a signal this way.
These two were the only non-blocking sends.

The bug ledger's gate was confirmed live on the new record by deliberate violation — blanking
`fixed-in` on a `fixed` record, and removing the index row — rather than by the green run.

**What cannot be verified here: the fix itself.** There is no Go toolchain on this machine, so CI
compiles and runs it. And CI is weak evidence for a flake fix in any case — a green run only fails to
disprove it, since the bug reproduced roughly one run in six. The argument that the fix is correct is
structural, not statistical: with capacity 1 the buffer is empty when the first send happens, so that
send cannot take `default`, so the signal cannot be dropped. There is no interleaving in which it
fails.

## Open

**Make `examples / service` a required status check.** The fix removes the flake; it does not close
the gap that let a red job ride through two merges. That is still a branch-protection change only the
maintainer can make.
