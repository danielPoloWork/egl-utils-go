// Package errx adds a context message and, on request, a captured call stack to
// an error, while staying fully interoperable with the standard library's errors
// package (errors.Is, errors.As, errors.Unwrap).
//
// Wrap and Wrapf attach a message and nothing else: they never touch the
// runtime. A call stack is captured only where WithStack is called, which is the
// one place the caller has said the stack is worth its cost.
//
// Because the stack lives at a single node in the chain rather than being copied
// into every wrap, a trace always points at the site WithStack ran — the
// original failure — no matter how many times the error is wrapped afterwards:
//
//	if err != nil {
//		return errx.Wrap(errx.WithStack(err), "loading config")
//	}
//
// Read a trace with Frames, which walks the chain for you and returns nil when
// no stack was ever captured. Frames resolve lazily: capturing records program
// counters, and the expensive symbolization happens once, only if something
// actually reads the trace.
//
// Unlike v1's errors package, this one does not shadow the standard library.
package errx

import (
	stderrors "errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

// maxStackDepth bounds a captured stack; deeper stacks are truncated.
const maxStackDepth = 32

// Frame is one resolved entry of a captured call stack. It carries no runtime
// types, so reading a trace never requires importing runtime.
type Frame struct {
	// Function is the fully qualified function name.
	Function string
	// File is the absolute path of the source file.
	File string
	// Line is the line number within File.
	Line int
}

// StackTracer is implemented by an error that carries a captured call stack.
// Frames searches a chain for it, so an error type defined outside this package
// can contribute a stack that Frames will find.
type StackTracer interface {
	StackTrace() []Frame
}

// Frames returns the call stack captured in err's chain, or nil if err is nil or
// no stack was ever captured. The first StackTracer found while unwrapping wins,
// which is the outermost — and therefore earliest-captured — one.
func Frames(err error) []Frame {
	if err == nil {
		return nil
	}
	var st StackTracer
	if stderrors.As(err, &st) {
		return st.StackTrace()
	}
	return nil
}

// hasStack reports whether err's chain already carries a stack. It deliberately
// does not go through Frames: answering this question must not trigger the
// symbolization Frames performs, or WithStack would resolve a whole trace just
// to decide it has nothing to do.
func hasStack(err error) bool {
	var st StackTracer
	return stderrors.As(err, &st)
}

// stack is a captured call stack whose symbolization is deferred until read.
// runtime.Callers is cheap; runtime.CallersFrames is not, and most errors are
// never formatted, so the second half is paid only on demand and only once.
type stack struct {
	pcs    []uintptr
	once   sync.Once
	frames []Frame
}

// newStack captures the stack of the caller of the exported function that calls
// it. Skipping 3 drops runtime.Callers, newStack, and that exported function.
func newStack() *stack {
	pcs := make([]uintptr, maxStackDepth)
	n := runtime.Callers(3, pcs)
	return &stack{pcs: pcs[:n]}
}

// resolve symbolizes the recorded counters once and caches the result, so a
// trace that is both printed and inspected does the work a single time.
func (s *stack) resolve() []Frame {
	s.once.Do(func() {
		// No length guard: CallersFrames over an empty slice reports more=false on
		// the first Next, so the loop below already handles it. A guard here would
		// be a branch no caller can reach.
		ci := runtime.CallersFrames(s.pcs)
		for {
			f, more := ci.Next()
			if f.Function != "" {
				s.frames = append(s.frames, Frame{Function: f.Function, File: f.File, Line: f.Line})
			}
			if !more {
				break
			}
		}
	})
	return s.frames
}

// writeStack appends a trace to s in the conventional one-frame-per-two-lines form.
func writeStack(s io.Writer, frames []Frame) {
	for _, f := range frames {
		_, _ = fmt.Fprintf(s, "\n%s\n\t%s:%d", f.Function, f.File, f.Line)
	}
}

// withStack is an error carrying a captured stack and no message of its own.
type withStack struct {
	err error
	st  *stack
}

// Error returns the underlying error's message unchanged: WithStack adds a
// stack, not text.
func (w *withStack) Error() string { return w.err.Error() }

// Unwrap returns the underlying error, keeping the chain Is/As-transparent.
func (w *withStack) Unwrap() error { return w.err }

// StackTrace returns the captured stack, resolving it on first use.
func (w *withStack) StackTrace() []Frame { return w.st.resolve() }

// Format supports fmt verbs: %v and %s print the message; %+v additionally
// prints the captured stack; %q prints the quoted message.
func (w *withStack) Format(s fmt.State, verb rune) { format(s, verb, w, w.Error()) }

// wrapped is an error with a context message and a wrapped cause. It carries no
// stack: any stack in the chain belongs to a withStack node further in, and
// Frames finds it by unwrapping.
type wrapped struct {
	msg string
	err error
}

// Error renders "message: cause".
func (w *wrapped) Error() string { return w.msg + ": " + w.err.Error() }

// Unwrap returns the wrapped cause, making Wrap transparent to errors.Is/As.
func (w *wrapped) Unwrap() error { return w.err }

// Format supports fmt verbs: %v and %s print "message: cause"; %+v additionally
// prints the chain's captured stack, if any; %q prints the quoted message.
func (w *wrapped) Format(s fmt.State, verb rune) { format(s, verb, w, w.Error()) }

// format implements the shared fmt.Formatter behavior of both error types.
func format(s fmt.State, verb rune, err error, msg string) {
	switch verb {
	case 'v':
		_, _ = io.WriteString(s, msg)
		if s.Flag('+') {
			writeStack(s, Frames(err))
		}
	case 's':
		_, _ = io.WriteString(s, msg)
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", msg)
	}
}

// WithStack returns err carrying a captured call stack. It returns nil if err is
// nil, and returns err unchanged if its chain already carries a stack, so the
// recorded trace keeps pointing at the earliest capture.
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	if hasStack(err) {
		return err
	}
	return &withStack{err: err, st: newStack()}
}

// Wrap returns an error that annotates err with msg, or nil if err is nil. It
// captures no stack; pair it with WithStack when a trace is wanted. The chain
// stays errors.Is/As-transparent to err.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &wrapped{msg: msg, err: err}
}

// Wrapf is Wrap with a printf-formatted message. It returns nil if err is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &wrapped{msg: fmt.Sprintf(format, args...), err: err}
}
