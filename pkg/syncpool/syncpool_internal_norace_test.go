//go:build !race

// Excluded from `-race` builds (BUG-0001). The assertion here is about sync.Pool
// *identity* — that the very buffer handed back is the one returned next — and the
// race detector does not preserve it: instrumentation changes P pinning and the
// per-P private slot, so Get may legitimately hand out a fresh buffer.
//
// The behaviour still gates. This file runs in the ordinary `go test ./...` on every
// CI cell (Linux/Windows/macOS x Go 1.25/1.26) — only the two -race jobs skip it.
// Its sibling TestPutDiscardsOversizedBuffer stays in the -race build on purpose:
// it asserts NotSame, which holds whether or not the pool returns the buffer.

package syncpool

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestPutRetainsBufferAtCap(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := NewBufferPool()

	small := p.Get()
	small.Grow(1 << 10) // well under the cap
	require.LessOrEqual(t, small.Cap(), maxRetainedCap)
	p.Put(small)

	require.Same(t, small, p.Get(), "a within-cap buffer is pooled for reuse")
}
