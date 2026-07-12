# ADR-0006: pubsub design — at-most-once buffered delivery, no broker goroutines

- **Status:** Accepted
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 2, §5 (pubsub API), §6; ROADMAP 2.2/2.6; ADR-0005 (shared
  lifecycle idiom); patterns catalogue (Publish-Subscribe)

## Context

Feature 2 commits to an "in-memory publish-subscribe broker over Go channels with filtered
subscriptions". The intake API is fixed: `NewBroker[T](opts ...Option) *Broker[T]`,
`Publish(topic string, msg T)` and `Subscribe(topic string, filter func(T) bool)
(<-chan T, func())`. `Publish` carries **neither a context nor an error return**, which
constrains the delivery policy: it cannot block indefinitely on a slow subscriber (no way
to cancel) and cannot report back-pressure failures (no error channel). The module NFRs
demand zero goroutine leaks and race-detector cleanliness.

## Decision

1. **Delivery policy: at-most-once, per-subscription buffer, drop-on-full.** `Publish`
   performs a non-blocking send to each matching subscription (default buffer 16,
   `WithSubscriberBuffer` to tune; 0 = rendezvous). A full buffer drops the message for
   that subscription only; `WithDropHandler` makes every drop observable. This is the only
   policy coherent with the fixed `Publish` signature.
2. **No broker goroutines.** Fan-out happens synchronously on the publishing goroutine —
   the broker is leak-free by construction, and the leak guard in tests proves the absence
   of accidental ones.
3. **Lifecycle safety, same idiom as ADR-0005.** Subscription channels are closed only
   under the registry's write lock; `Publish` sends only under the read lock — a send on a
   closed channel is provably impossible. `unsubscribe` is `sync.Once`-idempotent.
4. **`Close()` added as an additive surface.** The intake API listed no shutdown method,
   but the verification bar asserts leak-freedom "after Stop/Close/cancel". `Close` closes
   every subscription channel, clears the registry, turns `Publish` into a silent no-op and
   makes late `Subscribe` return an already-closed channel. Spec §5 is updated in the same
   PR (spec-sync rule).
5. **Exact-topic matching, v1.** Topics are opaque strings compared for equality.
6. **Ordering contract.** Sequential publishes from one goroutine arrive in order on any
   given subscription (subject to drops); concurrent publishers have no relative order.

## Alternatives Considered

- **Blocking fan-out** — head-of-line blocking: one slow subscriber stalls every publisher;
  uncancellable given no ctx. Rejected.
- **Per-subscriber pump goroutine with unbounded queue** — hides back-pressure as unbounded
  memory growth and adds N goroutines to leak-audit. Rejected.
- **Drop-oldest instead of drop-newest** — requires a ring buffer per subscription and
  reorders the "latest wins" semantics surprisingly for queue consumers. Rejected for v1;
  revisitable behind an option without breaking the API.
- **Wildcard/hierarchical topics** — real demand exists (MQTT-style trees) but is scope
  creep for feature 2's contract. Rejected for v1; a superseding ADR can add it.

## Consequences

- Consumers needing guaranteed delivery must size buffers for their burst profile or drain
  promptly; the drop handler provides the observability to tune this. At-most-once is a
  documented property, not a bug report.
- Filters run synchronously on the publishing goroutine: they must be fast and side-effect
  free (documented on `Subscribe`).
- Generic options (`Option[T]`) cannot be type-inferred from `NewBroker[T]`'s call site;
  call sites instantiate explicitly (`WithSubscriberBuffer[int](4)`) — a known Go generics
  ergonomics limit, documented on `Option`.
- The shared test helper moves to `internal/leakcheck` (workerpool's copy migrates in this
  PR); ROADMAP 2.6 replaces it with goleak and upgrades the randomized delivery test to
  rapid (shrinking).

## References

- `docs/specs/01_spec_utils.md` §2.2, §5, §6.
- `docs/patterns/design-patterns.md` §4 (Publish-Subscribe, EIP).
- ADR-0005 (RWMutex close-safety idiom).
