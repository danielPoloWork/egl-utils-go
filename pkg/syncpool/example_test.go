package syncpool_test

import (
	"fmt"
	"strings"
	"sync"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/syncpool"
)

// The borrow/return cycle: Get, write, read the result out, Put. The pool is
// created once at wiring time and shared — a pool created per call pools
// nothing.
func ExampleBufferPool() {
	pool := syncpool.NewBufferPool()

	buf := pool.Get() // empty and safe to write to immediately
	defer pool.Put(buf)

	buf.WriteString("order-")
	fmt.Fprintf(buf, "%d", 42)

	// Read the buffer before Put, never after: once returned, another goroutine
	// may already be writing to it. String() copies, so the value it yields is
	// safe to keep — Bytes() does not, and a slice kept past Put is a data race.
	// Here the read happens before the deferred Put runs.
	fmt.Println(buf.String())
	// Output: order-42
}

// A pooled buffer comes back empty: Put resets it, so nothing a previous
// borrower wrote can leak into the next one's output. That is the property that
// makes a shared pool safe to build responses in.
func ExampleBufferPool_Get() {
	pool := syncpool.NewBufferPool()

	first := pool.Get()
	first.WriteString("secret-tenant-A")
	pool.Put(first)

	// Under concurrency this may or may not be the same buffer — sync.Pool
	// promises nothing about which one comes back, and correctness must not
	// depend on it. What is promised is the state: length zero, either way.
	second := pool.Get()
	defer pool.Put(second)
	fmt.Println(second.Len())
	// Output: 0
}

// The pool exists for hot paths, so the realistic call site is concurrent: many
// goroutines borrowing and returning, which is what lets the pool amortize the
// allocation instead of the caller paying it every time.
func ExampleBufferPool_concurrent() {
	pool := syncpool.NewBufferPool()

	var wg sync.WaitGroup
	results := make([]string, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := pool.Get()
			defer pool.Put(buf)

			buf.WriteString("worker-")
			fmt.Fprintf(buf, "%d", i)
			// String() copies, so the result outlives the borrow safely.
			results[i] = buf.String()
		}()
	}
	wg.Wait()

	// Printed as one joined, index-ordered line — goroutine completion order is
	// not something the pool, or anything else here, promises.
	fmt.Println(strings.Join(results, " "))
	// Output: worker-0 worker-1 worker-2 worker-3
}
