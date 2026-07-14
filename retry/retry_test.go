package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danielPoloWork/egl-utils-go/retry"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var errBoom = errors.New("boom")

// quickPolicy keeps real-clock tests fast (millisecond delays, small cap)
// while exercising the production jitter path.
func quickPolicy(attempts int) retry.Policy {
	return retry.Policy{
		MaxAttempts: attempts,
		BaseDelay:   time.Millisecond,
		MaxDelay:    4 * time.Millisecond,
		Jitter:      0.5,
	}
}

func TestBackoffNilFunctionPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Panics(t, func() {
		_ = retry.Backoff(context.Background(), quickPolicy(1), nil)
	})
}

func TestBackoffInvalidPolicyPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	bad := []retry.Policy{
		{MaxAttempts: 0},
		{MaxAttempts: -1},
		{MaxAttempts: 1, BaseDelay: -time.Second},
		{MaxAttempts: 1, BaseDelay: 2 * time.Second, MaxDelay: time.Second},
		{MaxAttempts: 1, Jitter: -0.1},
		{MaxAttempts: 1, Jitter: 1.1},
	}
	for _, p := range bad {
		require.Panics(t, func() {
			_ = retry.Backoff(context.Background(), p, func(context.Context) error { return nil })
		}, "policy %+v must be rejected", p)
	}
}

func TestBackoffReturnsNilOnFirstSuccess(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := 0
	err := retry.Backoff(context.Background(), quickPolicy(5), func(context.Context) error {
		calls++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestBackoffRetriesUntilSuccess(t *testing.T) {
	defer goleak.VerifyNone(t)
	calls := 0
	err := retry.Backoff(context.Background(), quickPolicy(5), func(context.Context) error {
		calls++
		if calls < 3 {
			return errBoom
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestBackoffExhaustionReturnsLastErrorVerbatim(t *testing.T) {
	defer goleak.VerifyNone(t)
	errLast := errors.New("the final attempt's error")
	calls := 0
	err := retry.Backoff(context.Background(), quickPolicy(3), func(context.Context) error {
		calls++
		if calls == 3 {
			return errLast
		}
		return errBoom
	})
	require.Equal(t, 3, calls)
	require.Equal(t, errLast, err, "the last error must come back verbatim, not wrapped")
}

func TestBackoffDoneContextPreemptsFirstCall(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := retry.Backoff(ctx, quickPolicy(3), func(context.Context) error {
		calls++
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 0, calls, "fn must not run under a context that is already done")
}

func TestBackoffCancellationDuringWait(t *testing.T) {
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	p := retry.Policy{MaxAttempts: 2, BaseDelay: 10 * time.Second, MaxDelay: 10 * time.Second}
	calls := 0
	err := retry.Backoff(ctx, p, func(context.Context) error {
		calls++
		return errBoom
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, calls, "the second attempt must never start")
}

func TestBackoffPassesContextThrough(t *testing.T) {
	defer goleak.VerifyNone(t)
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "value")
	err := retry.Backoff(ctx, quickPolicy(1), func(got context.Context) error {
		require.Equal(t, "value", got.Value(key{}))
		return nil
	})
	require.NoError(t, err)
}
