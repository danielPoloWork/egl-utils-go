package cache_test

import (
	"fmt"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/cache"
)

// A cache is generic over its key and value types, so nothing is stored as any
// and nothing is type-asserted on the way out.
//
// Note what these examples do not show: expiry. Every entry dies ttl after it
// was Set, and Get refuses an expired entry the instant its deadline passes —
// but demonstrating that would mean waiting for a real clock, and an example
// that sleeps to work is one that fails on a loaded CI runner. The TTL contract
// is documented on Get instead; here the TTL is set long enough that no entry
// can expire mid-example.
func ExampleNew() {
	type session struct {
		User  string
		Admin bool
	}

	// One goroutine per cache, whatever its size — Close stops it. A cache that
	// lives for the whole process still deserves the defer, because the next
	// person to move this code will not add it.
	c := cache.New[string, session](5 * time.Minute)
	defer c.Close()

	c.Set("tok-1", session{User: "ada", Admin: true})
	c.Set("tok-2", session{User: "grace"})

	s, ok := c.Get("tok-1")
	fmt.Println(ok, s.User, s.Admin)

	c.Delete("tok-1")
	_, ok = c.Get("tok-1")
	fmt.Println(ok)
	// Output:
	// true ada true
	// false
}

// Get is comma-ok, not error-returning: a miss is an ordinary outcome of a
// cache, not a failure, so the caller's job is to compute the value instead of
// to handle an error.
func ExampleCache_Get() {
	c := cache.New[string, int](time.Minute)
	defer c.Close()

	lookup := func(key string) int {
		if v, ok := c.Get(key); ok {
			return v
		}
		v := len(key) * 100 // stand-in for the expensive call
		c.Set(key, v)
		return v
	}

	fmt.Println(lookup("orders"))
	fmt.Println(lookup("orders")) // served from the cache this time

	// A miss and an expired entry are deliberately indistinguishable: both are
	// "not usable, compute it". Telling them apart would make the eviction
	// timing part of the API.
	_, ok := c.Get("never-stored")
	fmt.Println(ok)
	// Output:
	// 600
	// 600
	// false
}

// The sweeper is a memory reclaimer, never a correctness mechanism, which is
// why its interval is safe to tune: Get judges expiry against the entry's own
// deadline, so a slow sweep can hold memory but can never serve a stale value.
func ExampleWithCleanupInterval() {
	// Short-lived entries, swept ten times more often than they expire: memory
	// comes back sooner, at the cost of more frequent scans. Setting the
	// interval far above the TTL is the opposite trade and equally valid.
	c := cache.New[string, string](10*time.Minute, cache.WithCleanupInterval(time.Minute))
	defer c.Close()

	c.Set("region", "eu-west-1")
	v, ok := c.Get("region")
	fmt.Println(ok, v)
	// Output: true eu-west-1
}
