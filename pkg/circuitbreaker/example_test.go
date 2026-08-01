package circuitbreaker_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/circuitbreaker"
)

// Do runs a call while the breaker is closed and refuses it while open, so a
// failing dependency stops being called at all.
func ExampleBreaker_Do() {
	// The open timeout is deliberately long here: the half-open transition is
	// evaluated lazily on admission, so a minute of cool-down provably cannot
	// elapse while this example runs. Nothing below depends on the clock.
	breaker := circuitbreaker.New(
		circuitbreaker.WithFailureThreshold(2),
		circuitbreaker.WithOpenTimeout(time.Minute),
	)

	unavailable := func() error { return errors.New("dependency unavailable") }

	// Two consecutive failures reach the threshold and trip the breaker.
	for range 2 {
		if err := breaker.Do(context.Background(), unavailable); err != nil {
			fmt.Println("call failed")
		}
	}
	fmt.Println(breaker.State())

	// Now the call is refused without running at all — which is the point: the
	// caller fails fast and the dependency gets room to recover.
	calls := 0
	err := breaker.Do(context.Background(), func() error {
		calls++
		return nil
	})
	fmt.Println(errors.Is(err, circuitbreaker.ErrOpen), calls)
	// Output:
	// call failed
	// call failed
	// open
	// true 0
}

// State reports the breaker's position for metrics and health endpoints without
// admitting a call or changing anything.
func ExampleBreaker_State() {
	breaker := circuitbreaker.New(circuitbreaker.WithFailureThreshold(1))

	fmt.Println(breaker.State())

	if err := breaker.Do(context.Background(), func() error {
		return errors.New("dependency unavailable")
	}); err != nil {
		fmt.Println(breaker.State())
	}
	// Output:
	// closed
	// open
}
