//go:build !race

// Excluded from `-race` builds (BUG-0001). Every test in this file counts
// allocations with testing.AllocsPerRun, and the race detector adds its own:
// the same chain measures 2 allocs where the budget is 1, and 13 where it is 11.
// Those numbers describe an instrumented binary, not the one consumers run, so
// asserting on them under -race gates nothing and fails always.
//
// The budgets still gate. This file runs in the ordinary `go test ./...` on every
// CI cell (Linux/Windows/macOS x Go 1.25/1.26) — only the two -race jobs skip it.

package middleware_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/middleware"
	"github.com/stretchr/testify/require"
)

// NFR-01's allocation half, enforced.
//
// The spec asks for **0 allocs/op** on the non-logging chain. That target is
// unreachable, and not by a little: a middleware that propagates a value through
// the request context and writes a response header must allocate. Measured on the
// reference workstation (ADR-0037 records the full attribution):
//
//	context.WithValue                              1 alloc  — how the ID propagates
//	r.WithContext (shallow *http.Request copy)     1 alloc  — must not mutate the caller's Request
//	Header().Set — the []string{value} it stores    1 alloc  — per header written
//	Header().Get/Set with a non-canonical key       1 alloc each — see below
//
// So the achievable floor for RequestID + Recoverer + Cors is around five or six
// allocations, never zero. Rather than carry an unachievable absolute — which
// enforces nothing, because it can only ever fail — this test pins the **measured
// floor as a budget**: a ratchet that fails when the count grows. That converts a
// dead target into a live regression gate.
//
// Allocation counts are a property of the code, not of the machine, so unlike the
// latency and throughput NFRs these are hard assertions rather than reported
// measurements. They run in the ordinary test suite on every CI cell.
//
// The two allocations 10.10 found and left for the maintainer are gone as of
// v1.1.1, and the way they went is worth keeping. The cost came from passing
// middleware.HeaderName ("X-Request-ID") to Header.Get and Header.Set:
// textproto.CanonicalMIMEHeaderKey allocates whenever its argument is not
// already canonical, and Go's canonical form is "X-Request-Id". 10.10 recorded
// this as blocked behind an API-visible change to the exported constant — but
// the constant's *value* and the *cost of using it as a map key* are separable.
// RequestID now uses an unexported canonical spelling for map access while
// HeaderName keeps its documented value, so nothing observable changed (Set
// stores under the canonical key regardless) and no MAJOR bump was needed.
// The budgets below no longer carry those two allocations.

// allocBudget is the per-request allocation ceiling for each middleware, measured
// on the adopt path (a valid inbound X-Request-ID) unless noted. Lowering an entry
// is always welcome; raising one requires a reason in the PR.
var allocBudget = map[string]int{
	"RequestID":       4, // v1.1.1: 6 before the canonical-key fix
	"Recoverer":       1,
	"Cors":            1,
	"Chain":           6, // RequestID + Recoverer + Cors; 8 before
	"Logger":          1,
	"ChainWithLogger": 9, // 11 before
}

func measureAllocs(t *testing.T, h http.Handler, r *http.Request) int {
	t.Helper()
	w := nullWriter{h: make(http.Header)}
	// A warm-up pass populates the response header map, so map growth is not
	// charged to the steady-state count the budget is about.
	h.ServeHTTP(w, r)
	return int(testing.AllocsPerRun(200, func() { h.ServeHTTP(w, r) }))
}

func TestNFR01AllocationBudget(t *testing.T) {
	discard := slog.New(slog.NewJSONHandler(io.Discard, nil))

	cases := []struct {
		name    string
		handler http.Handler
		request func() *http.Request
	}{
		{"RequestID", middleware.RequestID(noopHandler()), benchRequest},
		{"Recoverer", middleware.Recoverer(noopHandler()), benchRequest},
		{"Cors", middleware.Cors(corsForBench())(noopHandler()), benchRequest},
		{
			"Chain",
			middleware.RequestID(middleware.Recoverer(
				middleware.Cors(corsForBench())(noopHandler()),
			)),
			benchRequest,
		},
		{"Logger", middleware.Logger(discard)(noopHandler()), benchRequest},
		{
			"ChainWithLogger",
			middleware.RequestID(middleware.Logger(discard)(middleware.Recoverer(
				middleware.Cors(corsForBench())(noopHandler()),
			))),
			benchRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			budget, ok := allocBudget[tc.name]
			require.Truef(t, ok, "no allocation budget declared for %s", tc.name)

			got := measureAllocs(t, tc.handler, tc.request())
			t.Logf("%s: %d allocs/op (budget %d)", tc.name, got, budget)
			require.LessOrEqualf(t, got, budget,
				"%s allocates %d per request, over its budget of %d. If the increase is "+
					"intentional, raise the entry in allocBudget and say why in the PR; "+
					"NFR-01's nominal target of 0 is unreachable (see the note above).",
				tc.name, got, budget)
		})
	}
}

// TestNFR01LoggerWithinSpecAllocBudget checks the one part of NFR-01 the
// implementation actually satisfies as written: Logger adds ≤ 3 allocs/op.
func TestNFR01LoggerWithinSpecAllocBudget(t *testing.T) {
	const specBudget = 3 // spec v2 §5, NFR-01
	h := middleware.Logger(slog.New(slog.NewJSONHandler(io.Discard, nil)))(noopHandler())
	got := measureAllocs(t, h, benchRequest())
	require.LessOrEqualf(t, got, specBudget,
		"Logger allocates %d per request, over NFR-01's stated ≤ %d", got, specBudget)
}

// TestNFR01GeneratedIDCostsOneMoreAlloc pins the difference between adopting an
// inbound ID and minting one, so a change in the generation path is visible
// rather than hidden inside the chain's total.
func TestNFR01GeneratedIDCostsOneMoreAlloc(t *testing.T) {
	h := middleware.RequestID(noopHandler())

	adopted := measureAllocs(t, h, benchRequest())
	generated := measureAllocs(t, h, httptest.NewRequest(http.MethodGet, "/orders/42", nil))

	t.Logf("RequestID: %d allocs adopting an inbound ID, %d minting one", adopted, generated)
	require.Equal(t, adopted+1, generated,
		"minting an ID should cost exactly one allocation more than adopting one "+
			"(crypto/rand.Text's returned string)")
}
