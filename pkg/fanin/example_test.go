package fanin_test

import (
	"context"
	"fmt"
	"slices"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/fanin"
)

// Merge collapses several producers into one channel to range over.
func ExampleMerge() {
	// Each producer closes its own channel: Merge closes the output once every
	// input has closed, which is what ends the range below.
	produce := func(vals ...int) <-chan int {
		out := make(chan int, len(vals))
		for _, v := range vals {
			out <- v
		}
		close(out)
		return out
	}

	merged := fanin.Merge(context.Background(), produce(1, 2), produce(3, 4), produce(5))

	// Draining until the output closes is the consumer's half of the contract —
	// abandoning it without cancelling the context would leave forwarders
	// blocked on their next send. Values are sorted before printing because
	// Merge preserves the order *within* each input, not across inputs.
	got := make([]int, 0, 5)
	for v := range merged {
		got = append(got, v)
	}
	slices.Sort(got)
	fmt.Println(got)
	// Output: [1 2 3 4 5]
}
