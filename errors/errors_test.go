package errors_test

import (
	stderrors "errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var errSentinel = stderrors.New("root cause")

func TestWrapMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrap(errSentinel, "loading config")
	require.Equal(t, "loading config: root cause", err.Error())
}

func TestWrapfMessage(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrapf(errSentinel, "loading %s #%d", "config", 7)
	require.Equal(t, "loading config #7: root cause", err.Error())
}

func TestWrapNilReturnsNil(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.NoError(t, errors.Wrap(nil, "x"))
	require.NoError(t, errors.Wrapf(nil, "x %d", 1))
}

func TestIsTransparent(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrap(errors.Wrap(errSentinel, "inner"), "outer")
	require.ErrorIs(t, err, errSentinel, "errors.Is sees through the wrapping")
}

type customError struct{ code int }

func (e *customError) Error() string { return fmt.Sprintf("custom %d", e.code) }

func TestAsTransparent(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrap(&customError{code: 42}, "context")
	var ce *customError
	require.ErrorAs(t, err, &ce)
	require.Equal(t, 42, ce.code)
}

func TestUnwrap(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrap(errSentinel, "ctx")
	require.Equal(t, errSentinel, stderrors.Unwrap(err))
}

func TestStackCapturedAtCallSite(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrap(errSentinel, "ctx") // capture happens on this line

	var st errors.StackTracer
	require.ErrorAs(t, err, &st)
	pcs := st.StackTrace()
	require.NotEmpty(t, pcs)

	frame, _ := runtime.CallersFrames(pcs).Next()
	require.Contains(t, frame.Function, "TestStackCapturedAtCallSite",
		"the top frame is the caller of Wrap, not an internal helper")
}

func TestStackCapturedOnce(t *testing.T) {
	defer goleak.VerifyNone(t)
	inner := errors.Wrap(errSentinel, "inner") // the only capture point
	outer := errors.Wrap(inner, "outer")       // must NOT re-capture

	var innerST, outerST errors.StackTracer
	require.ErrorAs(t, inner, &innerST)
	require.ErrorAs(t, outer, &outerST)

	require.Equal(t, innerST.StackTrace(), outerST.StackTrace(),
		"the outer wrap reuses the inner stack rather than capturing a new one")
}

func TestFormatVerbs(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := errors.Wrap(errSentinel, "ctx")

	require.Equal(t, "ctx: root cause", fmt.Sprintf("%v", err))
	require.Equal(t, "ctx: root cause", fmt.Sprintf("%s", err))
	require.Equal(t, `"ctx: root cause"`, fmt.Sprintf("%q", err))

	plus := fmt.Sprintf("%+v", err)
	require.True(t, strings.HasPrefix(plus, "ctx: root cause"))
	require.Contains(t, plus, "TestFormatVerbs", "%+v includes the captured stack")
	require.Contains(t, plus, "errors_test.go")
}
