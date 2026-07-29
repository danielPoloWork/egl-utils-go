package pubsub_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/pubsub"
)

// NFR-03 (spec v2 §5): pubsub fans out ≥ 500 k msgs/s to 10 subscribers with a
// 64 B payload.
//
// The measurement trap this benchmark is built around: Publish never blocks on a
// slow subscriber — a full buffer simply drops the message (ADR-0006). So a
// benchmark that publishes into undrained subscriptions measures the *drop* path
// and reports a spectacular figure for delivering nothing. Two safeguards:
// ten real drainer goroutines run for the whole measurement, and both delivered
// messages and drops are counted, so the reported throughput is *delivery*
// throughput and any drop is visible rather than silently inflating it.
//
// Every subscription here is made with context.Background(), deliberately: the
// drainers end when Close shuts their channels, so the broker's lifetime — not a
// per-subscription context — is what bounds the measurement (ADR-0049).
// Publish's error is discarded rather than counted because the drop handler
// already counts the individual losses, and the accounting assertion below is
// the stronger check; ErrSlowSubscriber would only restate it per publish.

const (
	nfrSubscribers = 10
	nfrPayloadSize = 64
	nfrTopic       = "orders"
)

// BenchmarkNFR03FanOut measures delivered fan-out throughput to 10 subscribers.
//
// Timing spans publish *and* drain: Publish alone is a non-blocking channel send,
// so publisher-side throughput would overstate what the system delivers. Close
// shuts the subscription channels, which is what lets the drainers finish and be
// waited on inside the measured window.
func BenchmarkNFR03FanOut(b *testing.B) {
	var delivered, drops atomic.Int64

	br := pubsub.NewBroker[[]byte](
		// A deep buffer keeps the drop path out of the steady state; drops are
		// counted anyway, because "no drops" must be demonstrated, not assumed.
		pubsub.WithSubscriberBuffer[[]byte](4096),
		pubsub.WithDropHandler[[]byte](func(string, []byte) { drops.Add(1) }),
	)

	ctx := context.Background()

	var wg sync.WaitGroup
	for range nfrSubscribers {
		ch := br.Subscribe(ctx, nfrTopic, nil)
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := int64(0)
			for range ch { // ends when Close shuts the channel
				n++
			}
			delivered.Add(n)
		}()
	}

	payload := make([]byte, nfrPayloadSize)

	b.ResetTimer()
	for range b.N {
		_ = br.Publish(ctx, nfrTopic, payload)
	}
	br.Close()
	wg.Wait()
	elapsed := b.Elapsed()
	b.StopTimer()

	got, lost := delivered.Load(), drops.Load()
	want := int64(b.N) * nfrSubscribers
	if got+lost != want {
		b.Fatalf("accounting is wrong: delivered %d + dropped %d != %d expected deliveries",
			got, lost, want)
	}
	b.ReportMetric(float64(got)/elapsed.Seconds(), "delivered/s")
	b.ReportMetric(float64(lost)/float64(b.N), "drops/publish")
}

// BenchmarkNFR03PublishOnly isolates the publisher's own cost — the fan-out loop
// under the read lock, ten non-blocking channel sends — with the drainers still
// running. It is reported separately so a regression can be attributed to the
// broker rather than to scheduling of the consumers.
func BenchmarkNFR03PublishOnly(b *testing.B) {
	var drops atomic.Int64
	br := pubsub.NewBroker[[]byte](
		pubsub.WithSubscriberBuffer[[]byte](4096),
		pubsub.WithDropHandler[[]byte](func(string, []byte) { drops.Add(1) }),
	)

	ctx := context.Background()

	var wg sync.WaitGroup
	for range nfrSubscribers {
		ch := br.Subscribe(ctx, nfrTopic, nil)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range ch { //nolint:revive // draining is the point; the value is unused
			}
		}()
	}

	payload := make([]byte, nfrPayloadSize)

	b.ResetTimer()
	for range b.N {
		_ = br.Publish(ctx, nfrTopic, payload)
	}
	b.StopTimer()

	br.Close()
	wg.Wait()
	b.ReportMetric(float64(drops.Load())/float64(b.N), "drops/publish")
}

// BenchmarkNFR03FanOutFiltered adds a per-subscription filter that accepts
// everything, measuring what the filter call itself costs on the delivery path.
func BenchmarkNFR03FanOutFiltered(b *testing.B) {
	var drops atomic.Int64
	br := pubsub.NewBroker[[]byte](
		pubsub.WithSubscriberBuffer[[]byte](4096),
		pubsub.WithDropHandler[[]byte](func(string, []byte) { drops.Add(1) }),
	)

	ctx := context.Background()

	var wg sync.WaitGroup
	acceptAll := func([]byte) bool { return true }
	for range nfrSubscribers {
		ch := br.Subscribe(ctx, nfrTopic, acceptAll)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range ch { //nolint:revive // draining is the point
			}
		}()
	}

	payload := make([]byte, nfrPayloadSize)

	b.ResetTimer()
	for range b.N {
		_ = br.Publish(ctx, nfrTopic, payload)
	}
	b.StopTimer()

	br.Close()
	wg.Wait()
	b.ReportMetric(float64(drops.Load())/float64(b.N), "drops/publish")
}
