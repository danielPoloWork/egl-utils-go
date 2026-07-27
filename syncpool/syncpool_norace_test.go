//go:build !race

// Excluded from `-race` builds (BUG-0001). Both assertions here are about runtime
// internals the race detector deliberately perturbs: sync.Pool *identity* (that the
// buffer handed back is the one returned next — instrumentation changes P pinning
// and the per-P private slot) and *allocation counts* (the detector allocates on
// its own account, so the number describes an instrumented binary rather than the
// one consumers run).
//
// Both still gate. This file runs in the ordinary `go test ./...` on every CI cell
// (Linux/Windows/macOS x Go 1.25/1.26) — only the two -race jobs skip it.

package syncpool_test

import (
	"testing"

	"github.com/danielPoloWork/egl-utils-go/syncpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

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
