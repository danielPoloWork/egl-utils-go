package pgxhealth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/danielPoloWork/egl-utils-go/health"
)

// fakePinger stands in for a *pgxpool.Pool. A real pool cannot be usefully faked
// — it is a struct with unexported state and no interface — which is why the
// probe's logic is written against the narrow pinger interface.
type fakePinger struct {
	err    error
	block  time.Duration // if set, Ping waits for this long or until ctx ends
	gotCtx context.Context
	calls  int
}

func (f *fakePinger) Ping(ctx context.Context) error {
	f.calls++
	f.gotCtx = ctx
	if f.block > 0 {
		select {
		case <-time.After(f.block):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return f.err
}

func TestProbeHealthy(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	c := newCheck("postgres", f)

	require.Equal(t, "postgres", c.Name)
	require.NoError(t, c.Probe(context.Background()))
	require.Equal(t, 1, f.calls)
}

func TestProbeWrapsDriverError(t *testing.T) {
	defer goleak.VerifyNone(t)
	sentinel := errors.New("dial tcp: connection refused")
	c := newCheck("postgres", &fakePinger{err: sentinel})

	err := c.Probe(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel, "pgx's error stays reachable with errors.Is")
	require.Contains(t, err.Error(), "pgxhealth", "the error names this package")
	require.Contains(t, err.Error(), `"postgres"`, "and the check it came from")
}

func TestProbeHonoursTheRequestContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	c := newCheck("postgres", &fakePinger{block: time.Minute})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, c.Probe(ctx), context.DeadlineExceeded)
}

func TestProbeAppliesItsOwnTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The case WithTimeout exists for: an exhausted pool makes Ping queue rather
	// than fail, so the probe must bound itself even under a generous caller
	// deadline.
	f := &fakePinger{block: time.Minute}
	c := newCheck("postgres", f, WithTimeout(20*time.Millisecond))

	start := time.Now()
	err := c.Probe(context.Background())
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 5*time.Second, "the probe's own timeout fired")

	deadline, ok := f.gotCtx.Deadline()
	require.True(t, ok, "the timeout is applied by giving pgx a deadline context")
	require.WithinDuration(t, start.Add(20*time.Millisecond), deadline, time.Second)
}

func TestProbeAddsNoDeadlineWithoutTheOption(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	c := newCheck("postgres", f)
	require.NoError(t, c.Probe(context.Background()))

	_, ok := f.gotCtx.Deadline()
	require.False(t, ok, "without WithTimeout the caller's context is passed through unchanged")
}

func TestProbeIsRepeatable(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	c := newCheck("postgres", f)
	for i := 1; i <= 3; i++ {
		require.NoError(t, c.Probe(context.Background()))
		require.Equal(t, i, f.calls)
	}

	f.err = errors.New("server closed the connection")
	require.Error(t, c.Probe(context.Background()))

	f.err = nil
	require.NoError(t, c.Probe(context.Background()), "recovery is observed on the next call")
}

// TestComposesWithHealthHandler is the point of this module: the check must drop
// into the core's handler and produce the right status code and body.
func TestComposesWithHealthHandler(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	h := health.Handler(newCheck("postgres", f))

	serve := func() (int, string, map[string]any) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return rec.Code, rec.Body.String(), body
	}

	code, _, body := serve()
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "ok", body["status"])
	require.Equal(t, map[string]any{"postgres": "ok"}, body["checks"])

	f.err = errors.New("dial tcp: connection refused")
	code, raw, body := serve()
	require.Equal(t, http.StatusServiceUnavailable, code)
	require.Equal(t, "unavailable", body["status"])
	require.Equal(t, map[string]any{"postgres": "fail"}, body["checks"])
	require.NotContains(t, raw, "connection refused",
		"pgx's error must never reach the response body (ADR-0026)")
}

// The loud-by-default wiring checks (ADR-0005).

func TestCheckPanicsOnNilPool(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "pgxhealth: nil pool", func() {
		Check("postgres", nil)
	})
}

func TestCheckPanicsOnEmptyName(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "pgxhealth: empty check name", func() {
		newCheck("", &fakePinger{})
	})
}

func TestWithTimeoutPanicsOnNonPositive(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, d := range []time.Duration{0, -time.Second} {
		require.PanicsWithValue(t, "pgxhealth: non-positive timeout", func() {
			WithTimeout(d)
		})
	}
}

// TestCheckAcceptsARealPool wires the exported constructor to an actual
// *pgxpool.Pool, which is the signature a consumer uses. pgxpool.NewWithConfig
// does not dial eagerly, so no PostgreSQL server is needed to assert that the
// exported path compiles and accepts the driver's concrete type.
//
// pool.Close is deferred *after* the goleak defer so it runs first (defers are
// LIFO): NewWithConfig starts a backgroundHealthCheck goroutine, and registering
// the close with t.Cleanup instead would run it after goleak had already looked —
// which is exactly how the first version of this test failed.
func TestCheckAcceptsARealPool(t *testing.T) {
	defer goleak.VerifyNone(t)
	cfg, err := pgxpool.ParseConfig("postgres://user:pass@127.0.0.1:1/db")
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	defer pool.Close()

	c := Check("postgres", pool, WithTimeout(time.Second))
	require.Equal(t, "postgres", c.Name)
	require.NotNil(t, c.Probe)
}
