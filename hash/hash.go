// Package hash hashes and verifies passwords with bcrypt.
//
// HashPassword derives a salted bcrypt hash suitable for storage; CheckPassword
// verifies a candidate password against such a hash in constant time. bcrypt is
// an adaptive, deliberately slow algorithm: the per-hash work factor (cost) and
// the per-hash random salt are what make an offline attack on a leaked hash
// store expensive, and both are embedded in the returned hash string, so no
// separate salt column is needed.
//
// The cost is bcrypt's standard default (10). bcrypt hashes at most 72 bytes of
// input; a longer password is rejected with ErrPasswordTooLong rather than
// silently truncated (which would let two distinct long passwords collide). All
// errors this package returns are package sentinels or wrapped values — callers
// never need to import the underlying bcrypt package.
package hash

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrMismatch is returned by CheckPassword when the password does not match the
// hash. Callers should treat it as "wrong credentials" and surface a generic
// failure to the end user — never reveal which of the identifier or the
// password was wrong.
var ErrMismatch = errors.New("hash: password does not match")

// ErrPasswordTooLong is returned (wrapped) by HashPassword when pw exceeds
// bcrypt's 72-byte input limit. It is bcrypt's own sentinel, re-exported so a
// caller can test for it with errors.Is without importing bcrypt.
var ErrPasswordTooLong = bcrypt.ErrPasswordTooLong

// HashPassword returns a salted bcrypt hash of pw at the default cost, safe to
// store and later pass to CheckPassword. Each call produces a different hash
// (a fresh random salt), and every hash still verifies. It returns a wrapped
// ErrPasswordTooLong if pw is longer than bcrypt's 72-byte limit.
//
// The name stutters as hash.HashPassword but is frozen by spec §5; renaming it
// would break the public API, so the revive stutter check is suppressed.
//
//nolint:revive // name frozen by spec §5 (see above)
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash: generate password hash: %w", err)
	}
	return string(b), nil
}

// CheckPassword reports whether pw matches the bcrypt hash. It returns nil on a
// match, ErrMismatch when the password is wrong, and a wrapped error when hash
// is not a valid bcrypt hash. The comparison is constant-time with respect to
// the hash contents (bcrypt), so it does not leak information through timing.
func CheckPassword(pw, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw))
	switch {
	case err == nil:
		return nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return ErrMismatch
	default:
		return fmt.Errorf("hash: compare password: %w", err)
	}
}
