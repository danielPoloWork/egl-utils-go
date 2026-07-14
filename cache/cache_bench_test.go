package cache_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/cache"
)

// House rule (workerpool precedent): benchmarks use ReportAllocs with b.N /
// RunParallel, not b.Loop.

func BenchmarkGetHit(b *testing.B) {
	c := cache.NewInMemory[string, int](time.Hour)
	defer c.Close()
	c.Set("k", 42)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get("k")
	}
}

func BenchmarkGetMiss(b *testing.B) {
	c := cache.NewInMemory[string, int](time.Hour)
	defer c.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get("absent")
	}
}

func BenchmarkSet(b *testing.B) {
	c := cache.NewInMemory[string, int](time.Hour)
	defer c.Close()
	keys := make([]string, 1024)
	for i := range keys {
		keys[i] = strconv.Itoa(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(keys[i%1024], i)
	}
}

func BenchmarkGetParallel(b *testing.B) {
	c := cache.NewInMemory[string, int](time.Hour)
	defer c.Close()
	c.Set("k", 42)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Get("k")
		}
	})
}
