package pubsub

// Option customises a Broker at construction time (functional options;
// ADR-0006). Type inference cannot flow from NewBroker to its options, so
// instantiate options explicitly: pubsub.NewBroker[int](pubsub.WithSubscriberBuffer[int](4)).
type Option[T any] func(*Broker[T])

// WithSubscriberBuffer sets each subscription's channel buffer (default 16).
// n == 0 gives rendezvous delivery: a message is received only if the
// subscriber is ready at publish time and is otherwise dropped. A negative n
// panics.
func WithSubscriberBuffer[T any](n int) Option[T] {
	if n < 0 {
		panic("pubsub: negative subscriber buffer")
	}
	return func(b *Broker[T]) { b.bufSize = n }
}

// WithDropHandler installs h to observe every message dropped because a
// subscription's buffer was full at delivery time. h runs synchronously on
// the publishing goroutine and must be fast and non-blocking. A nil h panics.
//
// Which message h receives depends on the policy: under the default
// drop-newest it is the message that could not be delivered, and under
// WithDropOldest it is the message evicted to make room. Either way it is the
// message that was lost, which is what a consumer counting drops wants — but
// the distinction matters if the handler logs the payload.
func WithDropHandler[T any](h func(topic string, msg T)) Option[T] {
	if h == nil {
		panic("pubsub: nil drop handler")
	}
	return func(b *Broker[T]) { b.onDrop = h }
}

// WithDropOldest changes the slow-subscriber policy from drop-newest (the
// default) to drop-oldest: when a subscription's buffer is full, the oldest
// buffered message is discarded to make room for the new one, so the
// subscriber sees the most recent messages rather than the earliest.
//
// Choose it for state-like streams, where a later message supersedes an
// earlier one and stale data is worse than missing data — a metrics gauge, a
// price tick, a progress percentage. Keep the default for event-like streams,
// where each message is independently meaningful and the earliest pending work
// should not be thrown away.
//
// The policy is best-effort under concurrent publishers, and this is the one
// sharp edge worth knowing. Making room takes two steps — evict, then send —
// and a subscriber or another publisher can act between them. If the buffer
// empties in that window nothing is evicted, and if another publisher refills
// it the new message is dropped instead, so delivery degrades to drop-newest
// for that message rather than retrying without bound. Publish still never
// blocks, and every message is still either delivered or reported to the drop
// handler exactly once.
//
// It has no effect on a rendezvous subscription (WithSubscriberBuffer(0)):
// there is no buffered message to evict, so behaviour there is drop-newest
// whatever this option says.
func WithDropOldest[T any]() Option[T] {
	return func(b *Broker[T]) { b.dropOldest = true }
}
