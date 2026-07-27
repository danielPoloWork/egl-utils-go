// Package fanin merges multiple input channels into a single output channel —
// the fan-in half of the Go pipelines vocabulary (the fan-out half is package
// fanout).
//
// Merge preserves per-input ordering, never drops or duplicates a value
// (completeness), and leaks no goroutines: every internal forwarder exits when
// its input closes or ctx is canceled. The consumer's half of the contract:
// either drain the output until it closes or cancel ctx — abandoning the
// output without canceling leaves forwarders blocked on their next send.
// Design decisions are recorded in ADR-0007.
package fanin

import (
	"context"
	"sync"
)

// Merge fans every input channel into one output channel. The output closes
// once all inputs have closed and their values were forwarded, or once ctx is
// canceled — whichever comes first. With no inputs it returns an
// already-closed channel.
//
// Per-input ordering is preserved; values from different inputs interleave
// with no defined order. The output is unbuffered, so a slow consumer exerts
// backpressure on every input. ctx must be non-nil. A nil input channel
// panics: it could never contribute a value, only block a forwarder forever.
func Merge[T any](ctx context.Context, ins ...<-chan T) <-chan T {
	out := make(chan T)
	if len(ins) == 0 {
		close(out)
		return out
	}
	for _, in := range ins {
		if in == nil {
			panic("fanin: nil input channel")
		}
	}
	var wg sync.WaitGroup
	wg.Add(len(ins))
	for _, in := range ins {
		go func() {
			defer wg.Done()
			for {
				select {
				case v, ok := <-in:
					if !ok {
						return
					}
					select {
					case out <- v:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
