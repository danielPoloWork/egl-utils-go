package fanout_test

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/fanout"
	"github.com/danielPoloWork/egl-utils-go/internal/leakcheck"
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
	leakcheck.Guard(t)
	in := make(chan int, 1)
	in <- 42
	fanout.Split(context.Background(), in)
	// Zero outputs spawn zero forwarders, so this cannot race with one.
	if len(in) != 1 {
		t.Fatal("Split with no outputs consumed from the input")
	}
}

func TestSplitNilInputPanics(t *testing.T) {
	leakcheck.Guard(t)
	defer func() {
		if recover() == nil {
			t.Fatal("Split with a nil input did not panic")
		}
	}()
	fanout.Split[int](context.Background(), nil, make(chan int))
}

func TestSplitNilOutputPanics(t *testing.T) {
	leakcheck.Guard(t)
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
	leakcheck.Guard(t)
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

// TestSplitRandomizedCompletenessProperty drives Split with a random topology
// of concurrent consumers and asserts every value is delivered exactly once,
// in input order within each output (seed logged for reproduction; migrates
// to rapid under ROADMAP 2.6).
func TestSplitRandomizedCompletenessProperty(t *testing.T) {
	leakcheck.Guard(t)
	seed := rand.Uint64()
	t.Logf("seed: %d", seed)
	rng := rand.New(rand.NewPCG(seed, 0))

	outputs := 1 + rng.IntN(8)
	total := rng.IntN(201)

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

	var mu sync.Mutex
	counts := make([]int, total)
	var consumers sync.WaitGroup
	consumers.Add(outputs)
	for i := range chans {
		go func() {
			defer consumers.Done()
			last := -1
			for v := range chans[i] {
				if v <= last {
					// Errorf, not Fatalf: keep draining so the topology
					// still winds down and the leak guard stays clean.
					t.Errorf("output %d: value %d after %d — order broken (seed %d)",
						i, v, last, seed)
				}
				last = v
				mu.Lock()
				counts[v]++
				mu.Unlock()
			}
		}()
	}
	consumers.Wait()

	for v, c := range counts {
		if c != 1 {
			t.Fatalf("value %d delivered %d times, want exactly once (seed %d)", v, c, seed)
		}
	}
}

func TestCancelUnblocksForwardersAndClosesOutputs(t *testing.T) {
	leakcheck.Guard(t)
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
