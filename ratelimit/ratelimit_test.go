package ratelimit_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/ratelimit"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestNewLimiterPanicsOnInvalidArguments(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := []struct {
		name  string
		rate  float64
		burst int
	}{
		{"zero rate", 0, 1},
		{"negative rate", -1, 1},
		{"infinite rate", math.Inf(1), 1},
		{"NaN rate", math.NaN(), 1},
		{"zero burst", 1, 0},
		{"negative burst", 1, -1},
	}
	for _, tc := range cases {
		require.Panicsf(t, func() { ratelimit.NewLimiter(tc.rate, tc.burst) }, "case %q", tc.name)
	}
}

func TestAllowAdmitsTheBurstThenDenies(t *testing.T) {
	defer goleak.VerifyNone(t)
	// 1 token/s: the refill accrued during a microsecond-scale test is
	// negligible, so the fourth call must see an empty bucket.
	l := ratelimit.NewLimiter(1, 3)
	for i := range 3 {
		require.Truef(t, l.Allow(), "call %d is within the burst", i)
	}
	require.False(t, l.Allow(), "the burst is spent")
}

func TestWaitAdmitsImmediatelyWithinBurst(t *testing.T) {
	defer goleak.VerifyNone(t)
	l := ratelimit.NewLimiter(1, 2)
	ctx := context.Background()
	require.NoError(t, l.Wait(ctx))
	require.NoError(t, l.Wait(ctx))
}

func TestWaitBlocksUntilTheNextTokenIsFunded(t *testing.T) {
	defer goleak.VerifyNone(t)
	l := ratelimit.NewLimiter(50, 1) // 20ms per token
	require.True(t, l.Allow())

	start := time.Now()
	require.NoError(t, l.Wait(context.Background()))
	require.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond,
		"Wait must sleep until the token is funded")
}

func TestWaitPreCanceledConsumesNothing(t *testing.T) {
	defer goleak.VerifyNone(t)
	l := ratelimit.NewLimiter(1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, l.Wait(ctx), context.Canceled)
	require.True(t, l.Allow(), "a Wait that never started must not cost a token")
}

func TestWaitCancellationMidBlockReturnsPromptly(t *testing.T) {
	defer goleak.VerifyNone(t)
	l := ratelimit.NewLimiter(0.1, 1) // 10s per token: only cancellation can end the wait
	require.True(t, l.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, l.Wait(ctx), context.DeadlineExceeded)
}

func TestConcurrentUseIsRaceClean(t *testing.T) {
	defer goleak.VerifyNone(t)
	l := ratelimit.NewLimiter(1000, 4)

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for i := range 200 {
				if i%2 == 0 {
					l.Allow()
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				err := l.Wait(ctx)
				cancel()
				if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
					t.Errorf("unexpected error: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// Benchmarks — spec §6 lists ratelimit in the benchmark set; results are
// recorded under docs/benchmarks/.

func BenchmarkAllow(b *testing.B) {
	// 1e12 tokens/s refills the single-token bucket every nanosecond: the
	// admit path runs on every iteration.
	l := ratelimit.NewLimiter(1e12, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		l.Allow()
	}
}

func BenchmarkAllowSaturated(b *testing.B) {
	// One token per ~32 years: after the drain, the deny path runs on every
	// iteration.
	l := ratelimit.NewLimiter(1e-9, 1)
	l.Allow()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		l.Allow()
	}
}

func BenchmarkAllowParallel(b *testing.B) {
	l := ratelimit.NewLimiter(1e12, 1)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Allow()
		}
	})
}

func BenchmarkWaitFunded(b *testing.B) {
	// Tokens are always available: measures the no-sleep Wait fast path.
	// The burst absorbs the calls that land inside one clock tick (where
	// elapsed time, and therefore refill, is zero) so no iteration is
	// pushed onto the timer path.
	l := ratelimit.NewLimiter(1e12, 1024)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = l.Wait(ctx)
	}
}
