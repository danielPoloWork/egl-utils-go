package syncpool_test

import (
	"testing"

	"github.com/danielPoloWork/egl-utils-go/syncpool"
)

// House rule (workerpool precedent): ReportAllocs with b.N / RunParallel.

func BenchmarkGetPut(b *testing.B) {
	p := syncpool.NewBufferPool()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := p.Get()
		buf.WriteString("benchmark payload")
		p.Put(buf)
	}
}

func BenchmarkGetPutParallel(b *testing.B) {
	p := syncpool.NewBufferPool()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := p.Get()
			buf.WriteString("benchmark payload")
			p.Put(buf)
		}
	})
}
