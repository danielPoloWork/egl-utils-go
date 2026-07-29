package pubsub_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

// collect ends a subscription by cancelling its context and drains everything
// buffered. Ranging over the channel is what makes the asynchronous removal
// deterministic: the loop ends when the watcher closes the channel, so no test
// has to guess how long cancellation takes.
func collect[T any](ch <-chan T, cancel context.CancelFunc) []T {
	cancel()
	var out []T
	for v := range ch {
		out = append(out, v)
	}
	return out
}

// subscribe wires a subscription to its own cancellable context, which is the
// unit of subscription lifetime in v2.
func subscribe[T any](t *testing.T, b *pubsub.Broker[T], topic string, filter func(T) bool) (<-chan T, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	return b.Subscribe(ctx, topic, filter), cancel
}

func TestPublishDeliversWithAndWithoutFilter(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	all, cancelAll := subscribe(t, b, "numbers", nil)
	even, cancelEven := subscribe(t, b, "numbers", func(n int) bool { return n%2 == 0 })

	for i := 1; i <= 10; i++ {
		require.NoError(t, b.Publish(context.Background(), "numbers", i))
	}

	gotAll := collect(all, cancelAll)
	if len(gotAll) != 10 {
		t.Fatalf("unfiltered subscription received %d messages, want 10: %v", len(gotAll), gotAll)
	}
	for i, v := range gotAll {
		if v != i+1 {
			t.Fatalf("unfiltered subscription out of order: %v", gotAll)
		}
	}

	gotEven := collect(even, cancelEven)
	if len(gotEven) != 5 {
		t.Fatalf("filtered subscription received %d messages, want 5: %v", len(gotEven), gotEven)
	}
	for _, v := range gotEven {
		if v%2 != 0 {
			t.Fatalf("filter leaked an odd message: %v", gotEven)
		}
	}
}

func TestTopicIsolation(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[string]()
	defer b.Close()

	ch, cancel := subscribe(t, b, "alpha", nil)
	require.NoError(t, b.Publish(context.Background(), "beta", "stray"),
		"a topic with no subscribers is not an error")

	if got := collect(ch, cancel); len(got) != 0 {
		t.Fatalf("subscription on alpha received messages from beta: %v", got)
	}
}

func TestPublishWithoutSubscribersReturnsNil(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	// Nobody listening is a normal state in publish-subscribe, not a failure —
	// worth pinning, because "no subscribers" is the obvious candidate for an
	// error that this API deliberately does not report.
	require.NoError(t, b.Publish(context.Background(), "nobody-home", 1))
}

func TestDropOnFullBufferIsObservableAndReported(t *testing.T) {
	defer goleak.VerifyNone(t)
	type drop struct {
		topic string
		msg   int
	}
	drops := make(chan drop, 4)
	b := pubsub.NewBroker[int](
		pubsub.WithSubscriberBuffer[int](1),
		pubsub.WithDropHandler[int](func(topic string, msg int) {
			drops <- drop{topic, msg}
		}),
	)
	defer b.Close()

	ch, cancel := subscribe(t, b, "t", nil)
	require.NoError(t, b.Publish(context.Background(), "t", 1), "fills the single buffer slot")
	require.ErrorIs(t, b.Publish(context.Background(), "t", 2), pubsub.ErrSlowSubscriber,
		"a publish that lost a message says so")

	select {
	case d := <-drops:
		if d.topic != "t" || d.msg != 2 {
			t.Fatalf("drop handler observed %+v, want {t 2}", d)
		}
	case <-time.After(time.Second):
		t.Fatal("drop handler was never invoked")
	}
	if got := collect(ch, cancel); len(got) != 1 || got[0] != 1 {
		t.Fatalf("subscriber received %v, want [1]", got)
	}
}

func TestCancellingTheContextEndsTheSubscription(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx, "t", nil)

	cancel()
	cancel() // second call must be a safe no-op

	// The channel closing is the definitive end of the subscription; blocking on
	// it is what makes the assertion deterministic despite asynchronous removal.
	if _, ok := <-ch; ok {
		t.Fatal("channel still delivered after its context was cancelled")
	}

	require.NoError(t, b.Publish(context.Background(), "t", 42),
		"the subscription is gone, so this publish loses nothing")
}

func TestSubscribeWithCancelledContextReturnsClosedChannel(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := b.Subscribe(ctx, "t", nil)
	if _, ok := <-ch; ok {
		t.Fatal("Subscribe with an already-cancelled context returned a live channel")
	}
	require.NoError(t, b.Publish(context.Background(), "t", 1),
		"the doomed subscription was never registered, so nothing can be lost")
}

func TestPublishWithCancelledContextDeliversNothing(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	ch, cancel := subscribe(t, b, "t", nil)

	dead, cancelDead := context.WithCancel(context.Background())
	cancelDead()
	require.ErrorIs(t, b.Publish(dead, "t", 1), context.Canceled)

	// All-or-nothing: a cancelled publish reaches nobody, so the live
	// subscription must be empty rather than partially served.
	require.Empty(t, collect(ch, cancel))
}

func TestNonCancellableContextSubscriptionLivesUntilClose(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The documented cost of making the context the only lifetime: with
	// context.Background there is nothing to cancel, so Close is the only way
	// this subscription ever ends.
	b := pubsub.NewBroker[int]()
	ch := b.Subscribe(context.Background(), "t", nil)

	require.NoError(t, b.Publish(context.Background(), "t", 7))
	require.Equal(t, 7, <-ch)

	b.Close()
	if _, ok := <-ch; ok {
		t.Fatal("Close did not end a subscription held by a non-cancellable context")
	}
}

func TestCloseClosesEverythingAndPublishReturnsErrClosed(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int]()
	a, cancelA := subscribe(t, b, "a", nil)
	c, cancelC := subscribe(t, b, "c", nil)
	defer cancelC()

	b.Close()
	b.Close() // idempotent

	if _, ok := <-a; ok {
		t.Fatal("subscription a not closed by Close")
	}
	if _, ok := <-c; ok {
		t.Fatal("subscription c not closed by Close")
	}

	// The silent no-op v1 had here was forced by Publish having no error to
	// return. Now it has one, and discarding a message in silence would hide a
	// shutdown-ordering bug.
	require.ErrorIs(t, b.Publish(context.Background(), "a", 1), pubsub.ErrClosed)

	cancelA() // cancelling after Close must not panic (no double close)

	late := b.Subscribe(context.Background(), "a", nil)
	if _, ok := <-late; ok {
		t.Fatal("Subscribe on a closed broker returned an open channel")
	}
}

// TestDeliveryProperty is a rapid property over random publish sequences: for
// ample buffers and sequential publishes, every subscription receives exactly
// the filter-matching messages for its topic, in publish order. rapid shrinks
// a counterexample to a minimal failing sequence (replacing the seeded
// math/rand property retired in ROADMAP 2.6).
func TestDeliveryProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	topics := []string{"a", "b", "c"}
	mods := []int{1, 2, 3, 5}

	rapid.Check(t, func(rt *rapid.T) {
		publishSeq := rapid.SliceOfN(rapid.SampledFrom(topics), 0, 300).Draw(rt, "publishes")

		// Buffer at least the whole run so nothing is dropped on any topic.
		b := pubsub.NewBroker[int](pubsub.WithSubscriberBuffer[int](len(publishSeq) + 1))
		defer b.Close()

		type subscription struct {
			topic  string
			mod    int
			ch     <-chan int
			cancel context.CancelFunc
		}
		subs := make([]subscription, 0, len(topics)*len(mods))
		for _, topic := range topics {
			for _, mod := range mods {
				ctx, cancel := context.WithCancel(context.Background())
				ch := b.Subscribe(ctx, topic, func(n int) bool { return n%mod == 0 })
				subs = append(subs, subscription{topic, mod, ch, cancel})
			}
		}

		published := make(map[string][]int)
		for i, topic := range publishSeq {
			require.NoError(rt, b.Publish(context.Background(), topic, i))
			published[topic] = append(published[topic], i)
		}

		for _, s := range subs {
			var want []int
			for _, n := range published[s.topic] {
				if n%s.mod == 0 {
					want = append(want, n)
				}
			}
			got := collect(s.ch, s.cancel)
			require.Equalf(rt, want, got, "topic %s mod %d: delivery mismatch", s.topic, s.mod)
		}
	})
}

// TestConcurrentChurnIsRaceFree exercises publish/subscribe/cancel/close
// concurrency purely for the race detector and the leak guard. Cancellation now
// runs removal on a context-owned goroutine, so this is also the test that would
// catch a double close between a watcher firing and Close.
func TestConcurrentChurnIsRaceFree(t *testing.T) {
	defer goleak.VerifyNone(t)
	b := pubsub.NewBroker[int](pubsub.WithSubscriberBuffer[int](4))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for range 4 { // publishers
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
					_ = b.Publish(context.Background(), "churn", i)
				}
			}
		}()
	}
	for range 8 { // subscriber churn
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					ctx, cancel := context.WithCancel(context.Background())
					ch := b.Subscribe(ctx, "churn", func(n int) bool { return n%2 == 0 })
					select {
					case <-ch:
					default:
					}
					cancel()
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
	b.Close()
}
