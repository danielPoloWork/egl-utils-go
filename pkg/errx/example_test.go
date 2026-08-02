package errx_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/errx"
)

// errNotFound stands in for a caller's own sentinel. Both the sentinel's text
// and the wrap messages below belong to this example, so printing the composed
// message pins nothing about the errx package's own wording.
var errNotFound = errors.New("not found")

// Wrap adds context and nothing else — it never touches the runtime, so it
// costs a single allocation and no stack walk. The chain stays transparent to
// errors.Is and errors.As.
func ExampleWrap() {
	err := errx.Wrap(errNotFound, "loading user")
	err = errx.Wrap(err, "handling GET /profile")

	fmt.Println(err)
	// Each layer added a "what I was doing", and the sentinel is still reachable
	// underneath — which is the whole point of wrapping rather than replacing.
	fmt.Println(errors.Is(err, errNotFound))

	// Wrapping nil returns nil, so the idiomatic guard needs no companion check:
	// `return errx.Wrap(f(), "…")` is correct on the success path too.
	fmt.Println(errx.Wrap(nil, "loading user") == nil)
	// Output:
	// handling GET /profile: loading user: not found
	// true
	// true
}

// Wrapf is Wrap with a formatted message, for the identifiers that make a log
// line actionable.
func ExampleWrapf() {
	err := errx.Wrapf(errNotFound, "loading user %d for tenant %q", 42, "acme")

	fmt.Println(err)
	fmt.Println(errors.Is(err, errNotFound))
	// Output:
	// loading user 42 for tenant "acme": not found
	// true
}

// WithStack captures a call stack at the point of failure. It is separate from
// Wrap because the capture is the expensive part, and this is where the caller
// says it is worth paying for.
func ExampleWithStack() {
	err := loadConfig()

	// Wrapping after the capture cannot move the trace: the stack lives at one
	// node in the chain, and Wrap creates a different node. So a trace always
	// points at the failure site however far the error travels afterwards.
	err = errx.Wrap(err, "starting the service")

	frames := errx.Frames(err)
	fmt.Println(len(frames) > 0)
	// The innermost frame is where WithStack ran, not where it was read.
	fmt.Println(strings.HasSuffix(frames[0].Function, "loadConfig"))

	// WithStack is idempotent: applying it again to a chain that already carries
	// a stack returns the error unchanged, so the earliest capture survives a
	// second, later attempt to record one.
	fmt.Println(len(errx.Frames(errx.WithStack(err))) == len(frames))
	// Output:
	// true
	// true
	// true
}

// loadConfig is the failing call whose stack the example captures. Capturing at
// the origin — rather than at the top of the handler — is what makes the trace
// worth having.
func loadConfig() error {
	return errx.WithStack(errNotFound)
}

// Frames reads a trace back without importing runtime, and returns nil when no
// stack was ever captured — which is the ordinary case, since Wrap captures
// none. A trace reader must handle that rather than assume a stack is there.
func ExampleFrames() {
	stackless := errx.Wrap(errNotFound, "loading user")
	fmt.Println(errx.Frames(stackless) == nil)
	fmt.Println(errx.Frames(nil) == nil)

	// %+v prints the message and then the trace, one frame per two lines; %v and
	// %s print the message alone. Symbolizing the counters is deferred to the
	// first read, so an error that is never printed never pays for it.
	traced := errx.Wrap(errx.WithStack(errNotFound), "loading user")
	fmt.Println(strings.Count(fmt.Sprintf("%+v", traced), "\n") > 0)
	fmt.Println(strings.Count(fmt.Sprintf("%v", traced), "\n"))
	// Output:
	// true
	// true
	// true
	// 0
}
