package cache_test

import (
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/cache"
)

// NFR-06 (spec v2 §5): cache.InMemory's Get p99 is ≤ 200 ns at 1 M entries with
// a 90/10 read/write mix across 8 goroutines.
//
// Roadmap 10.11 (cache hardening) uses these numbers to decide whether internal
// sharding is warranted — "shard only if the bench demands" — so the benchmark is
// built here and the decision is taken there, on this evidence.

const (
	nfrEntries     = 1_000_000
	nfrGoroutines  = 8
	nfrWritePermil = 100 // 100/1000 = 10% writes, the NFR's 90/10 mix
	nfrTTL         = time.Hour
	getBatch       = 1000 // ~30 ns/op ⇒ ~30 µs per batch; see reportTail
)

// percentile, clockResolution and reportTail duplicate the workerpool NFR
// benchmark rather than being shared: a helper package would add a package to the
// module — and so to the coverage gate and the import-graph rules — to save a few
// lines. See the workerpool copy for why the clock resolution matters at all.
func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[int(float64(len(samples)-1)*p)]
}

func clockResolution() time.Duration {
	best := time.Hour
	for range 20 {
		start := time.Now()
		var d time.Duration
		for d == 0 {
			d = time.Since(start)
		}
		if d < best {
			best = d
		}
	}
	return best
}

// reportTail publishes the batch-derived tail, or refuses when the clock is too
// coarse for the samples to mean anything. NFR-06's 200 ns target is three to
// four orders of magnitude below the Windows clock's resolution under load, so
// locally this reports tail-unmeasurable and CI (Linux) is the authority.
func reportTail(b *testing.B, samples []time.Duration, batch int) {
	b.Helper()
	res := clockResolution()
	b.ReportMetric(float64(res.Nanoseconds()), "clock-res-ns")
	if len(samples) == 0 {
		return
	}
	// Quantization is detected from the spacing of the samples themselves; see
	// observedTick for why the probe above is reported but not trusted for this.
	median := percentile(samples, 0.50) // sorts in place, which observedTick needs
	tick := observedTick(samples)
	b.ReportMetric(float64(tick.Nanoseconds()), "observed-tick-ns")
	if tick*20 > median {
		b.ReportMetric(1, "tail-unmeasurable")
		return
	}
	b.ReportMetric(float64(percentile(samples, 0.50).Nanoseconds())/float64(batch), "p50-ns/op")
	b.ReportMetric(float64(percentile(samples, 0.99).Nanoseconds())/float64(batch), "p99-ns/op")
}

// observedTick estimates the clock granularity that actually applied *during the
// measurement*, as the **median** positive gap between adjacent sorted samples.
// samples must already be sorted.
//
// Two earlier attempts were unsound, and the reason is worth keeping: the
// Windows monotonic clock is bimodal under load. A busy-spin probe reads a fine
// 3-12 us, and occasional adjacent samples differ by the 100 ns QPC tick, yet
// most spaced Now calls do not advance at all for ~1 ms -- so a great many
// batches time as exactly 0 while others land on multiples of a millisecond.
// Checking a probed resolution missed it, counting distinct values missed it
// (hundreds of batches on *different* multiples of one tick give hundreds of
// distinct values), and taking the *smallest* gap missed it too (one lucky
// 100 ns pair is enough). The median gap describes the granularity that actually
// governed the data, which is the question being asked.
func observedTick(samples []time.Duration) time.Duration {
	gaps := make([]time.Duration, 0, len(samples))
	for i := 1; i < len(samples); i++ {
		if gap := samples[i] - samples[i-1]; gap > 0 {
			gaps = append(gaps, gap)
		}
	}
	if len(gaps) == 0 {
		// Every sample identical: maximally quantized.
		return time.Hour
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	return gaps[len(gaps)/2]
}

// parallelismFor converts a desired goroutine count into the multiplier
// RunParallel expects: it launches parallelism x GOMAXPROCS goroutines, so on a
// 4-core box 8 goroutines means a multiplier of 2. Never below 1, which would
// launch none.
func parallelismFor(goroutines int) int {
	p := goroutines / runtime.GOMAXPROCS(0)
	if p < 1 {
		return 1
	}
	return p
}

// newLoadedCache returns a cache holding nfrEntries entries. The TTL is an hour
// so nothing expires mid-run and the sweeper never fires: the benchmark measures
// Get and Set, not eviction.
func newLoadedCache(b *testing.B) *cache.Cache[int, int] {
	b.Helper()
	c := cache.New[int, int](nfrTTL)
	for i := range nfrEntries {
		c.Set(i, i)
	}
	return c
}

// xorshift is a cheap per-goroutine PRNG. Key selection must be spread across the
// whole keyspace — a sequential scan would measure a CPU-cache-friendly access
// pattern that no real workload has — and math/rand's global source would add
// lock contention to the thing being measured.
type xorshift uint64

func (x *xorshift) next() uint64 {
	v := *x
	v ^= v << 13
	v ^= v >> 7
	v ^= v << 17
	*x = v
	return uint64(v)
}

// BenchmarkNFR06Mixed is the NFR-06 headline: the 90/10 mix at 1 M entries across
// 8 goroutines. The mean here is the trustworthy local figure — the framework
// times the whole loop, so clock quantization averages out over millions of
// iterations even where per-operation sampling cannot work.
func BenchmarkNFR06Mixed(b *testing.B) {
	c := newLoadedCache(b)
	defer c.Close()

	b.SetParallelism(parallelismFor(nfrGoroutines))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rng := xorshift(0x9E3779B97F4A7C15)
		for pb.Next() {
			k := int(rng.next() % nfrEntries)
			if rng.next()%1000 < nfrWritePermil {
				c.Set(k, k)
				continue
			}
			_, _ = c.Get(k)
		}
	})
}

// BenchmarkNFR06GetOnly isolates the read path at 1 M entries — the number
// NFR-06's 200 ns target is actually about, with no writer contending.
func BenchmarkNFR06GetOnly(b *testing.B) {
	c := newLoadedCache(b)
	defer c.Close()

	b.SetParallelism(parallelismFor(nfrGoroutines))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rng := xorshift(0x9E3779B97F4A7C15)
		for pb.Next() {
			_, _ = c.Get(int(rng.next() % nfrEntries))
		}
	})
}

// BenchmarkNFR06GetTail samples the read path's tail under the 90/10 mix at the
// NFR's eight goroutines.
//
// **What this number is.** A batch is timed in wall clock from inside one
// goroutine, so when the goroutines outnumber the cores it measures *residency*
// — service time plus the time the goroutine spent runnable-but-descheduled —
// not the latency of a `Get`. The relationship is arithmetic, not a suspicion:
// with every core saturated, a per-goroutine batch mean equals the aggregate
// ns/op times the goroutine count. On a 4-core runner the 2026-08-06 CI run
// measured 97.1 ns/op aggregate on this mix and a p50 batch mean of 743 ns/op,
// which is 97.1 × 8 less the gap between a median and a mean.
//
// So this benchmark answers "what does a goroutine in this workload wait, end to
// end" and is the right regression detector for the NFR's stated load. It is
// *not* comparable to NFR-06's 200 ns target unless the runner has eight cores to
// give it. BenchmarkNFR06GetTailPerCore below is the service-time figure that is.
func BenchmarkNFR06GetTail(b *testing.B) {
	benchGetTail(b, parallelismFor(nfrGoroutines))
}

// BenchmarkNFR06GetTailPerCore samples the same 90/10 mix with exactly one
// goroutine per core, which is the only way to read a service time off a
// wall-clock batch: with no goroutine oversubscribed there is no scheduler
// queueing folded into the sample.
//
// The trade is stated rather than hidden. This is *not* NFR-06's load — the NFR
// says eight goroutines and this runs GOMAXPROCS of them — so on a runner with
// fewer than eight cores it measures the tail of a lighter mix, and lock
// contention on the shards (ADR-0038) is correspondingly lower. It is a
// diagnostic that isolates the code's own tail from the machine's scheduling,
// which is what makes the residency figure above interpretable.
//
// It is also the number that settled NFR-06's verdict, in the direction nobody
// expected: 887 ns at the p99 against a 200 ns target with no oversubscription at
// all, so the shortfall is not the runner's missing four cores. Note too that
// four goroutines measure *worse* aggregate throughput than eight (117.3 against
// 96.4 ns/op) — random access over 1 M entries stalls on memory, and
// oversubscribing hides those stalls behind other goroutines' work. Throughput
// and latency pull in opposite directions here, which is the whole reason these
// two benchmarks are separate.
func BenchmarkNFR06GetTailPerCore(b *testing.B) {
	benchGetTail(b, 1)
}

// benchGetTail runs the 90/10 mix at parallelism × GOMAXPROCS goroutines and
// reports the batch-derived tail.
//
// Samples are collected per goroutine and merged once at the end, so the
// collection itself adds no cross-goroutine contention to the measurement. The
// goroutine and core counts are reported alongside the percentiles because,
// per the note on BenchmarkNFR06GetTail, their ratio is what decides whether a
// batch mean is a service time or a residency time — a reader of the raw
// benchmark output should not have to reconstruct it.
func benchGetTail(b *testing.B, parallelism int) {
	b.Helper()
	c := newLoadedCache(b)
	defer c.Close()

	var (
		mu  sync.Mutex
		all []time.Duration
	)

	procs := runtime.GOMAXPROCS(0)
	b.SetParallelism(parallelism)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rng := xorshift(0x9E3779B97F4A7C15)
		local := make([]time.Duration, 0, 1024)
		for {
			start := time.Now()
			done := false
			n := 0
			for ; n < getBatch; n++ {
				if !pb.Next() {
					done = true
					break
				}
				k := int(rng.next() % nfrEntries)
				if rng.next()%1000 < nfrWritePermil {
					c.Set(k, k)
					continue
				}
				_, _ = c.Get(k)
			}
			if n == getBatch { // only whole batches are comparable
				local = append(local, time.Since(start))
			}
			if done {
				break
			}
		}
		mu.Lock()
		all = append(all, local...)
		mu.Unlock()
	})
	b.StopTimer()

	b.ReportMetric(float64(parallelism*procs), "goroutines")
	b.ReportMetric(float64(procs), "procs")
	reportTail(b, all, getBatch)
}
