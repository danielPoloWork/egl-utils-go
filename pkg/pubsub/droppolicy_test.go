package pubsub_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// drain reads everything currently buffered on ch without blocking, which is how
// these tests inspect what survived the policy: the subscriber deliberately does
// not consume during publishing, so the buffer is full and the policy decides.
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

func TestDropOldestKeepsTheNewestMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](2),
		pubsub.WithDropOldest[int](),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	for i := 1; i <= 4; i++ {
		br.Publish("t", i)
	}

	require.Equal(t, []int{3, 4}, drain(ch),
		"a buffer of 2 under drop-oldest holds the two most recent messages")
	require.Equal(t, []int{1, 2}, dropped,
		"the drop handler reports the evicted messages, oldest first")
}

func TestDefaultPolicyKeepsTheOldestMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	var dropped []int
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](2),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	for i := 1; i <= 4; i++ {
		br.Publish("t", i)
	}

	require.Equal(t, []int{1, 2}, drain(ch),
		"the default policy keeps the earliest pending messages")
	require.Equal(t, []int{3, 4}, dropped,
		"and reports the ones that could not be delivered")
}

func TestDropOldestPreservesOrderOfSurvivors(t *testing.T) {
	defer goleak.VerifyNone(t)
	br := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](4),
		pubsub.WithDropOldest[int](),
	)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	for i := 1; i <= 20; i++ {
		br.Publish("t", i)
	}

	got := drain(ch)
	require.Equal(t, []int{17, 18, 19, 20}, got,
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
		pubsub.WithDropOldest[int](),
		pubsub.WithDropHandler[int](func(string, int) { dropped++ }),
	)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	const published = 50
	for i := range published {
		br.Publish("t", i)
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
		pubsub.WithDropOldest[int](),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	br.Publish("t", 1)
	br.Publish("t", 2)

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
		pubsub.WithDropOldest[int](),
		pubsub.WithDropHandler[int](func(string, int) { dropped++ }),
	)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	const published = 200
	for i := range published {
		br.Publish("t", i)
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
		pubsub.WithDropOldest[int](),
		pubsub.WithDropHandler[int](func(string, int) { dropped.Add(1) }),
	)

	ch, _ := br.Subscribe("t", nil)
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
				br.Publish("t", w*perWorker+i)
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
		pubsub.WithDropOldest[int](),
	)
	defer br.Close()

	a, _ := br.Subscribe("topic-a", nil)
	b, _ := br.Subscribe("topic-a", nil)
	other, _ := br.Subscribe("topic-b", nil)

	for i := 1; i <= 3; i++ {
		br.Publish("topic-a", i)
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
		pubsub.WithDropOldest[int](),
		pubsub.WithDropHandler[int](func(_ string, m int) { dropped = append(dropped, m) }),
	)
	defer br.Close()

	// Only even numbers reach the subscription, so filtering happens before the
	// policy — a filtered-out message is not a drop.
	ch, _ := br.Subscribe("t", func(m int) bool { return m%2 == 0 })
	for i := 1; i <= 8; i++ {
		br.Publish("t", i)
	}

	require.Equal(t, []int{6, 8}, drain(ch), "the newest two matching messages survive")
	require.Equal(t, []int{2, 4}, dropped,
		"only messages that passed the filter and were then evicted count as drops")
}

// The two policies differ only when a buffer is full, so that is the path worth
// measuring: drop-newest is one failed send, drop-oldest is a failed send plus a
// receive plus a second send. Both benchmarks publish into a saturated,
// undrained subscription so every iteration takes the policy path.

func benchmarkSaturated(b *testing.B, opts ...pubsub.Option[int]) {
	br := pubsub.NewBroker[int](append([]pubsub.Option[int]{
		pubsub.WithSubscriberBuffer[int](8),
	}, opts...)...)
	defer br.Close()

	ch, _ := br.Subscribe("t", nil)
	for i := range 8 { // fill the buffer before timing
		br.Publish("t", i)
	}
	_ = ch

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		br.Publish("t", i)
	}
}

func BenchmarkPolicyDropNewestSaturated(b *testing.B) { benchmarkSaturated(b) }

func BenchmarkPolicyDropOldestSaturated(b *testing.B) {
	benchmarkSaturated(b, pubsub.WithDropOldest[int]())
}

// The option validators' panic paths, which were the package's only uncovered
// statements before this change.

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
