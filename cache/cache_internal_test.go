package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// fakeNow gives a test a hand-cranked clock: expiry becomes a pure function of
// the injected time, with no sleeping (the ADR-0010 deterministic-clock idiom).
type fakeNow struct{ t time.Time }

func (f *fakeNow) now() time.Time          { return f.t }
func (f *fakeNow) advance(d time.Duration) { f.t = f.t.Add(d) }

// newFakeCache builds a cache on the fake clock. The caller defers Close
// itself (after its goleak defer, so Close runs first — defers are LIFO).
func newFakeCache(t *testing.T, ttl time.Duration) (*Cache[string, int], *fakeNow) {
	t.Helper()
	c := NewInMemory[string, int](ttl, WithCleanupInterval(time.Hour))
	clk := &fakeNow{t: time.Unix(1_000_000, 0)}
	c.now = clk.now
	return c, clk
}

func (c *Cache[K, V]) length() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func TestExpiryBoundary(t *testing.T) {
	defer goleak.VerifyNone(t)
	c, clk := newFakeCache(t, time.Minute)
	defer c.Close()
	c.Set("k", 1)

	clk.advance(time.Minute - time.Nanosecond)
	_, err := c.Get("k")
	require.NoError(t, err, "one nanosecond before the deadline the entry is live")

	clk.advance(time.Nanosecond) // exactly at the deadline
	_, err = c.Get("k")
	require.ErrorIs(t, err, ErrNotFound, "at the deadline the entry is expired (deadline is exclusive)")
}

func TestRemoveExpiredSweepsOnlyExpired(t *testing.T) {
	defer goleak.VerifyNone(t)
	c, clk := newFakeCache(t, time.Minute)
	defer c.Close()
	c.Set("old", 1)
	clk.advance(30 * time.Second)
	c.Set("young", 2) // deadline 30s later than old's

	clk.advance(30 * time.Second) // old is exactly at its deadline; young has 30s left
	c.removeExpired()

	require.Equal(t, 1, c.length(), "only the expired entry is reclaimed")
	_, err := c.Get("young")
	require.NoError(t, err)
	_, err = c.Get("old")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSweeperReclaimsInBackground(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Real clock, tight interval: the sweeper (not Get) must shrink the map.
	c := NewInMemory[string, int](10*time.Millisecond, WithCleanupInterval(5*time.Millisecond))
	defer c.Close()
	for i := range 32 {
		c.Set(string(rune('a'+i)), i)
	}
	require.Eventually(t, c.isEmpty, time.Second, 5*time.Millisecond,
		"the sweeper must reclaim expired entries without any Get")
}

func (c *Cache[K, V]) isEmpty() bool { return c.length() == 0 }
