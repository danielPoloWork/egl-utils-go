// Package leakcheck provides the interim goroutine-leak assertion shared by
// the module's test suites: it snapshots the runtime goroutine count around a
// test and dumps every stack when the count fails to return to the baseline.
// ROADMAP 2.6 migrates the callers to goleak once the test-only dependencies
// land; this package then disappears. Being under internal/, it is invisible
// to consumers of the module.
package leakcheck

import (
	"runtime"
	"testing"
	"time"
)

// Guard registers a cleanup on t that fails the test if the process goroutine
// count has not returned to its pre-test baseline within a short grace
// period. Tests using Guard must not call t.Parallel — the goroutine count is
// process-global, so a parallel sibling would poison the baseline.
func Guard(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if runtime.NumGoroutine() <= before {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Errorf("goroutine leak: %d before, %d after\n%s",
			before, runtime.NumGoroutine(), buf[:n])
	})
}
