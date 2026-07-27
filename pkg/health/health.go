// Package health provides a preconfigured HTTP health-check handler that runs
// a set of dependency probes and reports readiness.
//
// Handler runs every probe concurrently on each request, passing the request's
// context, and responds 200 when all pass or 503 when any fails. The JSON body
// reports each check by name with an "ok"/"fail" status and an overall status —
// deliberately without the probe's error text, so an unauthenticated probe
// endpoint cannot leak internal detail (connection strings, hostnames) to the
// outside; the failing check's name is enough to locate the problem, and a
// consumer that wants error detail logs it inside the probe.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// Check is one named dependency probe. Probe reports the dependency's health;
// a nil error means healthy. It receives the request's context and should honor
// its cancellation so the endpoint stays responsive under a deadline.
type Check struct {
	Name  string
	Probe func(ctx context.Context) error
}

// response is the JSON body. encoding/json emits map keys sorted, so the output
// is deterministic.
type response struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

const (
	statusOK          = "ok"
	statusFail        = "fail"
	statusUnavailable = "unavailable"
)

// Handler returns an http.Handler that, on each request, runs every check's
// Probe concurrently with the request context and responds:
//
//   - 200 with {"status":"ok", ...} when every probe returns nil (or there are
//     no checks — a bare liveness endpoint);
//   - 503 with {"status":"unavailable", ...} when any probe returns an error.
//
// Each check appears in the "checks" object as name → "ok"/"fail"; the probe's
// error is never written to the response. Handler panics if a check has an
// empty name, a nil Probe, or a duplicate name — wiring errors caught at setup
// (ADR-0005 idiom).
func Handler(checks ...Check) http.Handler {
	seen := make(map[string]struct{}, len(checks))
	for _, c := range checks {
		switch {
		case c.Name == "":
			panic("health: empty check name")
		case c.Probe == nil:
			panic("health: nil probe for check " + c.Name)
		default:
			if _, dup := seen[c.Name]; dup {
				panic("health: duplicate check name " + c.Name)
			}
			seen[c.Name] = struct{}{}
		}
	}
	// Copy so a caller mutating its slice afterwards cannot affect the handler.
	cs := append([]Check(nil), checks...)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		results := run(r.Context(), cs)

		healthy := true
		for _, status := range results {
			if status != statusOK {
				healthy = false
				break
			}
		}

		code, status := http.StatusOK, statusOK
		if !healthy {
			code, status = http.StatusServiceUnavailable, statusUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(response{Status: status, Checks: results})
	})
}

// run executes every probe concurrently and collects name → status. It waits
// for all probes, so it owns no goroutine past its return.
func run(ctx context.Context, checks []Check) map[string]string {
	results := make(map[string]string, len(checks))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, c := range checks {
		wg.Add(1)
		go func(c Check) {
			defer wg.Done()
			status := statusOK
			if err := c.Probe(ctx); err != nil {
				status = statusFail
			}
			mu.Lock()
			results[c.Name] = status
			mu.Unlock()
		}(c)
	}
	wg.Wait()
	return results
}
