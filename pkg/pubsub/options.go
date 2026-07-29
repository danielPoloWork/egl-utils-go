package pubsub

// Option customises a Broker at construction time (functional options;
// ADR-0006). Type inference cannot flow from NewBroker to its options, so
// instantiate options explicitly: pubsub.NewBroker[int](pubsub.WithSubscriberBuffer[int](4)).
type Option[T any] func(*Broker[T])

// SlowSubscriberPolicy selects what happens to a subscription whose buffer is
// full when a message is published to it. Publish never blocks under any of
// them; the policy chooses only what is sacrificed (ADR-0049).
type SlowSubscriberPolicy int

const (
	// DropNewest discards the message that could not be delivered, keeping the
	// earliest pending work. It is the zero value, and so the default: right for
	// event-like streams where every message is independently meaningful.
	DropNewest SlowSubscriberPolicy = iota

	// DropOldest evicts the oldest buffered message to make room for the new
	// one, so the subscriber sees the most recent messages rather than the
	// earliest. Right for state-like streams, where a later message supersedes
	// an earlier one and stale data is worse than missing data — a metrics
	// gauge, a price tick, a progress percentage.
	//
	// It is best-effort by construction, and that is the one sharp edge worth
	// knowing. Making room takes two steps — evict, then send — and a subscriber
	// or another publisher can act between them. If the buffer empties in that
	// window nothing is evicted; if another publisher refills it the new message
	// is dropped instead, so delivery degrades to DropNewest for that message
	// rather than retrying without bound (ADR-0039).
	//
	// It has no effect on a rendezvous subscription (WithSubscriberBuffer(0)):
	// there is no buffered message to evict, so behaviour there is DropNewest
	// whatever this says.
	DropOldest

	// Disconnect unregisters the subscription and closes its channel instead of
	// choosing a message to lose. Right when a subscriber that has fallen behind
	// is better shed than served — the undeliverable message is reported to the
	// drop handler, and the subscription receives nothing further.
	//
	// The buffer is the tolerance: the first message a subscription cannot take
	// disconnects it, so size the buffer for the burst that should be survived
	// rather than looking for a second threshold to tune.
	//
	// Whatever was already buffered stays receivable — closing a channel does
	// not discard its contents — so a disconnected subscriber can drain what it
	// had accepted. It cannot, however, tell a disconnect from a normal
	// shutdown: both arrive as a closed channel.
	Disconnect
)

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

// WithDropHandler installs h to observe every message lost because a
// subscription's buffer was full at delivery time. h runs synchronously on the
// publishing goroutine and must be fast and non-blocking; it must not call back
// into the broker, since Publish and Subscribe re-enter the same lock.
//
// Which message h receives depends on the policy: under DropNewest and
// Disconnect it is the message that could not be delivered, and under
// DropOldest it is the message evicted to make room. Either way it is the
// message that was lost, which is what a consumer counting drops wants — but
// the distinction matters if the handler logs the payload.
//
// h answers "what was lost"; Publish's ErrSlowSubscriber answers "did this
// publish lose anything". A nil h panics.
func WithDropHandler[T any](h func(topic string, msg T)) Option[T] {
	if h == nil {
		panic("pubsub: nil drop handler")
	}
	return func(b *Broker[T]) { b.onDrop = h }
}

// WithSlowSubscriberPolicy sets the broker's slow-subscriber policy, which
// applies to every subscription (default DropNewest). An unknown policy panics:
// the enum exists so that an illegal combination cannot be expressed, and an
// out-of-range value is the one way left to try.
//
// A consumer needing different policies for different subscribers of one stream
// runs two brokers; per-subscription policy is deliberately not offered
// (ADR-0049).
func WithSlowSubscriberPolicy[T any](p SlowSubscriberPolicy) Option[T] {
	switch p {
	case DropNewest, DropOldest, Disconnect:
	default:
		panic("pubsub: unknown slow-subscriber policy")
	}
	return func(b *Broker[T]) { b.policy = p }
}
