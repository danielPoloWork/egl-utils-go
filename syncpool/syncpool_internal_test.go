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

func TestPutRetainsBufferAtCap(t *testing.T) {
	defer goleak.VerifyNone(t)
	p := NewBufferPool()

	small := p.Get()
	small.Grow(1 << 10) // well under the cap
	require.LessOrEqual(t, small.Cap(), maxRetainedCap)
	p.Put(small)

	require.Same(t, small, p.Get(), "a within-cap buffer is pooled for reuse")
}
