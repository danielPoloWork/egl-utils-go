package fanout_test

import (
	"context"
	"fmt"
	"sync"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/fanout"
)

// Split spreads one producer across several consumers, each value going to
// exactly one of them.
func ExampleSplit() {
	in := make(chan int, 6)
	for n := 1; n <= 6; n++ {
		in <- n
	}
	close(in)

	// Split takes send-ownership of the outputs and closes each one when the
	// input is done, so a consumer ranges over them and never closes one
	// itself. Split starts its forwarders and returns immediately.
	first := make(chan int)
	second := make(chan int)
	fanout.Split(context.Background(), in, first, second)

	// Which output receives which value depends on whichever consumer is ready
	// first, so only the totals are deterministic — that is what gets printed.
	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		count, total int
	)
	for _, out := range []chan int{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for v := range out {
				mu.Lock()
				count++
				total += v
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	fmt.Println(count, total)
	// Output: 6 21
}
