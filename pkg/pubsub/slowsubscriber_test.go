package pubsub_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// drain reads everything currently buffered on ch without blocking, which is how
// these tests inspect what survived the policy: the subscriber deliberately does
// not consume during publishing, so the buffer is full and the policy decides. It
// also terminates on a closed channel, which is what Disconnect leaves behind.
func drain[T any](ch <-chan T) []T {
	var got []T
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, v)
		default:
			return got
		}
	}
}

// live registers a subscription whose context outlives the test body, for the
// cases where the policy — not cancellation — is what ends it.
func live[T any](t *testing.T, b *pubsub.Broker[T], topic string, filter func(T) bool) <-chan T {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return b.Subscribe(ctx, topic, filter)
}

func TestDropOldestKeepsTheNewestMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](2),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	for i := 1; i <= 4; i++ {
		_ = br.Publish(context.Background(), "t", i)
	}

	require.Equal(t, []int{3, 4}, drain(ch),
		"a buffer of 2 under drop-oldest holds the two most recent messages")
	require.Equal(t, []int{1, 2}, dropped,
		"the drop handler reports the evicted messages, oldest first")
}

func TestDropOldestReportsSlowSubscriberEvenThoughTheMessageGotThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The subtle half of the error contract: under drop-oldest the *published*
	// message is delivered, and a previously buffered one is lost. The publish
	// still lost a message, so it must still say so — the error asks "did this
	// publish lose anything", not "was msg delivered".
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](1),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	require.NoError(t, br.Publish(context.Background(), "t", 1))
	require.ErrorIs(t, br.Publish(context.Background(), "t", 2), pubsub.ErrSlowSubscriber)
	require.Equal(t, []int{2}, drain(ch), "the newest message is the one that survived")
}

func TestDefaultPolicyKeepsTheOldestMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](2),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	for i := 1; i <= 4; i++ {
		_ = br.Publish(context.Background(), "t", i)
	}

	require.Equal(t, []int{1, 2}, drain(ch),
		"the default policy keeps the earliest pending messages")
	require.Equal(t, []int{3, 4}, dropped,
		"and reports the ones that could not be delivered")
}

func TestDefaultPolicyIsTheZeroValue(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The enum's zero value has to be the historical default, or a broker built
	// without the option would silently change behaviour.
	var zero pubsub.SlowSubscriberPolicy
	require.Equal(t, pubsub.DropNewest, zero)
}

func TestDropOldestPreservesOrderOfSurvivors(t *testing.T) {
	defer goleak.VerifyNone(t)
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](4),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	for i := 1; i <= 20; i++ {
		_ = br.Publish(context.Background(), "t", i)
	}

	require.Equal(t, []int{17, 18, 19, 20}, drain(ch),
		"eviction is FIFO, so the survivors stay in publication order")
}

func TestDropOldestAccountsForEveryMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The invariant NFR-03's benchmark relies on: per subscription, every
	// published message is either received or reported dropped — exactly once.
	// Drop-oldest could break it by reporting a message that was also received.
	var dropped int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](3),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
		pubsub.WithDropHandler[int](func(string, int) { dropped++ }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	const published = 50
	for i := range published {
		_ = br.Publish(context.Background(), "t", i)
	}

	received := len(drain(ch))
	require.Equal(t, published, received+dropped,
		"received %d + dropped %d must account for all %d publishes",
		received, dropped, published)
}

func TestDropOldestIsANoOpForRendezvousSubscriptions(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](0), // rendezvous: nothing to evict
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	_ = br.Publish(context.Background(), "t", 1)
	_ = br.Publish(context.Background(), "t", 2)

	require.Empty(t, drain(ch), "no subscriber was ready, so nothing was delivered")
	require.Equal(t, []int{1, 2}, dropped,
		"with no buffer to evict from, drop-oldest behaves as drop-newest")
}

func TestDropOldestDeliversWhenSubscriberKeepsUp(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The policy must be invisible when the buffer never fills.
	//
	// "Keeps up" has to be established deterministically rather than hoped for.
	// A first attempt published 1000 messages in a tight loop while a goroutine
	// drained: 729 were dropped, because Publish never blocks and so always
	// outruns a consumer — the subscriber was never keeping up at all. Consuming
	// each message before publishing the next makes the condition real, and holds
	// the buffer at one.
	var dropped int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](8),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
		pubsub.WithDropHandler[int](func(string, int) { dropped++ }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	const published = 200
	for i := range published {
		require.NoError(t, br.Publish(context.Background(), "t", i),
			"a publish that loses nothing returns nil")
		got, ok := <-ch
		require.True(t, ok)
		require.Equal(t, i, got, "messages arrive in order and none is skipped")
	}

	require.Zero(t, dropped, "a subscriber that genuinely keeps up loses nothing")
}

// TestDropOldestUnderConcurrentPublishers exercises the two-step evict-then-send
// window that makes the policy best-effort: publishers race each other and a
// subscriber drains concurrently. Nothing may panic (a receive on a closed
// channel would), and the accounting invariant must survive the race. Run under
// -race in CI.
func TestDropOldestUnderConcurrentPublishers(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped atomic.Int64
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](8), // small, so the policy actually fires
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
		pubsub.WithDropHandler[int](func(string, int) { dropped.Add(1) }),
	)

	ch := live(t, br, "t", nil)
	var received atomic.Int64
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range ch {
			received.Add(1)
		}
	}()

	const (
		publishers = 8
		perWorker  = 500
	)
	var wg sync.WaitGroup
	for w := range publishers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perWorker {
				_ = br.Publish(context.Background(), "t", w*perWorker+i)
			}
		}(w)
	}
	wg.Wait()
	br.Close()
	<-drained

	require.Equal(t, int64(publishers*perWorker), received.Load()+dropped.Load(),
		"received %d + dropped %d must equal %d publishes even under contention",
		received.Load(), dropped.Load(), publishers*perWorker)
}

func TestDropOldestPerTopicAndPerSubscription(t *testing.T) {
	defer goleak.VerifyNone(t)
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](2),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
	)
	defer br.Close()

	a := live(t, br, "topic-a", nil)
	b := live(t, br, "topic-a", nil)
	other := live(t, br, "topic-b", nil)

	for i := 1; i <= 3; i++ {
		_ = br.Publish(context.Background(), "topic-a", i)
	}

	require.Equal(t, []int{2, 3}, drain(a), "each subscription applies the policy to its own buffer")
	require.Equal(t, []int{2, 3}, drain(b))
	require.Empty(t, drain(other), "a different topic is untouched")
}

func TestDropOldestWithFilter(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](2),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	// Only even numbers reach the subscription, so filtering happens before the
	// policy — a filtered-out message is not a drop.
	ch := live(t, br, "t", func(m int) bool { return m%2 == 0 })
	for i := 1; i <= 8; i++ {
		_ = br.Publish(context.Background(), "t", i)
	}

	require.Equal(t, []int{6, 8}, drain(ch), "the newest two matching messages survive")
	require.Equal(t, []int{2, 4}, dropped,
		"only messages that passed the filter and were then evicted count as drops")
}

// Disconnect: the third policy, added in 13.5.

func TestDisconnectRemovesTheSlowSubscription(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](1),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.Disconnect),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	require.NoError(t, br.Publish(context.Background(), "t", 1), "fills the buffer")
	require.ErrorIs(t, br.Publish(context.Background(), "t", 2), pubsub.ErrSlowSubscriber,
		"the publish that could not be taken reports it and kills the subscription")

	// The disconnect happens inside the publish that triggered it, so this needs
	// no waiting: a closed channel is observable as soon as Publish returns.
	_, ok := <-ch // the buffered 1 is still receivable
	require.True(t, ok)
	_, ok = <-ch
	require.False(t, ok, "the subscription's channel is closed by the disconnect")

	require.Equal(t, []int{2}, dropped, "the undeliverable message is reported, as under DropNewest")
	require.NoError(t, br.Publish(context.Background(), "t", 3),
		"with the slow subscription gone there is nobody left to lose a message")
}

func TestDisconnectLeavesBufferedMessagesReceivable(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Closing a channel does not discard its contents, so a shed subscriber can
	// still drain what it had accepted. Worth pinning: it is the difference
	// between "disconnected" and "wiped".
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](3),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.Disconnect),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	for i := 1; i <= 4; i++ { // the fourth cannot be taken
		_ = br.Publish(context.Background(), "t", i)
	}

	require.Equal(t, []int{1, 2, 3}, drain(ch),
		"everything already buffered survives the disconnect")
}

func TestDisconnectAccountsForEveryMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The accounting invariant restated for a policy that ends subscriptions:
	// *while it is registered*, every message published to a subscription is
	// either delivered or reported dropped, exactly once. Messages published
	// after the disconnect belong to no subscription at all.
	var dropped int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](4),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.Disconnect),
		pubsub.WithDropHandler[int](func(string, int) { dropped++ }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	const beforeDisconnect = 5 // buffer 4, so the fifth disconnects
	for i := range beforeDisconnect {
		_ = br.Publish(context.Background(), "t", i)
	}
	for i := range 10 { // published to nobody
		require.NoError(t, br.Publish(context.Background(), "t", 100+i))
	}

	received := len(drain(ch))
	require.Equal(t, beforeDisconnect, received+dropped,
		"received %d + dropped %d must account for the %d publishes made while the subscription lived",
		received, dropped, beforeDisconnect)
}

func TestDisconnectLeavesOtherSubscriptionsAlone(t *testing.T) {
	defer goleak.VerifyNone(t)
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](1),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.Disconnect),
	)
	defer br.Close()

	fast := live(t, br, "t", nil)
	slow := live(t, br, "t", nil)

	require.NoError(t, br.Publish(context.Background(), "t", 1))
	require.Equal(t, 1, <-fast) // fast keeps up; slow leaves 1 in its buffer

	require.ErrorIs(t, br.Publish(context.Background(), "t", 2), pubsub.ErrSlowSubscriber,
		"only the subscription that could not take the message is affected")

	require.Equal(t, 2, <-fast, "the subscriber that kept up is still served")
	require.Equal(t, []int{1}, drain(slow), "the shed subscriber keeps what it had")

	require.NoError(t, br.Publish(context.Background(), "t", 3))
	require.Equal(t, 3, <-fast)
}

func TestDisconnectIsInvisibleWhileNobodyIsSlow(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](8),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.Disconnect),
		pubsub.WithDropHandler[int](func(string, int) { dropped++ }),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	for i := range 200 {
		require.NoError(t, br.Publish(context.Background(), "t", i))
		got, ok := <-ch
		require.True(t, ok, "a subscriber that keeps up is never disconnected")
		require.Equal(t, i, got)
	}
	require.Zero(t, dropped)
}

func TestDisconnectKillsARendezvousSubscriptionOnTheFirstMissedMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The sharp edge of combining the two: a rendezvous subscription has no
	// buffer, so any message published while it is not blocked on a receive is
	// undeliverable — and under Disconnect that ends it immediately. Documented
	// and tested rather than left to be discovered.
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](0),
		pubsub.WithSlowSubscriberPolicy[int](pubsub.Disconnect),
	)
	defer br.Close()

	ch := live(t, br, "t", nil)
	require.ErrorIs(t, br.Publish(context.Background(), "t", 1), pubsub.ErrSlowSubscriber)
	if _, ok := <-ch; ok {
		t.Fatal("a rendezvous subscription survived a message it could not receive")
	}
}

// The three policies differ only when a buffer is full, so that is the path
// worth measuring: drop-newest is one failed send, drop-oldest is a failed send
// plus a receive plus a second send. Both benchmarks publish into a saturated,
// undrained subscription so every iteration takes the policy path.
//
// Disconnect has no saturated benchmark on purpose: its first iteration removes
// the subscription, so every later one would publish to an empty topic and the
// figure would describe a broker with no subscribers rather than the policy.

func benchmarkSaturated(b *testing.B, opts ...pubsub.Option[int]) {
	br := pubsub.NewBroker[int](append([]pubsub.Option[int]{
		pubsub.WithSubscriberBuffer[int](8),
	}, opts...)...)
	defer br.Close()

	ctx := context.Background()
	_ = br.Subscribe(ctx, "t", nil)
	for i := range 8 { // fill the buffer before timing
		_ = br.Publish(ctx, "t", i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		_ = br.Publish(ctx, "t", i)
	}
}

func BenchmarkPolicyDropNewestSaturated(b *testing.B) { benchmarkSaturated(b) }

func BenchmarkPolicyDropOldestSaturated(b *testing.B) {
	benchmarkSaturated(b, pubsub.WithSlowSubscriberPolicy[int](pubsub.DropOldest))
}

// The option validators' panic paths.

func TestWithSubscriberBufferPanicsOnNegative(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "pubsub: negative subscriber buffer", func() {
		pubsub.WithSubscriberBuffer[int](-1)
	})
}

func TestWithDropHandlerPanicsOnNil(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "pubsub: nil drop handler", func() {
		pubsub.WithDropHandler[int](nil)
	})
}

func TestWithSlowSubscriberPolicyPanicsOnUnknown(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "pubsub: unknown slow-subscriber policy", func() {
		pubsub.WithSlowSubscriberPolicy[int](pubsub.SlowSubscriberPolicy(42))
	})
}
