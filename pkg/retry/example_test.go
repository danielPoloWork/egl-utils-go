package retry_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/retry"
)

// Backoff re-runs a call until it succeeds or its attempt budget is spent.
func ExampleBackoff() {
	// A zero BaseDelay means retries are immediate, which is what keeps this
	// example instant and clock-free. A real policy spreads load instead:
	//
	//	retry.Policy{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond,
	//		MaxDelay: 2 * time.Second, Jitter: 0.2}
	//
	// there the delay doubles per retry, stops at MaxDelay, and jitter keeps
	// simultaneous failers from retrying in lockstep.
	policy := retry.Policy{MaxAttempts: 3}

	attempts := 0
	err := retry.Backoff(context.Background(), policy, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("upstream not ready")
		}
		return nil
	})

	fmt.Println(attempts, err == nil)
	// Output: 3 true
}

// The budget counts the first call, and a spent budget returns the last error
// the call produced.
func ExampleBackoff_exhausted() {
	// MaxAttempts 2 means one call and one retry.
	policy := retry.Policy{MaxAttempts: 2, BaseDelay: 0, MaxDelay: time.Second}

	errBadRequest := errors.New("bad request")

	calls := 0
	err := retry.Backoff(context.Background(), policy, func(context.Context) error {
		calls++
		return errBadRequest
	})

	// Backoff retries anything non-nil, so a permanently-failing call still
	// spends the whole budget: classify errors in fn and return nil-or-fail
	// deliberately if some failures should not be retried.
	fmt.Println(calls, errors.Is(err, errBadRequest))
	// Output: 2 true
}
