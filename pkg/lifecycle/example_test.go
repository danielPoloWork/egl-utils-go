package lifecycle_test

import (
	"context"
	"fmt"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/lifecycle"
)

// Register queues a shutdown hook; Shutdown runs them all, once, in reverse.
//
// NOTE for anyone adding a second example here: the package API is a
// process-wide singleton by design, and this example shuts it down. Registering
// after a shutdown panics, so a second example that touched the same singleton
// would fail depending on the order the test binary ran them in. The package's
// own tests avoid this by swapping the coordinator behind an unexported seam,
// which an external example cannot reach — so keep this the only one.
func ExampleRegister() {
	// Hooks run in reverse registration order, like defer: register a resource's
	// shutdown right after opening it and the dependency order takes care of
	// itself — the database closes after the server that talks to it drains.
	lifecycle.Register(func(context.Context) error {
		fmt.Println("closing the database")
		return nil
	})
	lifecycle.Register(func(context.Context) error {
		fmt.Println("draining the HTTP server")
		return nil
	})

	// Shutdown runs every hook even if one fails, joining their errors, and it
	// converges: concurrent or repeated calls return the first run's result
	// instead of running anything twice.
	//
	// A service does not usually call this itself. It calls
	//
	//	lifecycle.WaitForSignals(20*time.Second, syscall.SIGINT, syscall.SIGTERM)
	//
	// which blocks in place, owning no goroutine, and runs Shutdown when a
	// signal arrives — with the timeout measured from the signal, and 0 meaning
	// no deadline at all. It cannot appear in an example because it blocks until
	// a signal that will not come; lifecycle.Trigger() is the programmatic way to
	// wake it from inside the process.
	if err := lifecycle.Shutdown(context.Background()); err != nil {
		fmt.Println("shutdown:", err)
	}
	// Output:
	// draining the HTTP server
	// closing the database
}
