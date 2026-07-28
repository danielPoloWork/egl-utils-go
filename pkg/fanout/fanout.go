// Package fanout distributes values from one input channel across multiple
// output channels — the fan-out half of the Go pipelines vocabulary (the
// fan-in half is package fanin).
//
// Split delivers every input value to exactly one output — whichever
// destination is ready first — and never drops or duplicates a value on the
// input-closed path (completeness). It leaks no goroutines: every internal
// forwarder exits when the input closes or ctx is canceled. Split takes
// send-ownership of the outputs and closes each one on exit; callers must
// not send on or close them after the handoff. Design decisions are recorded
// in ADR-0008.
package fanout

import "context"

// Split fans the input channel out across the output channels. One forwarder
// goroutine per output competes to receive from in, so each value is
// delivered to exactly one output — whichever is ready first; no round-robin
// or fairness is promised. The values any single output receives preserve
// their input order. Split returns immediately; the forwarders run until in
// closes or ctx is canceled, and each closes its output on exit. With no
// outputs Split is a no-op that never reads from in: with nowhere to deliver,
// draining would silently lose values.
//
// A slow destination exerts backpressure on in only once every destination
// is busy. ctx must be non-nil. A nil input or output channel panics: it
// could never move a value, only block a forwarder forever. On cancellation
// a value already taken by a forwarder but not yet delivered is dropped —
// cancellation abandons in-flight work, as it does in fanin.Merge.
func Split[T any](ctx context.Context, in <-chan T, outs ...chan<- T) {
	if in == nil {
		panic("fanout: nil input channel")
	}
	for _, out := range outs {
		if out == nil {
			panic("fanout: nil output channel")
		}
	}
	for _, out := range outs {
		go func() {
			defer close(out)
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
}
