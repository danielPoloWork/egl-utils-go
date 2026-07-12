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
func WithDropHandler[T any](h func(topic string, msg T)) Option[T] {
	if h == nil {
		panic("pubsub: nil drop handler")
	}
	return func(b *Broker[T]) { b.onDrop = h }
}
