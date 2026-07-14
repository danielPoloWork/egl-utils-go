package cache_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/cache"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestNewInMemoryNonPositiveTTLPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "cache: non-positive TTL", func() {
		cache.NewInMemory[string, int](0)
	})
	require.PanicsWithValue(t, "cache: non-positive TTL", func() {
		cache.NewInMemory[string, int](-time.Second)
	})
}

func TestWithCleanupIntervalNonPositivePanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "cache: non-positive cleanup interval", func() {
		cache.WithCleanupInterval(0)
	})
}

func TestSetGetDelete(t *testing.T) {
	defer goleak.VerifyNone(t)
	c := cache.NewInMemory[string, int](time.Minute)
	defer c.Close()

	_, err := c.Get("k")
	require.ErrorIs(t, err, cache.ErrNotFound, "a missing key is ErrNotFound")

	c.Set("k", 42)
	v, err := c.Get("k")
	require.NoError(t, err)
	require.Equal(t, 42, v)

	c.Set("k", 7) // overwrite
	v, err = c.Get("k")
	require.NoError(t, err)
	require.Equal(t, 7, v)

	c.Delete("k")
	_, err = c.Get("k")
	require.ErrorIs(t, err, cache.ErrNotFound)

	c.Delete("absent") // no-op, no panic
}

func TestGetRefusesExpiredEntry(t *testing.T) {
	defer goleak.VerifyNone(t)
	// A long cleanup interval guarantees the sweeper cannot be the reason the
	// entry disappears: Get itself must refuse it once the TTL passes.
	c := cache.NewInMemory[string, int](20*time.Millisecond, cache.WithCleanupInterval(time.Hour))
	defer c.Close()

	c.Set("k", 1)
	v, err := c.Get("k")
	require.NoError(t, err)
	require.Equal(t, 1, v)

	require.Eventually(t, func() bool {
		_, err := c.Get("k")
		return err != nil
	}, time.Second, 5*time.Millisecond, "the entry must expire without the sweeper's help")
}

func TestSetResetsTTL(t *testing.T) {
	defer goleak.VerifyNone(t)
	c := cache.NewInMemory[string, int](50*time.Millisecond, cache.WithCleanupInterval(time.Hour))
	defer c.Close()

	c.Set("k", 1)
	// Keep re-setting past the original deadline; the entry must stay live
	// because every Set grants a fresh TTL.
	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		c.Set("k", 1)
		_, err := c.Get("k")
		require.NoError(t, err, "a re-set entry must not expire on the old deadline")
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCloseIsIdempotentAndConcurrent(t *testing.T) {
	defer goleak.VerifyNone(t)
	c := cache.NewInMemory[string, int](time.Minute)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Close()
		}()
	}
	wg.Wait()
	c.Close() // and once more, sequentially
}

func TestCacheUsableAfterClose(t *testing.T) {
	defer goleak.VerifyNone(t)
	c := cache.NewInMemory[string, int](time.Minute)
	c.Set("k", 1)
	c.Close()

	v, err := c.Get("k")
	require.NoError(t, err)
	require.Equal(t, 1, v)
	c.Set("j", 2)
	c.Delete("k")
}

func TestConcurrentAccess(t *testing.T) {
	defer goleak.VerifyNone(t)
	// A mixed-operation hammer; the -race CI job is the real assertion here.
	c := cache.NewInMemory[string, int](time.Minute, cache.WithCleanupInterval(time.Millisecond))
	defer c.Close()

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				key := strconv.Itoa((g + i) % 16)
				switch i % 3 {
				case 0:
					c.Set(key, i)
				case 1:
					_, _ = c.Get(key)
				default:
					c.Delete(key)
				}
			}
		}()
	}
	wg.Wait()
}

func TestCloseStopsSweeper(t *testing.T) {
	// No deferred goleak here — this test IS the leak assertion: after Close,
	// VerifyNone must see no sweeper goroutine even mid-interval.
	c := cache.NewInMemory[string, int](time.Minute, cache.WithCleanupInterval(time.Hour))
	c.Set("k", 1)
	c.Close()
	goleak.VerifyNone(t)
}
