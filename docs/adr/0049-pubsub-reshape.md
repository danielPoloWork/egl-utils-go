# ADR-0049: the pubsub reshape — context-scoped subscriptions, a reporting `Publish`, and an explicit slow-subscriber policy

- **Status:** Accepted
- **Date:** 2026-07-29
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** ADR-0006 (the pubsub design — superseded on the `Subscribe`/`Publish` signatures and
  the fixed drop-newest policy; its *invariants* survive intact), ADR-0039 (drop-oldest and its
  best-effort reasoning, folded into the policy enum), ADR-0030 §2 ledger item 2, ADR-0025 (the
  no-hidden-timeout ruling this decision leans on), ADR-0045 (the `/v2` boundary), ADR-0037 (NFR-03,
  whose benchmark asserts the accounting invariant); spec §2 feature 2, §5; ROADMAP 13.5

## Context

Ledger item 2 is the largest single entry in ADR-0030 §2 and the only one that reopens an
*invariant* rather than a name. The gap analysis states the target as
`Subscribe(ctx, filter)` auto-removed on ctx cancel, `Publish(ctx, T) error`, `ErrSlowSubscriber`,
and an **explicit slow-subscriber policy (bounded buffer, drop-oldest or disconnect)**.

Three things about the v1 shape drove it:

**The unsubscribe function was a second lifetime to manage.** `Subscribe` returned
`(<-chan T, func())`, so every caller held a channel *and* a closer, and a subscription whose owner
forgot the `func()` stayed registered for the life of the broker. Meanwhile essentially every caller
already had a `context.Context` describing exactly the lifetime the subscription should have.

**`Publish` was silent by construction.** ADR-0006 gave it neither a context nor an error return —
which was not an oversight but the *mechanism* of its central promise: with no error to return and
no context to honour, `Publish` provably cannot block, and drop-newest was the only policy
expressible. The cost was that a publisher could not learn that anything had gone wrong. Only a
`WithDropHandler` installed at construction, on a different goroutine's terms, could see it.

**The policy was one bit and then two options.** 10.12 added `WithDropOldest[T]()` (ADR-0039) as an
additive boolean-shaped option. A third behaviour — shedding the slow subscriber entirely — cannot
be expressed by adding a second boolean without also making two illegal combinations expressible.

The hard question, and the one to settle before writing any code: **`Publish(ctx) error` looks
exactly like a method that may block or fail.** Every drop policy, the accounting invariant, and
NFR-03's 6.3 M-delivered/s figure hang off it not doing so.

## Decision

### 1 · `Publish` gains a context and an error, and still never blocks

```go
func (b *Broker[T]) Publish(ctx context.Context, topic string, msg T) error
```

**"Publish never blocks" survives unchanged, and is now a documented promise rather than a
consequence of an impoverished signature.** The two new parameters are given exact, narrow meanings
so that they cannot erode it:

- **`ctx` is consulted exactly once, before anything is delivered.** If it is already cancelled, no
  subscription receives the message and `ctx.Err()` is returned. Publish never *waits* on `ctx` —
  there is nothing to wait for. So cancellation is all-or-nothing rather than a partial fan-out, and
  the context is an admission check, not a deadline.
- **`error` is a report about what already happened, never a signal that the publisher waited.**
  `ErrClosed` on a closed broker; `ErrSlowSubscriber` when at least one matching subscription lost a
  message under the configured policy; `nil` otherwise.

A topic with no subscribers is **not** an error. In publish-subscribe, nobody listening is a normal
state; returning an error there would make the common case of a not-yet-started consumer look like a
failure.

`ErrSlowSubscriber` deliberately says *that* this publish lost a message, not *which* or *whose*.
Publish fans out to N subscriptions and can lose messages at several of them; a single sentinel is
the honest summary. **`WithDropHandler` answers "what was lost", `ErrSlowSubscriber` answers "did
this publish lose anything"** — the two are complementary, and the second is the one available
synchronously to the publisher.

### 2 · A subscription's lifetime is its context

```go
func (b *Broker[T]) Subscribe(ctx context.Context, topic string, filter func(T) bool) <-chan T
```

The returned `func()` is gone: **cancelling the context is the unsubscribe.** Cancellation
unregisters the subscription and closes its channel, so the channel closing — not `cancel()`
returning — is the definitive end of the subscription. Removal is prompt but asynchronous, and a
message published concurrently with cancellation may still be buffered; that is stated in the godoc
rather than papered over with a lock that would put cancellation on the delivery path.

**`context.AfterFunc` is what keeps ADR-0006's best property.** It registers a callback *on* the
context instead of parking a goroutine on `Done`, so N live subscriptions still cost **zero
goroutines** and "the broker owns none — it is leak-free by construction" remains literally true.
Registering the watcher while holding the write lock is safe because the `context` package always
runs the callback on its own goroutine, never inline: a cancellation landing in that instant leaves
`remove` blocked on the mutex until `Subscribe` returns.

Two edge cases collapse into one: **subscribing with an already-cancelled context, or to a closed
broker, returns an already-closed channel.** The subscription cannot live, and an already-closed
channel is the one result a `range` loop handles correctly without the caller testing for it — the
alternative, a channel that never delivers and never closes, hangs the consumer.

A subscription made with a context that cannot be cancelled (`context.Background()`) lives until
`Close`. That is not a leak but it *is* the shape that used to be a leak, so it is documented
explicitly.

### 3 · The policy becomes an enum, and gains `Disconnect`

`WithDropOldest[T]()` is replaced by `WithSlowSubscriberPolicy[T](p SlowSubscriberPolicy)` over a
three-valued enum:

| Policy | On a full buffer | For |
| --- | --- | --- |
| `DropNewest` (zero value, default) | discard the incoming message | event-like streams, every message independently meaningful |
| `DropOldest` | evict the oldest buffered message to fit the new one | state-like streams, where a later message supersedes an earlier one |
| `Disconnect` | unregister the subscription and close its channel | when a subscriber that has fallen behind is better shed than served |

**An enum rather than more booleans, because the enum makes the illegal states unrepresentable.**
Two independent options would admit "drop-oldest *and* disconnect", which has no meaning, and would
require a documented precedence rule — a rule readers must learn and the compiler cannot enforce.
`DropNewest` is the zero value, so the default is unchanged and a zero-configuration broker behaves
exactly as it did.

An **unknown** policy panics at construction (ADR-0005's loud-failure policy): the enum exists so an
illegal combination cannot be expressed, and an out-of-range integer is the one way left to try.

**`Disconnect` uses the buffer as its tolerance** — the first message a subscription cannot take
disconnects it. This is deliberate: a second threshold ("disconnect after K misses") is another knob
to tune and another number to justify, when `WithSubscriberBuffer` already expresses "the burst this
subscriber should survive". What was already buffered stays **receivable**, since closing a channel
does not discard its contents, so a shed subscriber can still drain what it accepted. It cannot
distinguish a disconnect from a normal shutdown — both arrive as a closed channel — which is stated
plainly as a cost rather than solved with a second channel or a sentinel value.

**Policy is per broker, not per subscription.** A consumer needing two policies over one stream runs
two brokers. Per-subscription policy would put a per-subscriber branch on the hot fan-out path to
serve a case nobody has asked for, and it remains additive later.

### 4 · The accounting invariant is unchanged, and now spans three policies

**While a subscription is registered, each message published to it is either delivered or reported
to the drop handler, exactly once.** Under `DropOldest` the message reported is the one *evicted*,
not the incoming one — reporting the incoming message would count safely delivered messages as
drops and break the arithmetic (ADR-0039), and an evicted message was buffered but never received,
so it is genuinely lost exactly once. Under `Disconnect` the undeliverable message is reported and
then the subscription ends. NFR-03's benchmark asserts `delivered + dropped == subscribers ×
publishes` and so keeps this honest across all three.

ADR-0039's **best-effort-by-construction** caveat for `DropOldest` carries over verbatim: making
room is evict-then-send, a subscriber or another publisher can act between the two steps, and a lost
race degrades that message to `DropNewest` rather than retrying without a bound. Retry-until-success
has no bound, and an unbounded retry inside `Publish` is exactly the thing this ADR is careful not
to permit.

### 5 · `topic` stays, against the ledger's literal signature

The gap analysis and spec v2's table both write `Subscribe(ctx, filter)` and `Publish(ctx, T)` — no
`topic`. **This is read as shorthand, and topics are kept.** The same tie-breaker ADR-0048 used for
`Close`'s context applies: the gap column flags the *API shape* (context, error return, sentinel,
policy) and never mentions topics, while topic-based routing is part of spec §2 feature 2, appears
in every §5 signature, and is the substance of the Publish-Subscribe entry in the patterns
catalogue. Removing it would be a **feature deletion wearing a signature change's clothes**, and
would push topic routing into either the filter (every subscriber pays a comparison for every
message on every topic) or into one broker per topic (the registry, moved to the caller).

## Alternatives Considered

- **Keep `Publish(topic, msg)` with no error and no context.** The status quo, and it has the real
  virtue that non-blocking is *unarguable* when there is nothing to return. Rejected because the
  publisher's inability to learn that delivery failed is the actual complaint behind ledger item 2,
  and the drop handler serves the broker's owner, not the publisher.
- **Make `Publish` block, or wait on `ctx`, when a buffer is full.** This is what the new signature
  *appears* to offer and is the one reading that must be foreclosed. Rejected: it converts every
  slow subscriber into backpressure on every publisher, so one stuck consumer stalls the producer —
  the failure mode a non-blocking broker exists to prevent — and it would put an unbounded wait or a
  hidden timeout on the delivery path, which ADR-0025 refused on principle.
- **Keep the `func()` unsubscribe alongside the context.** Rejected: two ways to end one
  subscription, and the interesting question ("which one closed the channel?") has no useful answer.
  `cancel()` on a `context.WithCancel` *is* the returned-closer shape, already in the caller's hands.
- **Park a goroutine per subscription on `ctx.Done()`.** The obvious implementation, and it would
  destroy the property that makes this broker cheap: N subscriptions would cost N goroutines and the
  package could no longer claim to own none. `context.AfterFunc` gives the same semantics for free.
- **A per-subscription mutex so cancellation is synchronous.** Rejected: it adds a lock to every
  delivery to make `cancel()` returning — rather than the channel closing — the end of the
  subscription, which is a guarantee no caller needs when `range ch` already terminates correctly.
- **`error` from `Subscribe` instead of an already-closed channel.** Rejected: it forces an error
  check on the ordinary path to report a condition (`broker closed`) the caller usually cannot act
  on, and a closed channel already communicates "this delivers nothing further" in the vocabulary
  the caller is about to use.
- **Two booleans (`WithDropOldest` + `WithDisconnectSlow`).** Rejected: see Decision 3 — it makes an
  illegal combination expressible and needs a precedence rule the compiler cannot check.
- **A `Disconnect` threshold of K missed messages.** Rejected: `WithSubscriberBuffer` is already
  that number, expressed once.

## Consequences

- **Breaking, on every call site in the package, all at compile time:**
  `br.Publish(topic, msg)` → `br.Publish(ctx, topic, msg)` (and the error is now available);
  `ch, unsub := br.Subscribe(topic, f)` → `ch := br.Subscribe(ctx, topic, f)`, with the
  `unsub()` call replaced by cancelling `ctx`; `pubsub.WithDropOldest[T]()` →
  `pubsub.WithSlowSubscriberPolicy[T](pubsub.DropOldest)`.
- **A caller who ignores the new error gets exactly v1's behaviour**, so the migration can be done
  in two passes: make it compile, then decide where `ErrSlowSubscriber` matters.
- **ADR-0006 is superseded on the two signatures and the fixed policy, and on nothing else.** Its
  at-most-once delivery, non-blocking Publish, zero-goroutine broker, `RWMutex` lifecycle idiom
  (a channel is only ever closed under the write lock, so a send on a closed channel is provably
  impossible), per-topic registry, synchronous filters, and ordering guarantee all stand.
- **ADR-0039 is superseded on the option's *name and shape* only.** Its reasoning — why drop-oldest
  is best-effort, why the evicted message is the one reported — is load-bearing here and reproduced
  above rather than referenced away.
- **Two new exported sentinels** (`ErrSlowSubscriber`, `ErrClosed`) and a new exported enum
  (`SlowSubscriberPolicy`, `DropNewest`, `DropOldest`, `Disconnect`); `WithDropOldest` is gone and
  `WithSlowSubscriberPolicy` takes its place. `pubsub` now imports `errors` and `context`.
- **The zero-goroutine claim is preserved and is now load-bearing in a second place** — it is what
  makes context-scoped subscriptions affordable at all.
- **100% statement coverage retained**, with the new invariants pinned rather than assumed: a
  cancelled context ends a subscription; a cancelled publish delivers to nobody; a non-cancellable
  subscription lives until `Close`; `Disconnect` sheds the slow subscription, leaves its buffered
  messages receivable, and leaves other subscriptions alone; the default policy is the zero value;
  and per-policy accounting holds under concurrent publishers.
- **A rendezvous subscription (`WithSubscriberBuffer(0)`) is where the policies diverge most:**
  `DropOldest` is a no-op there (no buffered message to evict), while `Disconnect` kills the
  subscription on the first message the subscriber is not already waiting for. Both are pinned by
  tests, because both are surprising.

## References

- `pkg/pubsub/pubsub.go`, `pkg/pubsub/options.go`, `pkg/pubsub/pubsub_test.go`,
  `pkg/pubsub/slowsubscriber_test.go`, `pkg/pubsub/nfr_bench_test.go`.
- ADR-0006 — the design this reshapes; ADR-0039 — the drop-oldest reasoning it absorbs.
- ADR-0030 §2 — the ledger, item 2 discharged; ADR-0045 — the `/v2` boundary.
- `docs/specs/02_spec_v2_gap_analysis.md` row 2 — the target this implements.
