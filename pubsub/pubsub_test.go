package pubsub_test

import (
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/internal/leakcheck"
	"github.com/danielPoloWork/egl-utils-go/pubsub"
)

// collect unsubscribes (closing the channel) and drains everything buffered.
func collect[T any](ch <-chan T, unsubscribe func()) []T {
	unsubscribe()
	var out []T
	for v := range ch {
		out = append(out, v)
	}
	return out
}

func TestPublishDeliversWithAndWithoutFilter(t *testing.T) {
	leakcheck.Guard(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	all, unsubAll := b.Subscribe("numbers", nil)
	even, unsubEven := b.Subscribe("numbers", func(n int) bool { return n%2 == 0 })

	for i := 1; i <= 10; i++ {
		b.Publish("numbers", i)
	}

	gotAll := collect(all, unsubAll)
	if len(gotAll) != 10 {
		t.Fatalf("unfiltered subscription received %d messages, want 10: %v", len(gotAll), gotAll)
	}
	for i, v := range gotAll {
		if v != i+1 {
			t.Fatalf("unfiltered subscription out of order: %v", gotAll)
		}
	}

	gotEven := collect(even, unsubEven)
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
	leakcheck.Guard(t)
	b := pubsub.NewBroker[string]()
	defer b.Close()

	ch, unsub := b.Subscribe("alpha", nil)
	b.Publish("beta", "stray")

	if got := collect(ch, unsub); len(got) != 0 {
		t.Fatalf("subscription on alpha received messages from beta: %v", got)
	}
}

func TestDropOnFullBufferIsObservable(t *testing.T) {
	leakcheck.Guard(t)
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

	ch, unsub := b.Subscribe("t", nil)
	b.Publish("t", 1) // fills the single buffer slot
	b.Publish("t", 2) // must be dropped, observably

	select {
	case d := <-drops:
		if d.topic != "t" || d.msg != 2 {
			t.Fatalf("drop handler observed %+v, want {t 2}", d)
		}
	case <-time.After(time.Second):
		t.Fatal("drop handler was never invoked")
	}
	if got := collect(ch, unsub); len(got) != 1 || got[0] != 1 {
		t.Fatalf("subscriber received %v, want [1]", got)
	}
}

func TestUnsubscribeIsIdempotentAndStopsDelivery(t *testing.T) {
	leakcheck.Guard(t)
	b := pubsub.NewBroker[int]()
	defer b.Close()

	ch, unsub := b.Subscribe("t", nil)
	unsub()
	unsub() // second call must be a safe no-op

	b.Publish("t", 42) // no live subscription — must not panic or deliver

	if _, ok := <-ch; ok {
		t.Fatal("channel still delivered after unsubscribe")
	}
}

func TestCloseClosesEverythingAndPublishBecomesNoop(t *testing.T) {
	leakcheck.Guard(t)
	b := pubsub.NewBroker[int]()
	a, unsubA := b.Subscribe("a", nil)
	c, _ := b.Subscribe("c", nil)

	b.Close()
	b.Close() // idempotent

	if _, ok := <-a; ok {
		t.Fatal("subscription a not closed by Close")
	}
	if _, ok := <-c; ok {
		t.Fatal("subscription c not closed by Close")
	}

	b.Publish("a", 1) // silent no-op, must not panic
	unsubA()          // unsubscribe after Close, must not panic

	late, lateUnsub := b.Subscribe("a", nil)
	if _, ok := <-late; ok {
		t.Fatal("Subscribe on a closed broker returned an open channel")
	}
	lateUnsub() // no-op, must not panic
}

// TestRandomizedDeliveryProperty is a property-style test over random
// topologies (ROADMAP 2.6 migrates it to rapid for shrinking): for ample
// buffers and sequential publishes, every subscription receives exactly the
// filter-matching messages for its topic, in publish order.
func TestRandomizedDeliveryProperty(t *testing.T) {
	leakcheck.Guard(t)
	seed := rand.Uint64()
	t.Logf("seed: %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))

	topics := []string{"a", "b", "c"}
	const messages = 200

	b := pubsub.NewBroker[int](pubsub.WithSubscriberBuffer[int](messages))
	defer b.Close()

	type subscription struct {
		topic string
		mod   int
		ch    <-chan int
		unsub func()
	}
	subs := make([]subscription, 0, 12)
	for _, topic := range topics {
		for _, mod := range []int{1, 2, 3, 5} {
			ch, unsub := b.Subscribe(topic, func(n int) bool { return n%mod == 0 })
			subs = append(subs, subscription{topic, mod, ch, unsub})
		}
	}

	published := make(map[string][]int)
	for i := range messages {
		topic := topics[rng.IntN(len(topics))]
		b.Publish(topic, i)
		published[topic] = append(published[topic], i)
	}

	for _, s := range subs {
		var want []int
		for _, n := range published[s.topic] {
			if n%s.mod == 0 {
				want = append(want, n)
			}
		}
		got := collect(s.ch, s.unsub)
		if len(got) != len(want) {
			t.Fatalf("topic %s mod %d: received %d messages, want %d (seed %d)",
				s.topic, s.mod, len(got), len(want), seed)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("topic %s mod %d: message %d is %d, want %d (seed %d)",
					s.topic, s.mod, i, got[i], want[i], seed)
			}
		}
	}
}

// TestConcurrentChurnIsRaceFree exercises publish/subscribe/unsubscribe/close
// concurrency purely for the race detector and the leak guard.
func TestConcurrentChurnIsRaceFree(t *testing.T) {
	leakcheck.Guard(t)
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
					b.Publish("churn", i)
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
					ch, unsub := b.Subscribe("churn", func(n int) bool { return n%2 == 0 })
					select {
					case <-ch:
					default:
					}
					unsub()
				}
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
	b.Close()
}
