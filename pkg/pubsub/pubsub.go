// Package pubsub provides a minimal in-memory publish-subscribe broker over
// Go channels with per-subscription filters.
//
// A subscription's lifetime is its context. Subscribe returns only a channel;
// cancelling the context unregisters the subscription and closes that channel,
// so cancel is the unsubscribe. Removal is prompt but asynchronous: a message
// published concurrently with cancellation may still be buffered, and the
// channel closing — not the cancel call returning — is the definitive end of
// the subscription. A subscription created with a context that cannot be
// cancelled (context.Background) lives until the broker is closed.
//
// Delivery is at-most-once per subscription: Publish never blocks on a slow
// subscriber. Its context is checked once, before anything is delivered, so a
// cancelled publish delivers to nobody; nothing about the error return means
// the publisher waited. The error reports whether the publish lost a message
// (ErrSlowSubscriber) or found the broker closed (ErrClosed).
//
// What happens to a subscription that cannot keep up is chosen with
// WithSlowSubscriberPolicy: keep the earliest pending work and discard the new
// message (DropNewest, the default), evict the oldest buffered message so the
// subscriber sees the most recent ones (DropOldest), or unregister the
// subscription entirely (Disconnect). Under every policy the same accounting
// invariant holds: while a subscription is registered, each message published
// to it is either delivered or reported to the drop handler, exactly once.
//
// The broker owns no goroutines — fan-out runs on the publishing goroutine and
// context watching costs nothing while it waits — so it is leak-free by
// construction. Design decisions are recorded in ADR-0006, ADR-0039 and
// ADR-0049.
//
// Ordering: messages published sequentially from one goroutine arrive in
// order on any given subscription (subject to drops); publishes from
// concurrent goroutines have no relative order.
package pubsub

import (
	"context"
	"errors"
	"sync"
)

// Sentinel errors returned by Publish.
var (
	// ErrSlowSubscriber is returned by Publish when at least one matching
	// subscription could not take the message and the slow-subscriber policy
	// fired. It says that this publish lost a message somewhere, not which one:
	// WithDropHandler reports the individual messages.
	ErrSlowSubscriber = errors.New("pubsub: subscriber cannot keep up")

	// ErrClosed is returned by Publish after Close has been called.
	ErrClosed = errors.New("pubsub: broker is closed")
)

type subscriber[T any] struct {
	ch     chan T
	filter func(T) bool

	// stopWatch deregisters the context watcher installed by Subscribe. It is
	// assigned under the write lock before the subscription becomes reachable
	// to anything that could read it, so it is never nil where it is called.
	stopWatch func() bool
}

// Broker is an in-memory publish-subscribe broker carrying messages of type
// T. All methods are safe for concurrent use. The zero value is not usable;
// construct a Broker with NewBroker.
type Broker[T any] struct {
	bufSize int
	onDrop  func(topic string, msg T)
	policy  SlowSubscriberPolicy

	// mu serialises Subscribe/remove/Close (write lock) against Publish bodies
	// (read lock): a subscription channel is only ever closed under the write
	// lock, so a send on a closed channel is provably impossible — the same
	// lifecycle idiom as workerpool (ADR-0005).
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
// accepts it, and reports what that cost. It never blocks on subscribers.
//
// ctx is consulted once, before anything is delivered: an already-cancelled
// context means no subscription receives the message and its error is
// returned, so cancellation is all-or-nothing rather than a partial fan-out.
// Publish does not wait on ctx — there is nothing to wait for.
//
// The error is ErrClosed on a closed broker, ErrSlowSubscriber when at least
// one matching subscription lost a message under the configured policy, and
// nil otherwise. A topic with no subscribers is not an error: in
// publish-subscribe, nobody listening is a normal state, not a failure.
func (b *Broker[T]) Publish(ctx context.Context, topic string, msg T) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var (
		lost       bool
		disconnect []*subscriber[T] // stays nil under every other policy
	)

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrClosed
	}
	for sub := range b.subs[topic] {
		if sub.filter != nil && !sub.filter(msg) {
			continue
		}
		if b.deliver(topic, sub, msg) {
			lost = true
			if b.policy == Disconnect {
				disconnect = append(disconnect, sub)
			}
		}
	}
	b.mu.RUnlock()

	// Unregistering mutates the registry, so it cannot happen under the read
	// lock the fan-out holds. remove re-checks membership under the write lock,
	// which is what makes the gap between the two harmless.
	for _, sub := range disconnect {
		b.remove(topic, sub)
	}

	if lost {
		return ErrSlowSubscriber
	}
	return nil
}

// deliver hands msg to one subscription, applying the configured
// slow-subscriber policy when its buffer is full. It reports whether a message
// was lost — which under DropOldest is a previously buffered message rather
// than msg itself. The caller holds mu for reading, which is what makes
// receiving from sub.ch here safe: a subscription channel is only ever closed
// under the write lock.
//
// Exactly one of two things happens to every message per subscription — it is
// buffered, or it is reported to the drop handler. Under DropOldest the
// *evicted* message is the one reported, and since an evicted message was
// buffered but never received, the invariant "every message is either received
// or reported dropped, once" holds under every policy. NFR-03's benchmark
// asserts it.
func (b *Broker[T]) deliver(topic string, sub *subscriber[T], msg T) (lost bool) {
	select {
	case sub.ch <- msg:
		return false
	default:
	}

	if b.policy != DropOldest {
		// DropNewest and Disconnect both sacrifice the incoming message; under
		// Disconnect the caller additionally unregisters the subscription, which
		// needs the write lock and so cannot happen here.
		b.reportDrop(topic, msg)
		return true
	}

	// Make room by discarding the oldest buffered message.
	evicted := false
	select {
	case oldest := <-sub.ch:
		b.reportDrop(topic, oldest)
		evicted = true
	default:
		// The subscriber drained the buffer between the failed send above and
		// this receive, so there is room again and nothing needs evicting. Also
		// the normal case for a rendezvous subscription, which has no buffer to
		// evict from.
	}

	select {
	case sub.ch <- msg:
		return evicted // msg got through; a message was lost only if one was evicted to fit it
	default:
		// A concurrent publisher refilled the buffer in the window. Retrying has
		// no bound and Publish must not block, so this message is dropped
		// instead: drop-oldest degrades to drop-newest under contention rather
		// than spinning.
		b.reportDrop(topic, msg)
		return true
	}
}

// reportDrop notifies the drop handler, if one is installed.
func (b *Broker[T]) reportDrop(topic string, msg T) {
	if b.onDrop != nil {
		b.onDrop(topic, msg)
	}
}

// Subscribe registers a subscription on topic and returns its channel, which
// receives matching messages until ctx is cancelled or the broker is closed —
// both close the channel. Cancelling ctx is the only way to unsubscribe, so a
// subscription made with a context that cannot be cancelled lives until Close;
// removal is prompt but asynchronous, so a concurrent publish may still land
// in the buffer after cancel returns.
//
// filter may be nil to receive every message on the topic; a non-nil filter
// runs synchronously on the publishing goroutine and must be fast and
// side-effect free. Subscribing with an already-cancelled context, or to a
// closed broker, returns an already-closed channel.
func (b *Broker[T]) Subscribe(ctx context.Context, topic string, filter func(T) bool) <-chan T {
	ch := make(chan T, b.bufSize)

	b.mu.Lock()
	defer b.mu.Unlock()

	// A closed broker and a context that is already done are the same case: the
	// subscription cannot live, so hand back a channel that is already closed
	// rather than one that would never deliver and never close.
	if b.closed || ctx.Err() != nil {
		close(ch)
		return ch
	}

	sub := &subscriber[T]{ch: ch, filter: filter}
	set, ok := b.subs[topic]
	if !ok {
		set = make(map[*subscriber[T]]struct{})
		b.subs[topic] = set
	}
	set[sub] = struct{}{}

	// The watcher is what makes a subscription context-scoped, and
	// context.AfterFunc is what keeps that free: it registers a callback on the
	// context rather than parking a goroutine on Done, so N live subscriptions
	// cost no goroutines and ADR-0006's "the broker owns none" survives. The
	// context package runs the callback on its own goroutine when the context is
	// cancelled — never inline — which is why registering it while holding the
	// write lock is safe: a cancellation landing in this instant leaves remove
	// blocked on mu until Subscribe returns.
	sub.stopWatch = context.AfterFunc(ctx, func() { b.remove(topic, sub) })
	return ch
}

// remove unregisters sub from topic and closes its channel, exactly once,
// whichever path asks: the context watcher, the Disconnect policy, or a second
// call after either. The registry is the single source of truth — a
// subscription still present in the map has not been closed — and every
// removal path takes the write lock, which is what makes a double close
// impossible without a per-subscription flag.
func (b *Broker[T]) remove(topic string, sub *subscriber[T]) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subs[topic][sub]; !ok {
		return // already gone: Close, the policy, or this watcher firing twice
	}
	delete(b.subs[topic], sub)
	if len(b.subs[topic]) == 0 {
		delete(b.subs, topic)
	}
	sub.stopWatch()
	close(sub.ch)
}

// Close shuts the broker down: every subscription channel is closed and the
// registry is cleared. Afterwards Publish returns ErrClosed and Subscribe
// returns an already-closed channel. Close is idempotent.
func (b *Broker[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, set := range b.subs {
		for sub := range set {
			// Deregister before closing. Left registered, the watcher keeps this
			// broker and this subscriber reachable from the caller's context
			// until that context is cancelled — which for a long-lived parent is
			// the lifetime of the process.
			sub.stopWatch()
			close(sub.ch)
		}
	}
	b.subs = nil
}
