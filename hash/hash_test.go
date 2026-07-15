package hash_test

import (
	"strings"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestHashAndCheckRoundTrip(t *testing.T) {
	defer goleak.VerifyNone(t)
	h, err := hash.HashPassword("correct horse battery staple")
	require.NoError(t, err)
	require.NotEmpty(t, h)
	require.NotEqual(t, "correct horse battery staple", h, "the hash is not the plaintext")
	require.NoError(t, hash.CheckPassword("correct horse battery staple", h))
}

func TestHashUsesDefaultCost(t *testing.T) {
	defer goleak.VerifyNone(t)
	h, err := hash.HashPassword("pw")
	require.NoError(t, err)
	// bcrypt encodes the algorithm and cost in the prefix; default cost is 10.
	require.True(t, strings.HasPrefix(h, "$2a$10$"), "unexpected hash prefix: %q", h)
}

func TestHashIsSaltedPerCall(t *testing.T) {
	defer goleak.VerifyNone(t)
	h1, err := hash.HashPassword("same-password")
	require.NoError(t, err)
	h2, err := hash.HashPassword("same-password")
	require.NoError(t, err)
	require.NotEqual(t, h1, h2, "a fresh random salt makes each hash distinct")
	require.NoError(t, hash.CheckPassword("same-password", h1))
	require.NoError(t, hash.CheckPassword("same-password", h2))
}

func TestCheckWrongPasswordReturnsMismatch(t *testing.T) {
	defer goleak.VerifyNone(t)
	h, err := hash.HashPassword("right")
	require.NoError(t, err)
	err = hash.CheckPassword("wrong", h)
	require.ErrorIs(t, err, hash.ErrMismatch)
}

func TestCheckMalformedHashIsNotMismatch(t *testing.T) {
	defer goleak.VerifyNone(t)
	err := hash.CheckPassword("pw", "not-a-bcrypt-hash")
	require.Error(t, err)
	require.NotErrorIs(t, err, hash.ErrMismatch, "a malformed hash is a real error, not a wrong password")
}

func TestHashPasswordTooLong(t *testing.T) {
	defer goleak.VerifyNone(t)
	_, err := hash.HashPassword(strings.Repeat("a", 73))
	require.ErrorIs(t, err, hash.ErrPasswordTooLong)

	h, err := hash.HashPassword(strings.Repeat("a", 72)) // exactly at the limit
	require.NoError(t, err)
	require.NoError(t, hash.CheckPassword(strings.Repeat("a", 72), h))
}

func TestEmptyPassword(t *testing.T) {
	defer goleak.VerifyNone(t)
	h, err := hash.HashPassword("")
	require.NoError(t, err, "hashing an empty password is the caller's policy, not an error here")
	require.NoError(t, hash.CheckPassword("", h))
	require.ErrorIs(t, hash.CheckPassword("x", h), hash.ErrMismatch)
}
