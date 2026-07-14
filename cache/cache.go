// Package cache provides a generic in-memory key-value cache with per-cache
// TTL expiry and a periodic cleanup goroutine.
//
// Every entry expires ttl after it was Set. An expired entry is never returned
// — Get reports it as a miss the instant its deadline passes, regardless of
// when the sweeper last ran — so the cleanup goroutine is purely a memory
// reclaimer, not a correctness mechanism. The cache owns exactly one goroutine
// (the sweeper); Close stops it deterministically, honouring the module's
// zero-goroutine-leak philosophy (goleak-verified).
package cache

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned by Get when the key is absent or its entry has
// expired.
var ErrNotFound = errors.New("cache: not found")

type options struct {
	cleanupInterval time.Duration
}

// Option configures NewInMemory.
type Option func(*options)

// WithCleanupInterval sets how often the sweeper scans for expired entries
// (default: the cache's ttl). A shorter interval reclaims memory sooner at the
// cost of more frequent scans; correctness is unaffected either way, because
// Get never returns an expired entry. It panics if d is not positive — a
// configuration error, caught at wiring (ADR-0005 idiom).
func WithCleanupInterval(d time.Duration) Option {
	if d <= 0 {
		panic("cache: non-positive cleanup interval")
	}
	return func(o *options) { o.cleanupInterval = d }
}

// entry is one stored value and its expiry deadline.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// Cache is a thread-safe in-memory TTL cache. Create it with NewInMemory; the
// zero value is not usable.
type Cache[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]entry[V]

	ttl time.Duration
	now func() time.Time // injectable for deterministic expiry tests

	done      chan struct{} // closed by Close; stops the sweeper
	closeOnce sync.Once
}

// NewInMemory returns a cache whose entries expire ttl after they are set, and
// starts the single cleanup goroutine that reclaims expired entries every
// cleanup interval (default ttl, override with WithCleanupInterval). Call
// Close when the cache is no longer needed, or the sweeper goroutine lives for
// the life of the process. NewInMemory panics if ttl is not positive — a
// cache in which nothing may live has no meaning, and the loud failure points
// at the wiring bug (ADR-0005 idiom).
func NewInMemory[K comparable, V any](ttl time.Duration, opts ...Option) *Cache[K, V] {
	if ttl <= 0 {
		panic("cache: non-positive TTL")
	}
	o := options{cleanupInterval: ttl}
	for _, opt := range opts {
		opt(&o)
	}
	c := &Cache[K, V]{
		entries: make(map[K]entry[V]),
		ttl:     ttl,
		now:     time.Now,
		done:    make(chan struct{}),
	}
	go c.sweeper(o.cleanupInterval)
	return c
}

// Set stores value under key, resetting its lifetime to a full TTL. An
// existing entry is overwritten.
func (c *Cache[K, V]) Set(key K, value V) {
	deadline := c.now().Add(c.ttl)
	c.mu.Lock()
	c.entries[key] = entry[V]{value: value, expiresAt: deadline}
	c.mu.Unlock()
}

// Get returns the live value stored under key, or the zero V and ErrNotFound
// when the key is absent or its entry has expired — expiry is judged against
// the deadline at call time, never against the sweeper's schedule.
func (c *Cache[K, V]) Get(key K) (V, error) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !c.now().Before(e.expiresAt) {
		var zero V
		return zero, ErrNotFound
	}
	return e.value, nil
}

// Delete removes key from the cache; deleting an absent key is a no-op.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

// Close stops the cleanup goroutine. It is idempotent and safe to call
// concurrently. The cache remains usable afterwards — Get still refuses
// expired entries — but expired memory is no longer reclaimed in the
// background, so a closed cache should be left to the garbage collector.
func (c *Cache[K, V]) Close() {
	c.closeOnce.Do(func() { close(c.done) })
}

// sweeper deletes expired entries every interval until Close. It is the
// cache's only goroutine; the ticker is stopped on the way out.
func (c *Cache[K, V]) sweeper(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			c.removeExpired()
		}
	}
}

// removeExpired deletes every entry whose deadline has passed, in one pass
// under the write lock.
func (c *Cache[K, V]) removeExpired() {
	now := c.now()
	c.mu.Lock()
	for k, e := range c.entries {
		if !now.Before(e.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}
