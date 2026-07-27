package syncpool

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestPutDiscardsOversizedBuffer(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := NewBufferPool()

	big := p.Get()
	big.Grow(maxRetainedCap + 1)
	require.Greater(t, big.Cap(), maxRetainedCap)
	p.Put(big) // over the cap → dropped, not pooled

	got := p.Get()
	require.NotSame(t, big, got, "an oversized buffer must not be retained (it would pin memory)")
}

// TestPutRetainsBufferAtCap lives in syncpool_internal_norace_test.go: it asserts
// pool *identity*, which the race detector does not preserve (BUG-0001).
