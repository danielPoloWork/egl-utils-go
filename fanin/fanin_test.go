package fanin_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/fanin"
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

func TestMergeNoInputsClosesImmediately(t *testing.T) {
	defer goleak.VerifyNone(t)
	out := fanin.Merge[int](context.Background())
	if _, ok := <-out; ok {
		t.Fatal("Merge with no inputs returned an open channel")
	}
}

func TestMergeNilInputPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	defer func() {
		if recover() == nil {
			t.Fatal("Merge with a nil input did not panic")
		}
	}()
	fanin.Merge(context.Background(), nil, make(chan int))
}

func TestMergeCompletenessAndPerInputOrder(t *testing.T) {
	defer goleak.VerifyNone(t)
	const inputs, perInput = 3, 50

	ins := make([]<-chan int, 0, inputs)
	for i := range inputs {
		ch := make(chan int, perInput)
		for seq := range perInput {
			ch <- i*1_000_000 + seq // encode (input, sequence) into the value
		}
		close(ch)
		ins = append(ins, ch)
	}

	out := fanin.Merge(context.Background(), ins...)

	lastSeq := make([]int, inputs)
	for i := range lastSeq {
		lastSeq[i] = -1
	}
	total := 0
	for v := range out {
		input, seq := v/1_000_000, v%1_000_000
		if seq <= lastSeq[input] {
			t.Fatalf("input %d: sequence %d arrived after %d — per-input order broken",
				input, seq, lastSeq[input])
		}
		lastSeq[input] = seq
		total++
	}
	if total != inputs*perInput {
		t.Fatalf("received %d values, want %d — merge is not complete", total, inputs*perInput)
	}
	for i, last := range lastSeq {
		if last != perInput-1 {
			t.Fatalf("input %d: last sequence %d, want %d", i, last, perInput-1)
		}
	}
}

// TestMergeCompletenessProperty drives Merge with rapid-generated topologies of
// concurrent producers and asserts no value is lost, duplicated, or reordered
// within its input. rapid shrinks a counterexample to a minimal failing
// topology (replacing the seeded math/rand property retired in ROADMAP 2.6).
func TestMergeCompletenessProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	rapid.Check(t, func(rt *rapid.T) {
		inputs := rapid.IntRange(1, 8).Draw(rt, "inputs")
		counts := make([]int, inputs)
		for i := range counts {
			counts[i] = rapid.IntRange(0, 100).Draw(rt, "count")
		}

		chans := make([]chan int, inputs)
		ins := make([]<-chan int, inputs)
		for i := range chans {
			chans[i] = make(chan int) // unbuffered: producers and consumer run concurrently
			ins[i] = chans[i]
		}

		var producers sync.WaitGroup
		producers.Add(inputs)
		for i := range chans {
			go func() {
				defer producers.Done()
				defer close(chans[i])
				for seq := range counts[i] {
					chans[i] <- i*1_000_000 + seq
				}
			}()
		}

		out := fanin.Merge(context.Background(), ins...)

		lastSeq := make([]int, inputs)
		for i := range lastSeq {
			lastSeq[i] = -1
		}
		perInput := make([]int, inputs)
		total := 0
		for v := range out {
			input, seq := v/1_000_000, v%1_000_000
			require.Greaterf(rt, seq, lastSeq[input],
				"input %d: per-input order broken", input)
			lastSeq[input] = seq
			perInput[input]++
			total++
		}
		producers.Wait()

		want := 0
		for i, c := range counts {
			want += c
			require.Equalf(rt, c, perInput[i], "input %d received the wrong count", i)
		}
		require.Equalf(rt, want, total, "merge is not complete")
	})
}

func TestCancelUnblocksForwardersAndClosesOutput(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Two values preloaded, input never closed, nobody reading the output:
	// the forwarder ends up blocked mid-send with a value in hand — exactly
	// the state cancellation must be able to unblock.
	in := make(chan int, 2)
	in <- 1
	in <- 2

	ctx, cancel := context.WithCancel(context.Background())
	out := fanin.Merge(ctx, in)

	time.Sleep(20 * time.Millisecond) // let the forwarder reach the blocked send
	cancel()

	drainUntilClosed(t, out, 2*time.Second)
}
