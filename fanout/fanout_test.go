package fanout_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/fanout"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

// drainUntilClosed discards values until ch closes, failing t if that takes
// longer than the deadline.
func drainUntilClosed[T any](t *testing.T, ch <-chan T, within time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, ok := <-ch; !ok {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatal("output channel did not close in time")
	}
}

func TestSplitNoOutputsIsNoOp(t *testing.T) {
	defer goleak.VerifyNone(t)
	in := make(chan int, 1)
	in <- 42
	fanout.Split(context.Background(), in)
	// Zero outputs spawn zero forwarders, so this cannot race with one.
	if len(in) != 1 {
		t.Fatal("Split with no outputs consumed from the input")
	}
}

func TestSplitNilInputPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	defer func() {
		if recover() == nil {
			t.Fatal("Split with a nil input did not panic")
		}
	}()
	fanout.Split[int](context.Background(), nil, make(chan int))
}

func TestSplitNilOutputPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	in := make(chan int)
	defer func() {
		if recover() == nil {
			t.Fatal("Split with a nil output did not panic")
		}
	}()
	// The nil sits after a valid output: validation must reject the call
	// before any forwarder spawns.
	fanout.Split(context.Background(), in, make(chan int), nil)
}

func TestSplitCompletenessAndPerOutputOrder(t *testing.T) {
	defer goleak.VerifyNone(t)
	const outputs, total = 3, 150

	in := make(chan int, total)
	for seq := range total {
		in <- seq
	}
	close(in)

	chans := make([]chan int, outputs)
	outs := make([]chan<- int, outputs)
	for i := range chans {
		chans[i] = make(chan int, total) // roomy: forwarders never block
		outs[i] = chans[i]
	}

	fanout.Split(context.Background(), in, outs...)

	seen := make([]bool, total)
	received := 0
	for i, ch := range chans {
		last := -1
		for v := range ch {
			if v <= last {
				t.Fatalf("output %d: value %d arrived after %d — per-output input order broken",
					i, v, last)
			}
			last = v
			if seen[v] {
				t.Fatalf("value %d delivered to more than one output", v)
			}
			seen[v] = true
			received++
		}
	}
	if received != total {
		t.Fatalf("received %d values, want %d — distribution is not complete", received, total)
	}
}

// TestSplitCompletenessProperty drives Split with rapid-generated topologies of
// concurrent consumers and asserts every value is delivered exactly once, in
// input order within each output. rapid shrinks a counterexample to a minimal
// failing topology (replacing the seeded math/rand property retired in
// ROADMAP 2.6). Each consumer records only its own output's slice, so the
// assertions all run on the main goroutine (testify require is not
// goroutine-safe).
func TestSplitCompletenessProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	rapid.Check(t, func(rt *rapid.T) {
		outputs := rapid.IntRange(1, 8).Draw(rt, "outputs")
		total := rapid.IntRange(0, 200).Draw(rt, "total")

		in := make(chan int) // unbuffered: producer, forwarders, consumers run concurrently
		chans := make([]chan int, outputs)
		outs := make([]chan<- int, outputs)
		for i := range chans {
			chans[i] = make(chan int)
			outs[i] = chans[i]
		}

		go func() {
			defer close(in)
			for seq := range total {
				in <- seq
			}
		}()

		fanout.Split(context.Background(), in, outs...)

		received := make([][]int, outputs)
		var consumers sync.WaitGroup
		consumers.Add(outputs)
		for i := range chans {
			go func() {
				defer consumers.Done()
				for v := range chans[i] {
					received[i] = append(received[i], v) // distinct index per goroutine — no shared write
				}
			}()
		}
		consumers.Wait()

		counts := make([]int, total)
		for i := range received {
			last := -1
			for _, v := range received[i] {
				require.Greaterf(rt, v, last, "output %d: input order broken", i)
				last = v
				counts[v]++
			}
		}
		for v, c := range counts {
			require.Equalf(rt, 1, c, "value %d was not delivered exactly once", v)
		}
	})
}

func TestCancelUnblocksForwardersAndClosesOutputs(t *testing.T) {
	defer goleak.VerifyNone(t)
	// One value per forwarder plus one spare, input never closed, nobody
	// reading the outputs: every forwarder ends up blocked mid-send with a
	// value in hand — exactly the state cancellation must be able to unblock.
	const outputs = 3
	in := make(chan int, outputs+1)
	for seq := range outputs + 1 {
		in <- seq
	}

	ctx, cancel := context.WithCancel(context.Background())
	chans := make([]chan int, outputs)
	outs := make([]chan<- int, outputs)
	for i := range chans {
		chans[i] = make(chan int)
		outs[i] = chans[i]
	}

	fanout.Split(ctx, in, outs...)

	time.Sleep(20 * time.Millisecond) // let every forwarder reach its blocked send
	cancel()

	for _, ch := range chans {
		drainUntilClosed(t, ch, 2*time.Second)
	}
}
