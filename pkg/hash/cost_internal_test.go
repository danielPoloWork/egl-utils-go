package hash

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/crypto/bcrypt"
)

// TestBcryptWouldAcceptWeakCosts pins the upstream behaviour that makes the
// range check in HashPasswordCost necessary rather than decorative. It asserts
// against bcrypt directly — the only place in this package's tests that does —
// because the point is precisely what bcrypt does when our guard is absent.
//
// Two distinct footguns, neither of which reports anything to the caller:
//
//   - a cost below bcrypt's MinCost (4) is silently *promoted* to DefaultCost,
//     so a zero value from an unset config field yields a cost-10 hash and the
//     caller's intent is discarded without a word;
//   - a cost of 4 through 9 is honoured verbatim, producing a hash that is up to
//     64 times cheaper to crack than the default and looks entirely normal.
//
// If this test ever fails, upstream changed its validation and the comments
// justifying our stricter floor need re-checking — not the floor itself.
func TestBcryptWouldAcceptWeakCosts(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.Equal(t, 4, bcrypt.MinCost, "the upstream floor our minCost deliberately exceeds")
	require.Equal(t, 10, bcrypt.DefaultCost,
		"the upstream default our defaultCost deliberately exceeds, and the value sub-MinCost costs "+
			"are silently promoted to below")
	require.Equal(t, 31, bcrypt.MaxCost, "maxCost tracks this")
	require.Equal(t, bcrypt.MaxCost, maxCost)
	require.Greater(t, minCost, bcrypt.MinCost, "our floor is stricter than bcrypt's")

	t.Run("sub-MinCost is silently promoted to the default", func(t *testing.T) {
		for _, cost := range []int{-1, 0, 3} {
			b, err := bcrypt.GenerateFromPassword([]byte("pw"), cost)
			require.NoError(t, err, "bcrypt accepts cost %d without complaint", cost)
			got, err := bcrypt.Cost(b)
			require.NoError(t, err)
			require.Equal(t, bcrypt.DefaultCost, got,
				"cost %d silently became %d — the caller is never told", cost, got)
		}
	})

	t.Run("costs 4 to 9 are honoured verbatim and are weak", func(t *testing.T) {
		// Only the boundaries are exercised; hashing at every value in between
		// buys nothing and costs suite time.
		for _, cost := range []int{4, 9} {
			b, err := bcrypt.GenerateFromPassword([]byte("pw"), cost)
			require.NoError(t, err, "bcrypt accepts weak cost %d", cost)
			got, err := bcrypt.Cost(b)
			require.NoError(t, err)
			require.Equal(t, cost, got, "the weak factor is stored as-is")
			require.Less(t, got, minCost, "and it is below the floor this package enforces")
		}
	})

	t.Run("above MaxCost is the one case bcrypt itself refuses", func(t *testing.T) {
		_, err := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MaxCost+1)
		require.Error(t, err, "the ceiling is enforced upstream; the floor is not")
	})
}

// TestDefaultCostIsInsideTheAcceptedRange guards the one way this package's own
// default can break it: HashPassword delegates to HashPasswordCost, which
// validates, so a default outside 10-31 would make HashPassword return
// ErrInvalidCost for every password — a total outage of the hashing path that no
// black-box test of the default cost's *value* would explain.
//
// It also pins the deliberate gap: the default sits above the floor, so asking
// for the floor stays legal but has to be asked for.
func TestDefaultCostIsInsideTheAcceptedRange(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.GreaterOrEqual(t, defaultCost, minCost, "HashPassword would reject every password")
	require.LessOrEqual(t, defaultCost, maxCost, "HashPassword would reject every password")
	require.Greater(t, defaultCost, minCost,
		"the default is deliberately stronger than the weakest accepted cost")
	require.Greater(t, defaultCost, bcrypt.DefaultCost,
		"and deliberately stronger than bcrypt's own default")
}
