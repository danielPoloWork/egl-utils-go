---
id: BUG-0002
title: A non-blocking send on an unbuffered channel drops the signal, deadlocking two examples/service tests until the 10-minute timeout
status: fixed
severity: medium
reporter: internal
discovered: 2026-08-08
affected-versions: "examples/service since 2026-08-04 (roadmap 14.5); the module is never tagged"
fixed-in: master (examples/service carries no tags — see Fix)
---

# BUG-0002: A non-blocking send on an unbuffered channel drops the signal, deadlocking two examples/service tests until the 10-minute timeout

## Summary

`TestReadinessFailsWhileTheQueueIsFull` and `TestOrderIsShedWhenTheQueueIsFull` in
`examples/service` deadlock intermittently. When they do, the test binary makes no progress until
Go's 10-minute panic timeout fires and the `examples / service` CI job fails. Nothing in the library
is involved: the defect is entirely in the tests' own synchronisation, and it is a race between the
worker goroutine and the test goroutine that is lost roughly one run in six.

## Environment

- **Affected versions:** `examples/service` since it was written (roadmap 14.5, 2026-08-04). The
  module has its own `go.mod`, is **never tagged**, and is not part of the published
  `github.com/danielPoloWork/egl-utils-go/v2` module, so **no consumer is affected**.
- **Toolchain / platform:** observed on Go 1.26.5, `ubuntu-24.04`, under `go test -race ./...`. The
  defect is scheduler-dependent, not platform-specific.
- **Configuration:** `cfg.Workers, cfg.QueueSize = 1, 1` — both tests deliberately use a single
  worker and a one-slot queue.

## Reproduction

Observed twice in CI, on two different branches, both times as a 10-minute hang:

| Run | Branch | Commit | `examples / service` |
|---|---|---|---|
| [31249725684](https://github.com/danielPoloWork/egl-utils-go/actions/runs/31249725684) | `release/v2.0.1` | `af07a04` | **failure** — `TestReadinessFailsWhileTheQueueIsFull` timed out |
| earlier run | `docs/repo-metadata-and-v0.1.0-release` | `db53e50` | **failure** |

The same commits' trees passed on `master`, which is what makes it a flake rather than a break:
the outcome depends on goroutine scheduling, not on the code under test.

The failing code, before the fix:

```go
started := make(chan struct{})        // unbuffered
release := make(chan struct{})
blocker := func(context.Context) {
    select {
    case started <- struct{}{}:       // non-blocking send
    default:                          // ... so the signal can be DROPPED
    }
    <-release
}
for i := range 2 {
    _ = svc.pool.Submit(context.Background(), blocker)
    if i == 0 {
        <-started                     // waits for a signal that may never come
    }
}
```

The panic trace names both halves of the deadlock exactly:

```text
panic: test timed out after 10m0s
running tests:
	TestReadinessFailsWhileTheQueueIsFull (10m0s)

goroutine 21 [chan receive]:
	...service.TestReadinessFailsWhileTheQueueIsFull(...)
		examples/service/service_test.go:183 +0x36d      // parked on <-started

goroutine 22 [chan receive]:
	...service.TestReadinessFailsWhileTheQueueIsFull.func1(...)
		examples/service/service_test.go:176 +0x45       // parked on <-release
	...workerpool.(*Pool).run(...)
```

Goroutine 22 is *past* the `started` send and parked on `<-release`, which is only reachable through
the `default` branch. That is the dropped signal, recorded in the trace.

## Expected vs. actual

- **Expected:** the first task announces that a worker has dequeued it; the test proceeds, fills the
  one-slot queue with a second submission, and asserts that the probe's submission is refused.
- **Actual:** the announcement is lost, the test parks on `<-started` forever, the single worker
  parks on `<-release` forever, and the run ends 10 minutes later with `panic: test timed out`.

## Root cause

**A non-blocking send on an unbuffered channel only delivers if a receiver is already parked on it.**
`select`/`default` makes the send non-blocking; an unbuffered channel makes delivery require a
rendezvous. Together they mean: if the worker goroutine reaches the send before the test goroutine
reaches `<-started`, there is no receiver, `default` is taken, and the signal is silently discarded.
Neither goroutine can then make progress — the test is waiting for a signal that will never be sent
again, and the worker is waiting on a `release` that is closed only by the test's `defer`.

The `select`/`default` itself is not the mistake: `blocker` is submitted twice, and the second
invocation must not park on a channel nobody reads. The mistake is pairing that guard with **zero
buffer**, which turns "don't block on the second send" into "drop the first send if the receiver is
late".

The comment above the code asserted the opposite of what the code did:

> `// No sleeping and no timing assumption: the first task announces that a worker has dequeued it`

`select`/`default` **is** a timing assumption — it assumes the receiver arrives first — and it is the
assumption that fails. The intent (an ordering guarantee with no sleeps) was right; the mechanism
did not implement it.

## Impact

- **Consumers: none.** `examples/service` is a separate, never-tagged module. It ships to nobody, and
  the published `…/v2` module contains none of this code.
- **CI: a 10-minute stall and a red job**, roughly one PR run in six. It cost two pull requests a red
  check (#97 and #98) and burned the full timeout budget each time.
- **Signal erosion, which is the reason this is `medium` and not `low`.** Because
  `examples / service` was **not a required status check** (recorded as an open hand-off since roadmap
  14.5 — "adding a job does not add a required context"), both pull requests merged with it red,
  and the second was the `v2.0.1` **release PR**. A job that fails intermittently and blocks nothing
  trains a reader to ignore it, which is exactly what happened: the failure on #97 went unremarked
  until the identical failure recurred on #98.

  > *Closed 2026-08-08: `examples / service` **is** a required status check now — `master` went from
  > thirteen contexts to fourteen. Deliberately in this order: the flake was fixed first, because
  > making an intermittently-failing job required converts an ignorable red into a blocked
  > repository.*

## Fix / workaround

**One character of buffer capacity**, at both sites:

```go
started := make(chan struct{}, 1)
```

With capacity 1 the first send always succeeds — the buffer accepts it whether or not the receiver
has arrived — so the announcement cannot be lost. The `select`/`default` guard keeps doing its real
job: the second invocation of `blocker` finds the buffer occupied (or unread) and falls through
instead of parking. The delivery is now ordered by the channel rather than by the scheduler, which is
what the comment claimed all along; the comment is corrected to say why the buffer is load-bearing.

Nothing else changes: no production code, no test assertion, no timing constant, and no `time.Sleep`
was added — a sleep would have converted a deadlock into a slow flake, which
[ADR-0053](../../../adr/0053-runnable-examples-convention.md) rejects for exactly this class of code.

**`fixed-in` is `master`, not a version.** `examples/service` carries no tags
([ADR-0054](../../../adr/0054-examples-service-module.md)), so there is no release for the fix to
land in; the fixing commit on `master` is the only meaningful coordinate. Recording a core version
here would claim the defect had shipped to consumers, and it never did.

**A repository-wide sweep found no third instance.** Every other `select`/`default` in a `_test.go`
file is a non-blocking *receive* (`case <-stop:`, `case <-ch:`), which cannot drop a signal the same
way, and the two sites fixed here are the only non-blocking *sends*.

## References

- Fixing PR: #99
- `CHANGELOG` entry: `[Unreleased]` → **Fixed**
- Failing run: [31249725684](https://github.com/danielPoloWork/egl-utils-go/actions/runs/31249725684)
  (job `examples / service`, step "Build, vet and test under the race detector")
- Related: [ADR-0054](../../../adr/0054-examples-service-module.md) (the module's boundary, why it is
  never tagged, and the required-context hand-off — **closed 2026-08-08**, after this fix),
  [ADR-0053](../../../adr/0053-runnable-examples-convention.md) (no example or its test may depend on
  the clock)
