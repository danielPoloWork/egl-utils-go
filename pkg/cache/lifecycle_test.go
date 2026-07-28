package cache_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/cache"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// thousandCaches is the scale spec v2 item 17 asks the cache lifecycle to be
// proven at. A process legitimately holds many caches — one per tenant, per
// resource type, per request-scoped registry — so "the sweeper stops" has to hold
// a thousand times over, not once.
const thousandCaches = 1000

// newTestCache builds a cache whose sweeper will not fire during the test: the
// point of these tests is the goroutine's lifecycle, not its work.
func newTestCache(t *testing.T) *cache.Cache[string, int] {
	t.Helper()
	return cache.NewInMemory[string, int](time.Minute, cache.WithCleanupInterval(time.Hour))
}

// TestThousandCachesCreateAndClose is the headline lifecycle assertion: a
// thousand caches created, used, and closed leave nothing behind.
//
// Close is called explicitly before the deferred goleak.VerifyNone. Registering
// it with t.Cleanup instead would run it *after* goleak — the trap recorded since
// roadmap 7.1 — and goleak would then see a thousand live sweepers and fail for
// the wrong reason.
func TestThousandCachesCreateAndClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	caches := make([]*cache.Cache[string, int], 0, thousandCaches)
	for i := range thousandCaches {
		c := newTestCache(t)
		c.Set(fmt.Sprintf("key-%d", i), i)
		caches = append(caches, c)
	}

	// Every cache is independently functional, not merely constructed.
	for i, c := range caches {
		got, err := c.Get(fmt.Sprintf("key-%d", i))
		require.NoErrorf(t, err, "cache %d lost its entry", i)
		require.Equal(t, i, got)
	}

	for _, c := range caches {
		c.Close()
	}
}

// TestThousandCachesOwnOneGoroutineEach pins the property sharding could have
// broken. A cache is split across shardCount internally locked shards; if each
// shard had its own sweeper, a thousand caches would cost tens of thousands of
// goroutines. The contract is one sweeper per *cache*, whatever the shard count.
func TestThousandCachesOwnOneGoroutineEach(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Let any goroutine left over from earlier tests settle, so the baseline is
	// not polluted by something else's teardown.
	runtime.GC()
	before := runtime.NumGoroutine()

	caches := make([]*cache.Cache[string, int], 0, thousandCaches)
	for range thousandCaches {
		caches = append(caches, newTestCache(t))
	}
	delta := runtime.NumGoroutine() - before

	for _, c := range caches {
		c.Close()
	}

	// Asserted as a band rather than exactly thousandCaches: the runtime may have
	// its own goroutines coming and going around the measurement. The band is
	// narrow enough to distinguish one-per-cache from one-per-shard, which is the
	// only thing this test is about — per-shard sweepers would be 32 000.
	require.GreaterOrEqualf(t, delta, thousandCaches,
		"expected at least one sweeper per cache, saw %d new goroutines", delta)
	require.Lessf(t, delta, 2*thousandCaches,
		"saw %d new goroutines for %d caches — a cache must own exactly one sweeper "+
			"regardless of how many shards it has", delta, thousandCaches)
}

// TestThousandCachesConcurrentCreateAndClose runs the lifecycle from many
// goroutines at once, which is how it happens in a real process: caches are
// created and discarded on request paths, not in a tidy sequential loop.
func TestThousandCachesConcurrentCreateAndClose(t *testing.T) {
	defer goleak.VerifyNone(t)

	const workers = 16
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range thousandCaches / workers {
				c := cache.NewInMemory[string, int](time.Minute, cache.WithCleanupInterval(time.Hour))
				key := fmt.Sprintf("w%d-k%d", w, i)
				c.Set(key, i)
				if _, err := c.Get(key); err != nil {
					t.Errorf("worker %d: entry %s missing: %v", w, key, err)
				}
				c.Close()
			}
		}(w)
	}
	wg.Wait()
}

// TestThousandCachesRepeatedCloseIsIdempotent closes each cache several times
// from several goroutines. Close is guarded by a sync.Once, so a second close of
// the done channel — which would panic — must be impossible even under a
// thousand-fold race.
func TestThousandCachesRepeatedCloseIsIdempotent(t *testing.T) {
	defer goleak.VerifyNone(t)

	caches := make([]*cache.Cache[string, int], 0, thousandCaches)
	for range thousandCaches {
		caches = append(caches, newTestCache(t))
	}

	var wg sync.WaitGroup
	for _, c := range caches {
		for range 3 { // three concurrent closers per cache
			wg.Add(1)
			go func(c *cache.Cache[string, int]) {
				defer wg.Done()
				c.Close()
			}(c)
		}
	}
	wg.Wait()

	// A closed cache stays usable — Get still refuses expired entries — which is
	// the graceful posture ADR-0021 chose over a loud one.
	for i, c := range caches {
		key := fmt.Sprintf("after-close-%d", i)
		c.Set(key, i)
		got, err := c.Get(key)
		require.NoErrorf(t, err, "cache %d unusable after Close", i)
		require.Equal(t, i, got)
	}
}

// TestShardedKeysAllRoundTrip guards the sharding itself: every key must land in
// a shard and be found again there. A hashing mistake — a bad mask, a seed read
// per call instead of per cache — would show up as keys that vanish, and at a
// thousand keys it would show up reliably.
func TestShardedKeysAllRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t)
	c := newTestCache(t)
	defer c.Close()

	const keys = 10_000
	for i := range keys {
		c.Set(fmt.Sprintf("key-%d", i), i)
	}
	for i := range keys {
		got, err := c.Get(fmt.Sprintf("key-%d", i))
		require.NoErrorf(t, err, "key-%d did not survive sharding", i)
		require.Equal(t, i, got)
	}
	for i := range keys {
		c.Delete(fmt.Sprintf("key-%d", i))
	}
	for i := range keys {
		_, err := c.Get(fmt.Sprintf("key-%d", i))
		require.ErrorIsf(t, err, cache.ErrNotFound, "key-%d survived Delete", i)
	}
}
