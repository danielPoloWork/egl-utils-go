# 2026-07-29 — 13.5: the pubsub reshape, or how to add a context and an error without letting Publish block

**The pubsub API is reshaped: subscriptions are context-scoped, `Publish` takes a context and returns
an error, and the slow-subscriber policy is an explicit three-valued enum.** ADR-0049 supersedes the
two signatures and the fixed drop-newest policy of ADR-0006, and the option *name* of ADR-0039.
Ledger item 2 discharged — the largest entry in ADR-0030 §2, and the only one in Milestone 13 that
reopens an **invariant** rather than a name.

## The question the brief said to answer first

The ROADMAP line for 13.5 ends *"decide what `error` means before writing any of it"*, and that was
genuinely the whole item. `Publish(ctx, topic, msg) error` looks exactly like a method that may block
or fail. Everything in the package hangs off it not doing so: the drop policies, the accounting
invariant, and NFR-03's 6.3 M-delivered/s.

The sharper way to put the problem: **ADR-0006 did not so much promise that `Publish` never blocks as
make it unarguable.** With no context to honour and no error to return, there was nothing a blocking
implementation could have been *for* — drop-newest was the only expressible policy. Adding both
parameters removes that structural guarantee and replaces it with a promise someone has to keep.

So both parameters were given deliberately narrow meanings:

- **`ctx` is consulted exactly once, before anything is delivered.** Already cancelled → nobody
  receives the message, `ctx.Err()` comes back. Publish never *waits* on it; there is nothing to wait
  for. Cancellation is therefore all-or-nothing rather than a partial fan-out, and the context is an
  admission check, not a deadline.
- **`error` reports what already happened, never that the publisher waited.**

The reading that had to be foreclosed explicitly, because it is the tempting one: a `Publish` that
blocks or waits on `ctx` when a buffer is full converts every slow subscriber into backpressure on
every publisher. One stuck consumer stalls the producer — the precise failure a non-blocking broker
exists to prevent — and it would put either an unbounded wait or a hidden timeout on the delivery
path, which ADR-0025 refused on principle.

## What `ErrSlowSubscriber` can honestly say

Publish fans out to N subscriptions and can lose messages at several of them, so a sentinel cannot
name a victim. It says *that* this publish lost a message, not which or whose. That is a real
division of labour rather than a limitation:

- **`WithDropHandler` answers "what was lost"** — asynchronously, on the broker owner's terms.
- **`ErrSlowSubscriber` answers "did this publish lose anything"** — synchronously, to the publisher,
  which is the party that previously could not find out at all.

A topic with **no subscribers is not an error**. In publish-subscribe, nobody listening is a normal
state; erroring there would make a not-yet-started consumer look like a failure and would train
callers to ignore the return value — which would cost the one thing this change bought.

## Cancel is the unsubscribe

`Subscribe` returned `(<-chan T, func())`. Two things to hold, and a subscription whose owner dropped
the `func()` stayed registered for the life of the broker — while essentially every caller already
had a `context.Context` describing exactly the lifetime the subscription should have.

Now it returns only a channel, and cancelling the context unregisters the subscription and closes it.
Three details worth having written down:

**`context.AfterFunc` is what kept ADR-0006's best property.** A goroutine per subscription parked on
`Done` is the obvious implementation and would have made N subscriptions cost N goroutines, ending
"the broker owns no goroutines — leak-free by construction". `AfterFunc` registers a callback *on* the
context instead, so context-scoped subscriptions are free. The zero-goroutine claim is now
load-bearing in a second place rather than merely being a nice property.

**Registering the watcher under the write lock is safe, for a reason worth stating.** The `context`
package always runs the callback on its own goroutine, never inline, so a cancellation landing in that
instant leaves `remove` blocked on the mutex until `Subscribe` returns.

**Two edge cases collapse into one.** Subscribing with an already-cancelled context, or to a closed
broker, both return an **already-closed channel** — the one result a `range` loop handles correctly
without the caller testing anything. The alternative, a channel that never delivers and never closes,
hangs the consumer. Returning an `error` instead was rejected: it forces a check on the ordinary path
to report a condition the caller usually cannot act on.

The honest cost, documented rather than hidden: removal is prompt but **asynchronous**, so a message
published concurrently with cancellation may still be buffered, and the channel closing — not
`cancel()` returning — is the definitive end. A per-subscription lock would make it synchronous by
putting a lock on every delivery, to provide a guarantee no `range ch` caller needs.

## Scope grew, on the gap analysis's authority

The ROADMAP line names ctx-subscription, `Publish(ctx) error` and `ErrSlowSubscriber`. The gap
analysis row 2 requires more: *"explicit slow-subscriber policy (bounded buffer, **drop-oldest or
disconnect**)"*. So `WithDropOldest[T]()` became `WithSlowSubscriberPolicy[T](p)` over three values.

**An enum, not a second boolean.** Two booleans make "drop-oldest *and* disconnect" expressible —
meaningless, and requiring a documented precedence rule readers must learn and the compiler cannot
check. The enum makes the illegal state unrepresentable, which leaves an out-of-range integer as the
only way left to try; that panics at construction, per ADR-0005.

`DropNewest` is the **zero value**, so a zero-configuration broker behaves exactly as before.

**`Disconnect` uses the buffer as its tolerance.** The first message a subscription cannot take
disconnects it. A "disconnect after K misses" threshold is another knob to tune and another number to
justify, when `WithSubscriberBuffer` already expresses "the burst this subscriber should survive".
Two consequences stated as costs rather than solved: what was already buffered stays **receivable**
(closing a channel does not discard its contents), and a disconnected subscriber **cannot tell a
disconnect from a normal shutdown** — both arrive as a closed channel. A second channel or a sentinel
value would fix the second one by complicating every consumer.

Policy is **per broker**. Two policies over one stream means two brokers; a per-subscription branch on
the hot fan-out path to serve a case nobody has asked for is additive later if anyone does.

## `topic` stays, and the tie-breaker was 13.4's

Both the gap analysis and spec v2's table write `Subscribe(ctx, filter)` and `Publish(ctx, T)` — no
`topic`. Read as shorthand, exactly as 13.4 read `Close() error` as shorthand for a `Close` that keeps
its context, and for the same reason: **the gap column flags the API *shape* — context, error return,
sentinel, policy — and never mentions topics**, while topic-based routing is part of §2 feature 2,
appears in every §5 signature, and is the substance of the Publish-Subscribe entry in the patterns
catalogue.

Removing it would be a **feature deletion wearing a signature change's clothes**, and it would not
even remove the work: routing would move into the filter (every subscriber pays a comparison for every
message on every topic) or into one broker per topic (the registry, relocated to the caller).

That is now twice in two items that the ledger's terse signature was not the ledger's decision. The
rule that has emerged: **when the ROADMAP and the gap analysis disagree, the gap analysis wins
(13.3, 13.4); when the gap analysis's *signature* and its own *gap column* disagree, the gap column
wins, because that is where it recorded what it was actually objecting to.**

## What ADR-0039 contributed rather than lost

ADR-0039 is superseded on its option's name and shape only. Its reasoning is load-bearing here and is
reproduced in ADR-0049 rather than referenced away:

- **Drop-oldest is best-effort by construction.** Making room is evict-then-send, and a subscriber or
  another publisher can act between the two steps. A lost race degrades that message to drop-newest.
  Retry-until-success has no bound — and an unbounded retry inside `Publish` is exactly what this ADR
  is careful not to permit.
- **The message reported to the drop handler is the *evicted* one.** Reporting the incoming message
  would count safely delivered messages as drops and break the arithmetic.

That arithmetic — **while a subscription is registered, each message published to it is either
delivered or reported to the drop handler, exactly once** — now spans three policies, and NFR-03's
benchmark still asserts `delivered + dropped == subscribers × publishes`, which is what keeps it
honest rather than aspirational.

## Verification

- **100% statement coverage retained**, and unlike 13.4 this item *did* create new invariants, so new
  tests were warranted rather than decorative: a cancelled context ends a subscription; a cancelled
  publish delivers to nobody; a non-cancellable subscription lives until `Close`; `Disconnect` sheds
  the slow subscription, leaves its buffered messages receivable, and leaves other subscriptions
  alone; the default policy is the zero value; per-policy accounting holds under concurrent
  publishers.
- **The two rendezvous cases are pinned because both surprise.** With `WithSubscriberBuffer(0)`,
  `DropOldest` is a no-op (nothing buffered to evict) while `Disconnect` kills the subscription on the
  first message the subscriber was not already waiting for.
- **Surface 135 → 139 identifiers**; `spec_api_lint.py` green, so §5 and `go doc` agree.
- **`nfr_bench_test.go` was the one file the reshape broke that tests did not cover.** It is a
  benchmark file, so `go test` on the package passed while `go vet ./...` was the thing that caught
  it — the same class as 13.1's hand-written CI paths: code that only a non-default command compiles
  is code the usual green tick does not cover.

## State

Milestone 13 is **5 of 10**. Next is 13.6 — removing the Prometheus SDK and `prometheus.Registerer`
from the metrics API — the only item in the milestone that changes the dependency graph, and the one
that has to keep ADR-0027's bounded-cardinality guarantees while dropping the vendor type from the
surface. 13.9 still must wait for the `v2.0.0` tag (ADR-0040).
