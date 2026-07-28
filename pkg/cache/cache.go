// Package cache provides a generic in-memory key-value cache with per-cache
// TTL expiry and a periodic cleanup goroutine.
//
// Every entry expires ttl after it was Set. An expired entry is never returned
// — Get reports it as a miss the instant its deadline passes, regardless of
// when the sweeper last ran — so the cleanup goroutine is purely a memory
// reclaimer, not a correctness mechanism. The cache owns exactly one goroutine
// (the sweeper) however many shards it has; Close stops it deterministically,
// honouring the module's zero-goroutine-leak philosophy (goleak-verified).
//
// Entries are distributed across a fixed number of internally locked shards, so
// a write blocks only the readers touching the same shard rather than every
// reader in the cache. That is invisible in the API — key ordering was never
// promised and iteration is not offered — and it is what makes the read path
// hold up under a mixed read/write load (ADR-0038, NFR-06).
package cache

import (
	"errors"
	"hash/maphash"
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

// shardCount is how many independently locked maps a cache is split across. It
// must be a power of two, so the shard index is a mask rather than a modulo.
//
// 32 is chosen to comfortably exceed the number of cores a single cache is likely
// to be hammered from, which is what keeps the chance of two concurrent writers
// colliding on one shard low, while staying small enough that the per-cache
// overhead is negligible — an empty Go map allocates its header and no buckets,
// so 32 of them cost on the order of a kilobyte and a cache holding nothing stays
// cheap. That matters: a process can hold thousands of caches, which the
// thousand-cache lifecycle test pins.
const shardCount = 32

// shard is one lock and the entries it guards.
//
// No cache-line padding between shards: the measured win from sharding alone is
// large (see ADR-0038), and padding would trade real memory in every cache
// against a false-sharing effect this workload has not been shown to suffer.
// Measure before adding it.
type shard[K comparable, V any] struct {
	mu      sync.RWMutex
	entries map[K]entry[V]
}

// Cache is a thread-safe in-memory TTL cache. Create it with NewInMemory; the
// zero value is not usable.
type Cache[K comparable, V any] struct {
	shards [shardCount]shard[K, V]
	// seed is per-cache, so shard assignment differs between caches and cannot
	// be predicted from outside — a caller cannot craft keys that pile onto one
	// cache's single shard.
	seed maphash.Seed

	ttl time.Duration
	now func() time.Time // injectable for deterministic expiry tests

	done      chan struct{} // closed by Close; stops the sweeper
	closeOnce sync.Once
}

// shardFor returns the shard owning key.
//
// maphash.Comparable hashes any comparable type, which is exactly the constraint
// K already carries. It panics only where using the value as a map key would
// panic anyway — an interface-typed K holding a non-comparable dynamic value —
// so this introduces no failure mode the map underneath did not already have.
func (c *Cache[K, V]) shardFor(key K) *shard[K, V] {
	return &c.shards[maphash.Comparable(c.seed, key)&(shardCount-1)]
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
		seed: maphash.MakeSeed(),
		ttl:  ttl,
		now:  time.Now,
		done: make(chan struct{}),
	}
	for i := range c.shards {
		c.shards[i].entries = make(map[K]entry[V])
	}
	go c.sweeper(o.cleanupInterval)
	return c
}

// Set stores value under key, resetting its lifetime to a full TTL. An
// existing entry is overwritten.
func (c *Cache[K, V]) Set(key K, value V) {
	deadline := c.now().Add(c.ttl) // computed outside the lock
	s := c.shardFor(key)
	s.mu.Lock()
	s.entries[key] = entry[V]{value: value, expiresAt: deadline}
	s.mu.Unlock()
}

// Get returns the live value stored under key, or the zero V and ErrNotFound
// when the key is absent or its entry has expired — expiry is judged against
// the deadline at call time, never against the sweeper's schedule.
func (c *Cache[K, V]) Get(key K) (V, error) {
	s := c.shardFor(key)
	s.mu.RLock()
	e, ok := s.entries[key]
	s.mu.RUnlock()
	// The deadline comparison happens after the lock is released: it needs no
	// protection, and holding the lock across a clock read would lengthen the
	// critical section for nothing.
	if !ok || !c.now().Before(e.expiresAt) {
		var zero V
		return zero, ErrNotFound
	}
	return e.value, nil
}

// Delete removes key from the cache; deleting an absent key is a no-op.
func (c *Cache[K, V]) Delete(key K) {
	s := c.shardFor(key)
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
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

// removeExpired deletes every entry whose deadline has passed.
//
// Shards are swept one at a time, each under its own write lock, so a sweep of a
// large cache never holds a single lock across the whole keyspace — the pause any
// one reader can see is bounded by one shard rather than by the entire map. The
// sweep is consequently not an atomic snapshot across shards, which costs nothing
// here: expiry is judged by Get against the deadline, so a shard swept a moment
// later than its neighbour is only memory reclaimed a moment later (ADR-0021).
func (c *Cache[K, V]) removeExpired() {
	now := c.now()
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.Lock()
		for k, e := range s.entries {
			if !now.Before(e.expiresAt) {
				delete(s.entries, k)
			}
		}
		s.mu.Unlock()
	}
}
