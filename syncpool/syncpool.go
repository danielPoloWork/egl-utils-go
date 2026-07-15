// Package syncpool provides a pool of reusable *bytes.Buffer values to relieve
// GC pressure on temporary-buffer hot paths (serialization, string building).
//
// A pooled buffer is borrowed with Get, used, and returned with Put; in steady
// state Get/Put cycle without allocating. Put resets each buffer before pooling
// it, and — crucially — discards any buffer that has grown past a cap instead
// of retaining it, so an occasional very large buffer cannot pin that memory in
// the pool forever (the classic sync.Pool footgun).
package syncpool

import (
	"bytes"
	"sync"
)

// maxRetainedCap bounds the capacity of a buffer the pool keeps. A buffer grown
// beyond this (e.g. by one outsized payload) is dropped by Put and left to the
// GC rather than parked in the pool, capping steady-state memory. 64 KiB is
// generous for the string/serialization work this pool targets while still
// bounding a pathological retention.
const maxRetainedCap = 64 << 10

// BufferPool is a pool of reusable *bytes.Buffer. Create it with NewBufferPool;
// its zero value is not usable. It is safe for concurrent use.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool returns a ready BufferPool. Buffers it hands out are empty
// (length zero) and safe to write to immediately.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{New: func() any { return new(bytes.Buffer) }},
	}
}

// Get returns an empty buffer from the pool, allocating a fresh one only when
// the pool is empty. The caller should return it with Put when done.
func (p *BufferPool) Get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

// Put resets buf and returns it to the pool for reuse. A buffer whose capacity
// has grown past the retention cap is dropped (left to the GC) instead of
// pooled, so a one-off large buffer does not pin memory. A nil buf is ignored.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	if buf.Cap() > maxRetainedCap {
		return
	}
	buf.Reset()
	p.pool.Put(buf)
}
