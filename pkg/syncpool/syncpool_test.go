package syncpool_test

import (
	"sync"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/syncpool"
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

// TestPutResetsAndReuses and TestGetPutIsZeroAllocInSteadyState live in
// syncpool_norace_test.go: they assert pool identity and allocation counts, neither
// of which the race detector preserves (BUG-0001).

func TestPutNilIsIgnored(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := syncpool.NewBufferPool()
	require.NotPanics(t, func() { p.Put(nil) })
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
