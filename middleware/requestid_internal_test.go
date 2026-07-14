package middleware

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"pgregory.net/rapid"
)

func TestIsValidIDBoundaries(t *testing.T) {
	defer goleak.VerifyNone(t)
	valid := []string{
		"a",
		"550e8400-e29b-41d4-a716-446655440000",
		"01ARZ3NDEKTSV4RRFFQ69G5FAV", // ULID
		"deadbeefcafe",
		"a/b+c=d_e-f.g", // base64url + base64 punctuation
		string(fill('x', maxIDLen)),
	}
	for _, id := range valid {
		require.Truef(t, isValidID(id), "expected valid: %q (len %d)", id, len(id))
	}

	invalid := []string{
		"",                            // empty
		string(fill('x', maxIDLen+1)), // one over the cap
		"a b",                         // space (0x20)
		"a\tb",                        // tab
		"a\rb",                        // CR
		"a\nb",                        // LF
		"a\x00b",                      // NUL
		"a\x7fb",                      // DEL
		"café",                        // non-ASCII (multi-byte)
	}
	for _, id := range invalid {
		require.Falsef(t, isValidID(id), "expected invalid: %q", id)
	}
}

// TestIsValidIDProperty pins the acceptance rule end to end: an ID is valid
// exactly when it is non-empty, at most maxIDLen bytes, and every byte is a
// visible ASCII character (0x21–0x7e).
func TestIsValidIDProperty(t *testing.T) {
	defer goleak.VerifyNone(t)
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.String().Draw(rt, "id")
		want := len(id) > 0 && len(id) <= maxIDLen && allVisibleASCII(id)
		require.Equalf(rt, want, isValidID(id), "disagreed on %q", id)
	})
}

func TestGenerateIDIsValidAndUnpredictable(t *testing.T) {
	defer goleak.VerifyNone(t)
	first := generateID()
	require.True(t, isValidID(first), "a generated ID must itself pass validation")
	require.NotEqual(t, first, generateID(), "generated IDs must differ")
}

func allVisibleASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func fill(b byte, n int) []byte {
	s := make([]byte, n)
	for i := range s {
		s[i] = b
	}
	return s
}
