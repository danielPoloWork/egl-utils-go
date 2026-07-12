# ADR-0007: fanin design — forwarder-per-input, cancel-or-drain contract

- **Status:** Accepted
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 3, §5 (fanin API), §6; ROADMAP 2.3; patterns catalogue
  (Fan-In / Fan-Out); ADR-0005/0006 (house lifecycle idioms)

## Context

Feature 3 commits to "merge multiple input channels into a single output channel without
goroutine leaks", with the API fixed at intake: `Merge[T](ctx, ins ...<-chan T) <-chan T`.
The correctness bar is completeness (no value lost or duplicated), per-input ordering, and
leak-freedom under every exit path — including the adversarial one where a forwarder sits
blocked mid-send and the consumer walks away.

## Decision

The canonical Go-pipelines fan-in shape: one forwarder goroutine per input plus one closer
goroutine that waits on a `sync.WaitGroup` and closes the output. Every blocking point in a
forwarder — the receive and the send — carries a `ctx.Done()` select branch, so
cancellation unblocks a forwarder in any state. The output closes when all inputs are
closed and drained, or on cancellation, whichever comes first; zero inputs return an
already-closed channel. The output is unbuffered: a slow consumer exerts backpressure on
every input rather than hiding latency in a buffer. The **consumer contract** is explicit
and documented: drain the output until it closes or cancel ctx — abandoning the output
without canceling leaves forwarders parked on their next send (that is the caller's bug,
mirroring the task-ignores-context caveat of ADR-0005). A nil input channel panics loudly:
it could never contribute a value, only block a forwarder forever.

## Alternatives Considered

- **Single multiplexing goroutine over `reflect.Select`** — one goroutine regardless of
  input count, but reflection-based selects are an order of magnitude slower and the code
  stops being obviously correct. Rejected; N+1 cheap goroutines are the idiomatic price.
- **Buffered output (or a buffer option)** — hides backpressure and invites sizing
  bikeshed; the intake API exposes no options and callers can buffer downstream themselves.
  Rejected for v1.
- **Silently skipping nil inputs** — a nil channel in the slice is a caller bug; skipping
  it would convert a loud mistake into a silent one. Rejected (loud-by-default house rule).

## Consequences

- Goroutine cost is `len(ins) + 1`, all provably terminated by input-close or cancel — the
  leak guard plus the blocked-send cancellation test pin this mechanically.
- No cross-input ordering exists and none is promised; consumers needing global order must
  sequence upstream.
- The patterns catalogue row "Fan-In / Fan-Out" is **outside the series taxonomy**
  (design-patterns.md §6 has no such entry; nearest kin Producer-Consumer): like Functional
  Options in ADR-0005, it is catalogued under its community-canonical name — the Go blog's
  pipelines vocabulary — with the deviation recorded here. The row lists the merge half
  now; package fanout completes it at ROADMAP 2.4.

## References

- `docs/specs/01_spec_utils.md` §2.3, §5, §6.
- Go blog, "Go Concurrency Patterns: Pipelines and cancellation" (fan-in vocabulary).
- ADR-0005 (caller-bug caveat idiom), ADR-0006.
