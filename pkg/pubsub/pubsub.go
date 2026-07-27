// Package pubsub provides a minimal in-memory publish-subscribe broker over
// Go channels with per-subscription filters.
//
// Delivery is at-most-once per subscription: Publish never blocks on a slow
// subscriber — a subscription whose buffer is full at delivery time simply
// does not receive that message (observable via WithDropHandler). This is the
// only policy coherent with the Publish(topic, msg) contract, which carries
// neither a context nor an error return; the trade-off is recorded in
// ADR-0006. The broker owns no goroutines, so it is leak-free by
// construction.
//
// Which message is sacrificed is configurable. By default the new one is
// dropped, keeping the earliest pending work — right for event-like streams
// where every message is independently meaningful. WithDropOldest inverts it,
// evicting the oldest buffered message so the subscriber sees the most recent
// ones — right for state-like streams where a later message supersedes an
// earlier one (ADR-0039). Under either policy every message is, per
// subscription, either delivered or reported to the drop handler exactly once.
//
// Ordering: messages published sequentially from one goroutine arrive in
// order on any given subscription (subject to drops); publishes from
// concurrent goroutines have no relative order.
package pubsub

import "sync"

type subscriber[T any] struct {
	ch     chan T
	filter func(T) bool
}

// Broker is an in-memory publish-subscribe broker carrying messages of type
// T. All methods are safe for concurrent use. The zero value is not usable;
// construct a Broker with NewBroker.
type Broker[T any] struct {
	bufSize    int
	onDrop     func(topic string, msg T)
	dropOldest bool // WithDropOldest; default false = drop-newest

	// mu serialises Subscribe/unsubscribe/Close (write lock) against Publish
	// bodies (read lock): a subscription channel is only ever closed under
	// the write lock, so a send on a closed channel is provably impossible —
	// the same lifecycle idiom as workerpool (ADR-0005).
	mu     sync.RWMutex
	subs   map[string]map[*subscriber[T]]struct{}
	closed bool
}

// NewBroker builds a Broker for messages of type T. By default each
// subscription buffers 16 messages; tune with WithSubscriberBuffer.
func NewBroker[T any](opts ...Option[T]) *Broker[T] {
	b := &Broker[T]{
		bufSize: 16,
		subs:    make(map[string]map[*subscriber[T]]struct{}),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Publish delivers msg to every current subscription on topic whose filter
// accepts it. A subscription whose buffer is full does not receive the
// message (the drop handler, if installed, observes it). Publish never
// blocks on subscribers, and is a silent no-op on a closed broker.
func (b *Broker[T]) Publish(topic string, msg T) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return
	}
	for sub := range b.subs[topic] {
		if sub.filter != nil && !sub.filter(msg) {
			continue
		}
		b.deliver(topic, sub, msg)
	}
}

// deliver hands msg to one subscription, applying the configured
// slow-subscriber policy when its buffer is full. The caller holds mu for
// reading, which is what makes receiving from sub.ch here safe: a subscription
// channel is only ever closed under the write lock.
//
// Exactly one of two things happens to msg per subscription — it is buffered, or
// it is reported to the drop handler. Under drop-oldest the *evicted* message is
// the one reported, and since an evicted message was buffered but never
// received, the invariant "every message is either received or reported dropped,
// once" holds under both policies. NFR-03's benchmark asserts it.
func (b *Broker[T]) deliver(topic string, sub *subscriber[T], msg T) {
	select {
	case sub.ch <- msg:
		return
	default:
	}

	if !b.dropOldest {
		b.reportDrop(topic, msg)
		return
	}

	// Make room by discarding the oldest buffered message.
	select {
	case oldest := <-sub.ch:
		b.reportDrop(topic, oldest)
	default:
		// The subscriber drained the buffer between the failed send above and
		// this receive, so there is room again and nothing needs evicting. Also
		// the normal case for a rendezvous subscription, which has no buffer to
		// evict from.
	}

	select {
	case sub.ch <- msg:
	default:
		// A concurrent publisher refilled the buffer in the window. Retrying has
		// no bound and Publish must not block, so this message is dropped
		// instead: drop-oldest degrades to drop-newest under contention rather
		// than spinning.
		b.reportDrop(topic, msg)
	}
}

// reportDrop notifies the drop handler, if one is installed.
func (b *Broker[T]) reportDrop(topic string, msg T) {
	if b.onDrop != nil {
		b.onDrop(topic, msg)
	}
}

// Subscribe registers a subscription on topic. The returned channel receives
// matching messages until unsubscribe is called or the broker is closed —
// both close the channel. filter may be nil to receive every message on the
// topic; a non-nil filter runs synchronously on the publishing goroutine and
// must be fast and side-effect free. unsubscribe is idempotent and safe for
// concurrent use. Subscribing to a closed broker returns an already-closed
// channel and a no-op unsubscribe.
func (b *Broker[T]) Subscribe(topic string, filter func(T) bool) (<-chan T, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan T, b.bufSize)
	if b.closed {
		close(ch)
		return ch, func() {}
	}
	sub := &subscriber[T]{ch: ch, filter: filter}
	set, ok := b.subs[topic]
	if !ok {
		set = make(map[*subscriber[T]]struct{})
		b.subs[topic] = set
	}
	set[sub] = struct{}{}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.closed {
				return // Close already closed every channel
			}
			delete(b.subs[topic], sub)
			if len(b.subs[topic]) == 0 {
				delete(b.subs, topic)
			}
			close(sub.ch)
		})
	}
	return ch, unsubscribe
}

// Close shuts the broker down: every subscription channel is closed and the
// registry is cleared. Afterwards Publish is a silent no-op and Subscribe
// returns an already-closed channel. Close is idempotent. It extends the
// intake API surface additively (recorded in ADR-0006 and spec §5).
func (b *Broker[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, set := range b.subs {
		for sub := range set {
			close(sub.ch)
		}
	}
	b.subs = nil
}
