package errx_test

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/errx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBase = stderrors.New("base failure")

// captureHere is the origin site every trace assertion below expects to find.
// It exists as a named function so the expectation can be spelled as a name
// rather than a line number.
func captureHere() error { return errx.WithStack(errBase) }

func TestWrapNilReturnsNil(t *testing.T) {
	t.Parallel()

	assert.NoError(t, errx.Wrap(nil, "msg"))
	assert.NoError(t, errx.Wrapf(nil, "msg %d", 1))
	assert.NoError(t, errx.WithStack(nil))
}

func TestWrapRendersMessageAndCause(t *testing.T) {
	t.Parallel()

	assert.EqualError(t, errx.Wrap(errBase, "loading"), "loading: base failure")
	assert.EqualError(t, errx.Wrapf(errBase, "tenant %s", "acme"), "tenant acme: base failure")
}

func TestWithStackLeavesTheMessageAlone(t *testing.T) {
	t.Parallel()

	// WithStack adds a stack, not text.
	assert.EqualError(t, errx.WithStack(errBase), "base failure")
}

func TestChainStaysTransparentToStdErrors(t *testing.T) {
	t.Parallel()

	err := errx.Wrap(errx.WithStack(errBase), "outer")

	assert.ErrorIs(t, err, errBase, "errors.Is must see through Wrap and WithStack")
	assert.Equal(t, errBase, stderrors.Unwrap(stderrors.Unwrap(err)))

	var target *customError
	wrapped := errx.Wrap(errx.WithStack(&customError{code: 42}), "outer")
	require.True(t, stderrors.As(wrapped, &target), "errors.As must see through the chain")
	assert.Equal(t, 42, target.code)
}

// --- stacks -----------------------------------------------------------------

func TestWrapCapturesNoStack(t *testing.T) {
	t.Parallel()

	// The whole point of 13.2: Wrap is message-only and never touches runtime.
	assert.Nil(t, errx.Frames(errx.Wrap(errBase, "outer")))
	assert.Nil(t, errx.Frames(errx.Wrapf(errBase, "outer %d", 1)))
}

func TestFramesReturnsNilWithoutACapture(t *testing.T) {
	t.Parallel()

	assert.Nil(t, errx.Frames(nil))
	assert.Nil(t, errx.Frames(errBase))
}

func TestWithStackCapturesAtTheCallSite(t *testing.T) {
	t.Parallel()

	frames := errx.Frames(captureHere())

	require.NotEmpty(t, frames)
	assert.True(t, strings.HasSuffix(frames[0].Function, ".captureHere"),
		"the first frame must be the caller of WithStack, got %q", frames[0].Function)
	assert.True(t, strings.HasSuffix(frames[0].File, "errx_test.go"), "got %q", frames[0].File)
	assert.Positive(t, frames[0].Line)
}

func TestTraceKeepsPointingAtTheOriginThroughLaterWraps(t *testing.T) {
	t.Parallel()

	err := captureHere()
	for i := range 5 {
		err = errx.Wrapf(err, "layer %d", i)
	}

	frames := errx.Frames(err)
	require.NotEmpty(t, frames)
	assert.True(t, strings.HasSuffix(frames[0].Function, ".captureHere"),
		"wrapping must not move the trace off the origin, got %q", frames[0].Function)
}

func TestWithStackIsIdempotent(t *testing.T) {
	t.Parallel()

	origin := captureHere()
	again := errx.WithStack(origin)

	assert.Same(t, origin, again, "a chain that already carries a stack is returned unchanged")

	// Even reached through wraps, a second WithStack must not re-capture.
	wrapped := errx.Wrap(origin, "outer")
	assert.Same(t, wrapped, errx.WithStack(wrapped))
	frames := errx.Frames(errx.WithStack(wrapped))
	require.NotEmpty(t, frames)
	assert.True(t, strings.HasSuffix(frames[0].Function, ".captureHere"), "got %q", frames[0].Function)
}

func TestFramesResolveOnceAndAreStable(t *testing.T) {
	t.Parallel()

	err := captureHere()

	first := errx.Frames(err)
	second := errx.Frames(err)

	require.NotEmpty(t, first)
	assert.Equal(t, first, second, "repeated reads must agree")
	// Same backing array: resolution is cached, not redone.
	assert.Same(t, &first[0], &second[0], "the resolved trace must be cached, not rebuilt")
}

func TestDeepStackIsTruncated(t *testing.T) {
	t.Parallel()

	frames := errx.Frames(recurse(64))

	require.NotEmpty(t, frames)
	assert.LessOrEqual(t, len(frames), 32, "a captured stack is bounded by maxStackDepth")
}

func recurse(n int) error {
	if n == 0 {
		return errx.WithStack(errBase)
	}
	return recurse(n - 1)
}

// --- formatting -------------------------------------------------------------

func TestFormatVerbs(t *testing.T) {
	t.Parallel()

	err := errx.Wrap(errBase, "outer")

	assert.Equal(t, "outer: base failure", fmt.Sprintf("%v", err))
	assert.Equal(t, "outer: base failure", fmt.Sprintf("%s", err))
	assert.Equal(t, `"outer: base failure"`, fmt.Sprintf("%q", err))
}

func TestPlusVPrintsTheStackWhenThereIsOne(t *testing.T) {
	t.Parallel()

	out := fmt.Sprintf("%+v", errx.Wrap(captureHere(), "outer"))

	assert.True(t, strings.HasPrefix(out, "outer: base failure"), "got %q", out)
	assert.Contains(t, out, ".captureHere", "%%+v must print the captured trace")
	assert.Contains(t, out, "errx_test.go:")
}

func TestPlusVWithoutAStackPrintsOnlyTheMessage(t *testing.T) {
	t.Parallel()

	// Wrap alone captures nothing, so %+v has no trace to add.
	assert.Equal(t, "outer: base failure", fmt.Sprintf("%+v", errx.Wrap(errBase, "outer")))
}

func TestWithStackFormatting(t *testing.T) {
	t.Parallel()

	err := captureHere()

	assert.Equal(t, "base failure", fmt.Sprintf("%v", err))
	assert.Equal(t, "base failure", fmt.Sprintf("%s", err))
	assert.Equal(t, `"base failure"`, fmt.Sprintf("%q", err))
	assert.Contains(t, fmt.Sprintf("%+v", err), ".captureHere")
}

// --- extension point --------------------------------------------------------

func TestFramesFindsAForeignStackTracer(t *testing.T) {
	t.Parallel()

	want := []errx.Frame{{Function: "foreign.Fn", File: "foreign.go", Line: 7}}

	// StackTracer is the documented extension point: an error type from outside
	// this package can contribute a trace that Frames will find.
	assert.Equal(t, want, errx.Frames(errx.Wrap(&foreignTracer{frames: want}, "outer")))
}

type foreignTracer struct{ frames []errx.Frame }

func (f *foreignTracer) Error() string            { return "foreign" }
func (f *foreignTracer) StackTrace() []errx.Frame { return f.frames }

type customError struct{ code int }

func (c *customError) Error() string { return fmt.Sprintf("custom %d", c.code) }
