package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/health"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func ok(name string) health.Check {
	return health.Check{Name: name, Probe: func(context.Context) error { return nil }}
}

func fail(name string) health.Check {
	return health.Check{Name: name, Probe: func(context.Context) error { return errors.New("down") }}
}

// serve runs one GET through the handler and decodes the JSON body.
func serve(t *testing.T, h http.Handler) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec, body
}

func TestAllHealthy(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, body := serve(t, health.Handler(ok("db"), ok("redis")))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, "ok", body["status"])
	require.Equal(t, map[string]any{"db": "ok", "redis": "ok"}, body["checks"])
}

func TestOneUnhealthy(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, body := serve(t, health.Handler(ok("db"), fail("redis")))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "unavailable", body["status"])
	require.Equal(t, map[string]any{"db": "ok", "redis": "fail"}, body["checks"])
}

func TestNoChecksIsHealthy(t *testing.T) {
	defer goleak.VerifyNone(t)
	rec, body := serve(t, health.Handler())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", body["status"])
	require.Equal(t, map[string]any{}, body["checks"])
}

func TestErrorTextIsNotLeaked(t *testing.T) {
	defer goleak.VerifyNone(t)
	// G101: a fake DSN fixture, used to assert the probe error is never leaked.
	secret := "postgres://user:hunter2@db.internal:5432" //nolint:gosec
	h := health.Handler(health.Check{
		Name:  "db",
		Probe: func(context.Context) error { return errors.New(secret) },
	})
	rec, _ := serve(t, h)
	require.NotContains(t, rec.Body.String(), secret, "the probe's error must never reach the response")
	require.NotContains(t, rec.Body.String(), "hunter2")
}

func TestProbeReceivesRequestContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	type key struct{}
	var got any
	h := health.Handler(health.Check{
		Name: "ctx",
		Probe: func(ctx context.Context) error {
			got = ctx.Value(key{})
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil).
		WithContext(context.WithValue(context.Background(), key{}, "v"))
	h.ServeHTTP(httptest.NewRecorder(), req)
	require.Equal(t, "v", got, "each probe runs with the request's context")
}

func TestProbesRunConcurrently(t *testing.T) {
	defer goleak.VerifyNone(t)
	const n = 5
	var started sync.WaitGroup
	started.Add(n)
	release := make(chan struct{})

	checks := make([]health.Check, n)
	for i := range checks {
		checks[i] = health.Check{
			Name: string(rune('a' + i)),
			Probe: func(context.Context) error {
				started.Done() // signal this probe is running
				<-release      // block until every probe has started
				return nil
			},
		}
	}
	h := health.Handler(checks...)

	done := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		done <- rec.Code
	}()

	allStarted := make(chan struct{})
	go func() { started.Wait(); close(allStarted) }()
	select {
	case <-allStarted: // every probe was running at once — concurrent
	case <-time.After(2 * time.Second):
		t.Fatal("probes did not run concurrently")
	}
	close(release)
	require.Equal(t, http.StatusOK, <-done)
}

func TestPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := []struct {
		name string
		fn   func()
	}{
		{"empty name", func() { health.Handler(health.Check{Probe: func(context.Context) error { return nil }}) }},
		{"nil probe", func() { health.Handler(health.Check{Name: "db"}) }},
		{"duplicate name", func() { health.Handler(ok("db"), ok("db")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Panics(t, tc.fn)
		})
	}
}
