package pubsub_test

import (
	"context"
	"fmt"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub"
)

// Subscribe scopes a subscription to a context: cancelling it is the
// unsubscribe, so there is no second lifetime to keep track of.
func ExampleBroker_Subscribe() {
	broker := pubsub.NewBroker[string]()
	defer broker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	orders := broker.Subscribe(ctx, "orders", nil)

	// A subscription is buffered (16 messages by default), so a sequential
	// publisher can hand over several messages before the consumer reads any.
	// That buffer is the whole reason this works: Publish never waits for a
	// subscriber, so a full buffer means a dropped message, not a blocked
	// publisher.
	for _, id := range []string{"a-1", "a-2", "a-3"} {
		if err := broker.Publish(context.Background(), "orders", id); err != nil {
			fmt.Println("publish:", err)
			return
		}
	}

	// Messages published in sequence from one goroutine arrive in order.
	for range 3 {
		fmt.Println(<-orders)
	}
	// Output:
	// a-1
	// a-2
	// a-3
}

// A filter belongs to the subscription, so two subscribers on one topic can see
// different slices of it.
func ExampleBroker_Subscribe_filter() {
	broker := pubsub.NewBroker[int]()
	defer broker.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evens := broker.Subscribe(ctx, "numbers", func(n int) bool { return n%2 == 0 })

	for n := range 6 {
		if err := broker.Publish(context.Background(), "numbers", n); err != nil {
			fmt.Println("publish:", err)
			return
		}
	}

	// Only the three even values were ever queued for this subscription; the
	// odd ones were never delivered and are not drops.
	got := make([]int, 0, 3)
	for range 3 {
		got = append(got, <-evens)
	}
	fmt.Println(got)
	// Output: [0 2 4]
}
