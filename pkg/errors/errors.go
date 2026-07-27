// Package errors adds a context message and a one-time captured call stack to
// an error, while staying fully interoperable with the standard library's
// errors package (errors.Is, errors.As, errors.Unwrap).
//
// Wrap and Wrapf attach a message to an error. The first time an error that
// carries no stack is wrapped, they capture the call stack at that point;
// wrapping an error that already carries one adds the message without
// re-capturing, so the recorded trace always points at the original failure
// site rather than at some later wrap. Wrapping a nil error returns nil.
//
// This package is named "errors" and shadows the standard library package of
// the same name; a file that needs both imports one under an alias (this
// package does exactly that internally).
package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"runtime"
)

// maxStackDepth bounds a captured stack; deeper stacks are truncated.
const maxStackDepth = 32

// StackTracer is implemented by an error that carries a captured call stack.
// Reach it with errors.As and turn the program counters into frames with
// runtime.CallersFrames:
//
//	var st errors.StackTracer
//	if errors.As(err, &st) {
//		frames := runtime.CallersFrames(st.StackTrace())
//		// ...
//	}
type StackTracer interface {
	StackTrace() []uintptr
}

// wrapped is an error with a context message, a wrapped cause, and the origin
// call stack — captured at the first wrap and inherited (by reference) by every
// later wrap in the chain, so it always points at the original failure site.
type wrapped struct {
	msg   string
	err   error
	stack []uintptr
}

// Error renders "message: cause".
func (w *wrapped) Error() string { return w.msg + ": " + w.err.Error() }

// Unwrap returns the wrapped cause, making Wrap transparent to errors.Is/As.
func (w *wrapped) Unwrap() error { return w.err }

// StackTrace returns the origin call stack of the chain.
func (w *wrapped) StackTrace() []uintptr { return w.stack }

// Format supports fmt verbs: %v and %s print "message: cause"; %+v additionally
// prints the captured stack; %q prints the quoted message.
func (w *wrapped) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, w.Error())
			frames := runtime.CallersFrames(w.StackTrace())
			for {
				f, more := frames.Next()
				if f.Function != "" {
					_, _ = fmt.Fprintf(s, "\n%s\n\t%s:%d", f.Function, f.File, f.Line)
				}
				if !more {
					break
				}
			}
			return
		}
		_, _ = io.WriteString(s, w.Error())
	case 's':
		_, _ = io.WriteString(s, w.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", w.Error())
	}
}

// Wrap returns an error that annotates err with msg. It returns nil if err is
// nil. The chain stays errors.Is/As-transparent to err, and a stack is captured
// the first time a stackless chain is wrapped.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &wrapped{msg: msg, err: err, stack: originStack(err)}
}

// Wrapf is Wrap with a printf-formatted message. It returns nil if err is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &wrapped{msg: fmt.Sprintf(format, args...), err: err, stack: originStack(err)}
}

// originStack reuses the stack already captured in err's chain, or — the first
// time a stackless chain is wrapped — captures a fresh one. runtime.Callers
// therefore runs exactly once per chain; later wraps only copy the reference.
func originStack(err error) []uintptr {
	var st StackTracer
	if stderrors.As(err, &st) {
		return st.StackTrace()
	}
	pcs := make([]uintptr, maxStackDepth)
	// skip runtime.Callers, originStack, and Wrap/Wrapf → start at the caller.
	n := runtime.Callers(3, pcs)
	return pcs[:n]
}
