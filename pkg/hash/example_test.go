package hash_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/hash"
)

// The registration-then-login pair, which is the whole of the package's normal
// use: hash once at registration, store the string, and verify a candidate
// against it at every login.
//
// Note what these examples never do: hash in a loop. At the default cost of 12
// a single hash or verify is roughly 222 ms of CPU on the reference hardware —
// that is the point of an adaptive algorithm — so this example does three
// bcrypt operations in total and no more.
func ExampleHashPassword() {
	// Each call produces a different string (a fresh random salt is embedded in
	// it), so the hash is never compared for equality — only verified. There is
	// no salt column to store.
	stored, err := hash.HashPassword("correct-horse-battery-staple")
	fmt.Println(err == nil)

	fmt.Println(hash.CheckPassword("correct-horse-battery-staple", stored) == nil)

	// A wrong password is ErrMismatch, not a generic failure — but the caller
	// should still surface one message for a bad password and a bad username
	// alike, or the response tells an attacker which identifiers exist.
	err = hash.CheckPassword("hunter2", stored)
	fmt.Println(errors.Is(err, hash.ErrMismatch))
	// Output:
	// true
	// true
	// true
}

// bcrypt hashes at most 72 bytes. A longer password is refused rather than
// silently truncated, because truncation would make two distinct long passwords
// verify against the same hash — the caller must decide what to do, and this is
// where it finds out.
func ExampleHashPassword_tooLong() {
	// 73 bytes — the limit is on bytes, not runes, so a passphrase of accented
	// or CJK characters reaches it in far fewer than 72 characters.
	_, err := hash.HashPassword(strings.Repeat("a", 73))
	fmt.Println(errors.Is(err, hash.ErrPasswordTooLong))
	// Output: true
}

// Cost reads the work factor back out of a stored hash, with no password in
// hand. That is what makes a cost upgrade actionable: a store can be audited
// for hashes still at an old factor, and a just-verified login can be rehashed
// at the new one.
//
// The explicit cost 10 here is not a recommendation — it is what a store
// written by an older deployment (or by v1 of this module) contains. It is also
// the cheapest legal factor, which is why this example uses it rather than
// paying the default's 222 ms to demonstrate an unrelated point.
func ExampleCost() {
	const target = 12 // what this deployment now hashes at

	legacy, err := hash.HashPasswordCost("correct-horse-battery-staple", 10)
	fmt.Println(err == nil)

	stored, err := hash.Cost(legacy)
	fmt.Println(err == nil, stored)

	// Old hashes keep verifying unchanged — bcrypt reads the factor from the
	// hash itself, so raising the default invalidates nothing. The upgrade
	// happens on login, with the plaintext in hand: verify, then if the stored
	// cost is below target, rehash that same plaintext and replace the record.
	// Never rehash a hash.
	fmt.Println(stored < target)
	// Output:
	// true
	// true 10
	// true
}

// Cost below 10 or above 31 is refused with ErrInvalidCost and no hash is
// produced. The floor is this package's, not bcrypt's: bcrypt silently promotes
// a cost under 4 to its own default — so an unset config field holding 0 would
// yield a cost-10 hash with no sign the intent was ignored — and accepts 4
// through 9 verbatim, producing a genuinely weak hash. Neither is acceptable
// quietly, so the range is enforced before the value reaches bcrypt.
func ExampleHashPasswordCost_invalidCost() {
	_, err := hash.HashPasswordCost("correct-horse-battery-staple", 0)
	fmt.Println(errors.Is(err, hash.ErrInvalidCost))

	_, err = hash.HashPasswordCost("correct-horse-battery-staple", 32)
	fmt.Println(errors.Is(err, hash.ErrInvalidCost))
	// Output:
	// true
	// true
}
