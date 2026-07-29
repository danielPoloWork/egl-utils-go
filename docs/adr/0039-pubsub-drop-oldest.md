# ADR-0039: `pubsub.WithDropOldest` — an opt-in slow-subscriber policy, best-effort by construction

- **Status:** Accepted — the **option's name and shape** are superseded by
  [ADR-0049](0049-pubsub-reshape.md): `WithDropOldest[T]()` becomes
  `WithSlowSubscriberPolicy[T](DropOldest)`, one value of a three-valued enum that also carries
  `Disconnect`. **Everything else here stands, and the reasoning is load-bearing in ADR-0049 rather
  than replaced by it** — why drop-oldest is best-effort by construction, why retry-until-success is
  inadmissible, and why the *evicted* message is the one reported to the drop handler.
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo)
- **Related:** ADR-0006 (the pubsub design: non-blocking Publish, drop-newest, the drop handler), ADR-0030 (spec v2 reconciliation: 10.12 adopted), ADR-0037 (NFR-03, whose benchmark asserts the accounting invariant), spec v2.0 item 2, roadmap 10.12

## Context

ADR-0006 settled that `Publish(topic, msg)` never blocks: it carries neither a context nor an error
return, so a subscription whose buffer is full at delivery time simply misses that message, with
`WithDropHandler` there to observe it. That is drop-newest, and it is the right default for an
event-like stream where each message is independently meaningful and the earliest pending work should
not be discarded.

It is the wrong choice for a state-like stream. When a later message supersedes an earlier one — a
metrics gauge, a price tick, a progress percentage — keeping the oldest buffered messages means the
subscriber catches up through a queue of values that are already wrong, and the freshest value is the one
thrown away. Spec v2 item 2 asks for the alternative, and ADR-0030 placed it in the additive bucket as
roadmap 10.12, with the default explicitly unchanged.

The mechanism is where the design work is. Evicting the oldest message from a Go channel is not one
operation: it is a non-blocking receive followed by a non-blocking send, and **a subscriber or another
publisher can act between the two**.

## Decision

`WithDropOldest()` is an opt-in broker-level `Option` that, on a full buffer, discards the **oldest
buffered** message to make room for the new one. The drop handler receives the message that was actually
lost — under drop-oldest that is the **evicted** message, not the incoming one. The policy is
**best-effort by construction**: the evict-then-send sequence is attempted once, and if a concurrent
publisher refills the buffer in the window the new message is dropped instead, degrading to drop-newest
for that message rather than retrying without bound.

The invariant that survives both policies, and is asserted: per subscription, every published message is
either delivered or reported to the drop handler, **exactly once**.

## Alternatives Considered

- **Retrying the evict-and-send until it succeeds.** The intuitive reading of "drop oldest", and it would
  make the policy exact rather than best-effort. Rejected because it has no bound: under sustained
  publish contention a publisher could spin arbitrarily long, which breaks the one guarantee ADR-0006
  actually froze — that `Publish` never blocks. A bounded single attempt keeps that promise and makes the
  failure mode (one message treated as drop-newest) both predictable and reportable.
- **Taking a per-subscription lock around evict-and-send** to make the pair atomic. Rejected as
  disproportionate: it adds a lock to the delivery hot path of every subscription to remove a race whose
  only consequence is that one message is dropped by the other policy — while the whole point of the
  feature is *which* message gets dropped when messages are already being lost.
- **Reporting the incoming message to the drop handler** instead of the evicted one, keeping the
  handler's meaning literally identical to before ("the message that arrived when the buffer was full").
  Rejected because it would lie about what was lost: under drop-oldest the incoming message *is*
  delivered, and a consumer counting drops or logging payloads would be counting messages that arrived
  safely while the genuinely lost ones went unreported. The accounting invariant would break too. The
  handler's docs now state which message it gets under each policy, since the distinction matters to
  anyone logging the payload.
- **A per-subscription policy**, passed to `Subscribe`. More expressive — different consumers of one
  topic may genuinely want different policies. Rejected because `Subscribe(topic, filter)` is frozen by
  the v1 API-stability commitment, and a variadic or a second method would be a larger surface than the
  feature earns. A consumer needing both policies on one stream can run two brokers.
  > *Annotated 2026-07-29 (ADR-0049): the stated reason has expired but the conclusion held.*
  > `Subscribe` is no longer frozen — ADR-0049 changed its signature at the `/v2` boundary — so
  > "frozen by the v1 commitment" is no longer available as a reason. Per-subscription policy is
  > still rejected there, on the surviving half of this entry: it would put a per-subscriber branch
  > on the hot fan-out path for a case nobody has asked for, and two brokers still serve it. It
  > remains additive later.
- **An enum option** (`WithDropPolicy(DropNewest|DropOldest)`) instead of a boolean flag option.
  Rejected as more surface for the same expressiveness: it would export a policy type and two constants
  to say what one nullary option says, and the default is already the documented behaviour rather than
  something a caller should have to name.
  > *Annotated 2026-07-29 (ADR-0049): **this alternative was subsequently adopted**, and the
  > reasoning here was sound for the world it was written in.* The argument was "more surface for the
  > same expressiveness" — true while there were exactly **two** policies, where a nullary option and
  > a two-valued enum say precisely the same thing. ADR-0049 adds a third (`Disconnect`), and at
  > three the accounting inverts: independent boolean options make illegal combinations
  > ("drop-oldest *and* disconnect") expressible and require a precedence rule the compiler cannot
  > enforce, which the enum removes. So `WithSlowSubscriberPolicy` is not a reversal of this
  > judgement but the same judgement applied to a different number of policies — worth recording,
  > because a future reader finding an enum where this ADR rejected one would otherwise conclude the
  > rejection was simply wrong.
- **Blocking, or an unbounded per-subscription queue,** as ways to avoid dropping at all. Both were
  already rejected in ADR-0006 and nothing here changes that reasoning: blocking couples publishers to
  the slowest subscriber, and an unbounded queue converts a bounded loss into unbounded memory.

## Consequences

- Additive: a new `Option`, no signature change, and the default policy is untouched — a broker
  constructed without the option behaves exactly as before, which a test asserts side by side with the
  new behaviour.
- **The extra cost is confined to the path where messages are already being lost.** Measured on a
  saturated, undrained subscription (median of 5): drop-newest **74.9 ns/op**, drop-oldest
  **138.4 ns/op** — about 63 ns for the additional receive and send. The delivering path is untouched, and
  NFR-03's fan-out throughput is unchanged at ~6.4 M delivered/s.
- **The accounting invariant holds under both policies**, which is what lets NFR-03's benchmark keep
  asserting `delivered + dropped == subscribers × publishes`. An evicted message was buffered but never
  received, so it is reported exactly once and never double-counted as both delivered and dropped.
- Ordering among survivors is preserved: channel eviction is FIFO, so the messages left in the buffer
  stay in publication order — the guarantee ADR-0006's package doc makes about sequential publishes from
  one goroutine still holds for whatever survives.
- Receiving from a subscription channel inside `Publish` is safe because `Publish` holds the broker's read
  lock and a channel is only ever closed under the write lock — the same lifecycle argument ADR-0006 used
  to prove a send on a closed channel is impossible. Worth stating because "the publisher reads from the
  subscriber's channel" looks alarming until that is spelled out.
- The option is a **no-op for a rendezvous subscription** (`WithSubscriberBuffer(0)`): there is nothing
  buffered to evict, so behaviour is drop-newest whatever the option says. Documented and tested rather
  than left as a surprise.
- `pubsub` reaches **100% coverage**, up from 96.4% — the two previously uncovered statements were the
  option validators' panic branches, which this change tests.
- Deferred: a per-subscription policy, if a consumer presents the need.

## References

- Spec v2.0 item 2; ADR-0006 (non-blocking `Publish`, drop-newest, drop handler — extended here),
  ADR-0030 (adoption bucket), ADR-0037 / NFR-03 (the benchmark asserting the accounting invariant).
- `pubsub/options.go` (`WithDropOldest`, `WithDropHandler`'s amended docs), `pubsub/pubsub.go`
  (`deliver`), `pubsub/droppolicy_test.go`.
