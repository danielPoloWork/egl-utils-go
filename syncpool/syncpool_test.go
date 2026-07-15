package syncpool_test

import (
	"sync"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/syncpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestGetReturnsEmptyBuffer(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := syncpool.NewBufferPool()
	b := p.Get()
	require.NotNil(t, b)
	require.Zero(t, b.Len(), "a borrowed buffer starts empty")
}

func TestPutResetsAndReuses(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := syncpool.NewBufferPool()
	b := p.Get()
	b.WriteString("payload")
	p.Put(b)

	got := p.Get()
	require.Same(t, b, got, "a returned buffer is handed back out (single goroutine, no GC)")
	require.Zero(t, got.Len(), "and it comes back reset")
}

func TestPutNilIsIgnored(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := syncpool.NewBufferPool()
	require.NotPanics(t, func() { p.Put(nil) })
}

func TestGetPutIsZeroAllocInSteadyState(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := syncpool.NewBufferPool()
	allocs := testing.AllocsPerRun(1000, func() {
		b := p.Get()
		b.WriteString("hello, world") // fits in the retained capacity after warm-up
		p.Put(b)
	})
	require.Zero(t, allocs, "steady-state Get/write/Put must not allocate")
}

func TestConcurrentGetPut(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The -race CI job is the real assertion; this drives contention.
	p := syncpool.NewBufferPool()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				b := p.Get()
				b.WriteString("concurrent")
				require.Equal(t, "concurrent", b.String())
				p.Put(b)
			}
		}()
	}
	wg.Wait()
}
