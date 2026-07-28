package errx_test

import (
	stderrors "errors"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/errx"
)

// House rule (workerpool precedent): ReportAllocs with b.N / RunParallel.
//
// Package-level sinks: a discarded result lets the compiler eliminate the call
// and report a false zero — a trap this repo has published a wrong number into
// before (ADR-0037).
var (
	sinkErr    error
	sinkFrames []errx.Frame
)

var errBench = stderrors.New("bench failure")

// Wrap is message-only since 13.2, so it never reaches runtime.Callers.
func BenchmarkWrap(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkErr = errx.Wrap(errBench, "outer")
	}
}

func BenchmarkWrapf(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkErr = errx.Wrapf(errBench, "tenant %s", "acme")
	}
}

// Wrapping a chain that already carries a stack costs the same as wrapping one
// that does not: Wrap never inspects the chain.
func BenchmarkWrapOverExistingStack(b *testing.B) {
	base := errx.WithStack(errBench)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkErr = errx.Wrap(base, "outer")
	}
}

// The capture the caller now opts into explicitly.
func BenchmarkWithStack(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkErr = errx.WithStack(errBench)
	}
}

// Returning an error that already carries a stack is the no-op path.
func BenchmarkWithStackIdempotent(b *testing.B) {
	base := errx.WithStack(errBench)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkErr = errx.WithStack(base)
	}
}

// Symbolization: the expensive half, paid only on the first read. This is the
// measurement that justifies resolving lazily rather than at capture.
func BenchmarkFramesFirstRead(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		err := errx.WithStack(errBench)
		b.StartTimer()
		sinkFrames = errx.Frames(err)
	}
}

// Later reads hit the cache, so a trace that is both logged and inspected
// symbolizes once.
func BenchmarkFramesCachedRead(b *testing.B) {
	err := errx.WithStack(errBench)
	sinkFrames = errx.Frames(err) // warm the cache
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkFrames = errx.Frames(err)
	}
}
