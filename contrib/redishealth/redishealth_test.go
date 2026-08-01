package redishealth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/health"
)

// fakePinger stands in for a Redis client. go-redis has no in-memory server, so
// the probe's behaviour is driven through the narrow interface instead: a
// *redis.StatusCmd is constructible with NewStatusCmd, and SetErr puts it in the
// state a failed PING would.
type fakePinger struct {
	err    error
	block  time.Duration // if set, Ping waits for this long or until ctx ends
	gotCtx context.Context
	calls  int
}

func (f *fakePinger) Ping(ctx context.Context) *redis.StatusCmd {
	f.calls++
	f.gotCtx = ctx
	cmd := redis.NewStatusCmd(ctx, "ping")
	if f.block > 0 {
		select {
		case <-time.After(f.block):
		case <-ctx.Done():
			cmd.SetErr(ctx.Err())
			return cmd
		}
	}
	if f.err != nil {
		cmd.SetErr(f.err)
		return cmd
	}
	cmd.SetVal("PONG")
	return cmd
}

func TestProbeHealthy(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	c := newCheck("redis", f)

	require.Equal(t, "redis", c.Name)
	require.NoError(t, c.Probe(context.Background()))
	require.Equal(t, 1, f.calls)
}

func TestProbeWrapsDriverError(t *testing.T) {
	defer goleak.VerifyNone(t)
	sentinel := errors.New("connection refused")
	c := newCheck("redis", &fakePinger{err: sentinel})

	err := c.Probe(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel, "the driver's error stays reachable with errors.Is")
	require.Contains(t, err.Error(), "redishealth", "the error names this package")
	require.Contains(t, err.Error(), `"redis"`, "and the check it came from")
}

func TestProbeHonoursTheRequestContext(t *testing.T) {
	defer goleak.VerifyNone(t)
	// No per-probe timeout: the context health.Handler passes in must still bound
	// the probe, which is the contract health.Check documents.
	c := newCheck("redis", &fakePinger{block: time.Minute})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := c.Probe(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestProbeAppliesItsOwnTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	// A per-probe timeout must bound a slow dependency even when the caller's
	// context has a much longer deadline — the reason WithTimeout exists.
	f := &fakePinger{block: time.Minute}
	c := newCheck("redis", f, WithTimeout(20*time.Millisecond))

	start := time.Now()
	err := c.Probe(context.Background())
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, elapsed, 5*time.Second, "the probe's own timeout fired, not the driver's")

	deadline, ok := f.gotCtx.Deadline()
	require.True(t, ok, "the timeout is applied by giving the driver a deadline context")
	require.WithinDuration(t, start.Add(20*time.Millisecond), deadline, time.Second)
}

func TestProbeAddsNoDeadlineWithoutTheOption(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	c := newCheck("redis", f)
	require.NoError(t, c.Probe(context.Background()))

	_, ok := f.gotCtx.Deadline()
	require.False(t, ok, "without WithTimeout the caller's context is passed through unchanged")
}

func TestProbeIsRepeatable(t *testing.T) {
	defer goleak.VerifyNone(t)
	// health.Handler calls the probe once per request, so it must be reusable and
	// must not latch a previous result.
	f := &fakePinger{}
	c := newCheck("redis", f)
	for i := 1; i <= 3; i++ {
		require.NoError(t, c.Probe(context.Background()))
		require.Equal(t, i, f.calls)
	}

	f.err = errors.New("gone away")
	require.Error(t, c.Probe(context.Background()))

	f.err = nil
	require.NoError(t, c.Probe(context.Background()), "recovery is observed on the next call")
}

// TestComposesWithHealthHandler is the point of this module: the check must drop
// into the core's handler and produce the right status code and body. Without it,
// the module's whole reason for existing is untested.
func TestComposesWithHealthHandler(t *testing.T) {
	defer goleak.VerifyNone(t)
	f := &fakePinger{}
	h := health.Handler(newCheck("redis", f))

	serve := func() (int, map[string]any) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return rec.Code, body
	}

	code, body := serve()
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "ok", body["status"])
	require.Equal(t, map[string]any{"redis": "ok"}, body["checks"])

	f.err = errors.New("connection refused")
	code, body = serve()
	require.Equal(t, http.StatusServiceUnavailable, code)
	require.Equal(t, "unavailable", body["status"])
	require.Equal(t, map[string]any{"redis": "fail"}, body["checks"])
	require.NotContains(t, string(mustJSON(t, body)), "connection refused",
		"the driver's error must never reach the response body (ADR-0026)")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// The loud-by-default wiring checks (ADR-0005).

func TestCheckPanicsOnNilClient(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "redishealth: nil client", func() {
		Check("redis", nil)
	})
}

func TestCheckPanicsOnEmptyName(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "redishealth: empty check name", func() {
		newCheck("", &fakePinger{})
	})
}

func TestWithTimeoutPanicsOnNonPositive(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, d := range []time.Duration{0, -time.Second} {
		require.PanicsWithValue(t, "redishealth: non-positive timeout", func() {
			WithTimeout(d)
		})
	}
}

// TestCheckAcceptsARealClient wires the exported constructor to an actual
// go-redis client, which is the signature a consumer uses. No server is
// contacted: constructing a client does not dial, so this asserts the exported
// path compiles and accepts the driver's types without needing Redis.
// The client is closed by a defer registered *after* the goleak defer so it runs
// first (defers are LIFO): NewClient starts a background goroutine, and
// registering the close with t.Cleanup instead would run it after goleak had
// already looked — which is exactly how the first version of this test failed.
func TestCheckAcceptsARealClient(t *testing.T) {
	defer goleak.VerifyNone(t)
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	defer func() { _ = client.Close() }()

	c := Check("redis", client, WithTimeout(time.Second))
	require.Equal(t, "redis", c.Name)
	require.NotNil(t, c.Probe)
}
