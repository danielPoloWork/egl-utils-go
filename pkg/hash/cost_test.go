package hash_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestHashPasswordCostRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Cost 10 keeps the suite fast; the cost knob itself is verified by the
	// stored-cost assertions below rather than by hashing at an expensive factor.
	h, err := hash.HashPasswordCost("correct horse battery staple", 10)
	require.NoError(t, err)
	require.NoError(t, hash.CheckPassword("correct horse battery staple", h))
	require.ErrorIs(t, hash.CheckPassword("wrong", h), hash.ErrMismatch)
}

func TestHashPasswordCostStoresTheRequestedCost(t *testing.T) {
	defer goleak.VerifyNone(t)
	// 11 is the cheapest cost distinguishable from the default, so this proves the
	// argument is honoured rather than silently replaced (~110ms on the reference
	// workstation — the most this suite is willing to spend on the assertion).
	for _, cost := range []int{10, 11} {
		t.Run(fmt.Sprintf("cost %d", cost), func(t *testing.T) {
			h, err := hash.HashPasswordCost("pw", cost)
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(h, fmt.Sprintf("$2a$%02d$", cost)),
				"the cost is encoded in the hash prefix: %q", h)

			got, err := hash.Cost(h)
			require.NoError(t, err)
			require.Equal(t, cost, got, "Cost reads back the factor that produced the hash")
		})
	}
}

func TestHashPasswordDelegatesAtDefaultCost(t *testing.T) {
	defer goleak.VerifyNone(t)
	h, err := hash.HashPassword("pw")
	require.NoError(t, err)
	got, err := hash.Cost(h)
	require.NoError(t, err)
	require.Equal(t, 10, got, "HashPassword is HashPasswordCost at bcrypt's default cost")
}

// TestHashPasswordCostRejectsOutOfRange is the security assertion of this
// change. Every case here is one bcrypt itself would have accepted — silently
// promoting the sub-MinCost values to cost 10 and honouring 4-9 verbatim as a
// genuinely weak hash (see TestBcryptWouldAcceptWeakCosts, which pins that
// upstream behaviour). None of them may produce a hash here.
func TestHashPasswordCostRejectsOutOfRange(t *testing.T) {
	defer goleak.VerifyNone(t)
	cases := []struct {
		name string
		cost int
	}{
		{"zero value from an unset config field", 0},
		{"negative", -1},
		{"bcrypt's own floor is far too weak", 4},
		{"one below our floor", 9},
		{"one above bcrypt's ceiling", 32},
		{"absurdly high", 1_000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := hash.HashPasswordCost("pw", tc.cost)
			require.ErrorIs(t, err, hash.ErrInvalidCost)
			require.Empty(t, h, "a rejected cost produces no hash at all")
			require.Contains(t, err.Error(), fmt.Sprint(tc.cost),
				"the error names the offending value so a misconfiguration is diagnosable")
		})
	}
}

func TestHashPasswordCostAcceptsTheRangeBoundaries(t *testing.T) {
	defer goleak.VerifyNone(t)
	// The lower boundary is cheap to exercise. The upper boundary (31) is not:
	// it is roughly 2^21 times the default and would take hours, so it is
	// asserted as *accepted* by the range check via the error surface only —
	// hashing at it is a denial of service against the test suite, exactly as
	// the package documentation warns.
	h, err := hash.HashPasswordCost("pw", 10)
	require.NoError(t, err, "cost 10 is the accepted floor")
	require.NotEmpty(t, h)

	// 31 must not be rejected by the range check: the failure mode we guard
	// against is an off-by-one at the ceiling that makes the documented range a
	// lie. Pair it with an over-long password so bcrypt refuses on the length
	// *before* doing any work, which proves the cost passed validation without
	// ever paying for it.
	_, err = hash.HashPasswordCost(strings.Repeat("a", 73), 31)
	require.ErrorIs(t, err, hash.ErrPasswordTooLong,
		"cost 31 is inside the accepted range, so the length is what fails")
	require.NotErrorIs(t, err, hash.ErrInvalidCost)
}

func TestHashPasswordCostValidatesCostBeforePassword(t *testing.T) {
	defer goleak.VerifyNone(t)
	// Both inputs are invalid; the contract fixes which error wins, so callers
	// can rely on the cost being reported first.
	_, err := hash.HashPasswordCost(strings.Repeat("a", 73), 9)
	require.ErrorIs(t, err, hash.ErrInvalidCost)
	require.NotErrorIs(t, err, hash.ErrPasswordTooLong)
}

func TestHashPasswordCostPropagatesTooLong(t *testing.T) {
	defer goleak.VerifyNone(t)
	_, err := hash.HashPasswordCost(strings.Repeat("a", 73), 10)
	require.ErrorIs(t, err, hash.ErrPasswordTooLong)

	h, err := hash.HashPasswordCost(strings.Repeat("a", 72), 10) // exactly at the limit
	require.NoError(t, err)
	require.NoError(t, hash.CheckPassword(strings.Repeat("a", 72), h))
}

func TestCostRejectsMalformedHash(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, tc := range []struct{ name, in string }{
		{"not a hash at all", "not-a-bcrypt-hash"},
		{"empty", ""},
		{"truncated bcrypt hash", "$2a$10$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := hash.Cost(tc.in)
			require.Error(t, err)
			require.Zero(t, c)
		})
	}
}

// TestCostUpgradeOnLogin walks the migration path the package documentation
// prescribes, so the documented procedure is executable rather than aspirational.
func TestCostUpgradeOnLogin(t *testing.T) {
	defer goleak.VerifyNone(t)
	const pw, target = "correct horse battery staple", 11

	stored, err := hash.HashPasswordCost(pw, 10) // the legacy hash
	require.NoError(t, err)

	// A login: verify first — only a verified plaintext may be rehashed.
	require.NoError(t, hash.CheckPassword(pw, stored))

	current, err := hash.Cost(stored)
	require.NoError(t, err)
	require.Less(t, current, target, "this hash is below the deployment's target")

	upgraded, err := hash.HashPasswordCost(pw, target)
	require.NoError(t, err)

	got, err := hash.Cost(upgraded)
	require.NoError(t, err)
	require.Equal(t, target, got, "the replacement hash sits at the new work factor")
	require.NoError(t, hash.CheckPassword(pw, upgraded), "the user is not locked out")
	require.NoError(t, hash.CheckPassword(pw, stored),
		"the legacy hash keeps working until it is replaced — no flag day")
}

// BenchmarkHashPasswordCost is the cost-sizing benchmark spec §7 requires, so a
// deployer can size the work factor on their own hardware rather than trusting
// the numbers in the package documentation. Each step of cost doubles the work.
//
// It stops at 14 deliberately: the doubling makes higher costs trivially
// extrapolable, and a full sweep to 31 would take days.
func BenchmarkHashPasswordCost(b *testing.B) {
	for cost := 10; cost <= 14; cost++ {
		b.Run(fmt.Sprintf("cost=%d", cost), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := hash.HashPasswordCost("correct horse battery staple", cost); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCheckPassword measures the verify side, which is the number that
// bounds login throughput: every login pays it, on an endpoint an
// unauthenticated caller can reach.
func BenchmarkCheckPassword(b *testing.B) {
	const pw = "correct horse battery staple"
	for cost := 10; cost <= 14; cost++ {
		h, err := hash.HashPasswordCost(pw, cost)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("cost=%d", cost), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if err := hash.CheckPassword(pw, h); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCost(b *testing.B) {
	h, err := hash.HashPasswordCost("pw", 10)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for range b.N {
		if _, err := hash.Cost(h); err != nil {
			b.Fatal(err)
		}
	}
}
