# ADR-0008: fanout design — forwarder-per-output, exactly-once distribution

- **Status:** Accepted
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 4, §5 (fanout API), §6; ROADMAP 2.4; patterns catalogue
  (Fan-In / Fan-Out); ADR-0007 (the fan-in dual, whose vocabulary this completes)

## Context

Feature 4 commits to "distribute messages from one source channel to multiple destination
channels in parallel", with the API fixed at intake:
`Split[T](ctx, in <-chan T, outs ...chan<- T)`. The verb *distribute* admits two readings —
**broadcast** (every value to every destination) or **work distribution** (every value to
exactly one destination) — and the choice is one-way once published, so it is recorded
here rather than left as an undocumented judgment call. The correctness bar is
completeness (no value lost or duplicated, spec §6), leak-freedom under every exit path,
and useful behavior when destinations run at different speeds.

## Decision

**Exactly-once work distribution**, on three convergent grounds: the Go blog's pipelines
vocabulary — which ADR-0007 adopted for this pattern pair — defines fan-out as multiple
consumers reading from one channel; spec §6's completeness property says "no message lost
*or duplicated*"; and broadcast semantics would duplicate `pubsub.Broker`'s concern,
violating the spec's one-concern-per-package architecture.

The shape is the exact dual of ADR-0007: **one forwarder goroutine per output**, each
competing to receive from the shared input and owning its output. Channel receive
semantics give exactly-once delivery by construction, and each output observes an
input-order subsequence (a single goroutine's successive receives from one channel see
values in send order). A value lands on whichever destination is ready first — natural
load balancing; **no round-robin or fairness is promised**. Every blocking point carries a
`ctx.Done()` select branch. `Split` returns immediately and takes **send-ownership of the
outputs**: each forwarder closes its output when the input closes or ctx is canceled —
with no return value, closing the outputs is the caller's only completion signal. Zero
outputs make `Split` a no-op that never reads the input (with nowhere to deliver, draining
would silently lose values). A nil input or output panics loudly. On cancellation a value
already in a forwarder's hand is dropped — cancellation abandons in-flight work, exactly
as in fanin.Merge.

## Alternatives Considered

- **Broadcast (every value to every output)** — the AMQP "fanout exchange" reading.
  Rejected: duplicates pubsub.Broker, contradicts spec §6's no-duplication property and
  the Go-pipelines vocabulary already committed to in ADR-0007.
- **Round-robin distributor goroutine** — deterministic fairness, but one slow destination
  head-of-line-blocks the rotation and therefore every other destination. The
  ready-first race keeps fast consumers fed. Rejected.
- **Single multiplexing goroutine over `reflect.Select`** — rejected for the same reasons
  as in ADR-0007: an order of magnitude slower, no longer obviously correct.
- **Silently skipping nil outputs** — converts a loud caller bug into a silent one.
  Rejected (loud-by-default house rule, ADR-0005/0007 lineage).

## Consequences

- Goroutine cost is `len(outs)` — no closer goroutine needed, since each forwarder closes
  the output it owns — all provably terminated by input-close or cancel; the leak guard
  plus the blocked-send cancellation test pin this mechanically.
- No fairness guarantee: a fast consumer may take arbitrarily more values than a slow one.
  Callers needing balanced work counts want workerpool.Pool, not fanout.
- Completeness holds on the input-closed path (what the property tests pin);
  cancellation deliberately abandons at most `len(outs)` in-flight values.
- Handing `Split` the outputs transfers their send side: the caller must not send on or
  close them afterward. The godoc states this contract explicitly.
- The patterns catalogue row "Fan-In / Fan-Out" (ADR-0007 records the outside-taxonomy
  name) is now complete: both halves implemented.

## References

- `docs/specs/01_spec_utils.md` §2.4, §5, §6.
- Go blog, "Go Concurrency Patterns: Pipelines and cancellation" (fan-out vocabulary).
- ADR-0005 (loud-by-default idiom), ADR-0006 (pubsub broadcast concern), ADR-0007.
