// Package hash hashes and verifies passwords with bcrypt.
//
// HashPassword derives a salted bcrypt hash suitable for storage; CheckPassword
// verifies a candidate password against such a hash in constant time. bcrypt is
// an adaptive, deliberately slow algorithm: the per-hash work factor (cost) and
// the per-hash random salt are what make an offline attack on a leaked hash
// store expensive, and both are embedded in the returned hash string, so no
// separate salt column is needed.
//
// HashPassword uses cost 12. HashPasswordCost takes the cost explicitly, and
// Cost reads the cost back out of a stored hash so a deployment can raise its
// work factor over time. bcrypt hashes at most 72 bytes
// of input; a longer password is rejected with ErrPasswordTooLong rather than
// silently truncated (which would let two distinct long passwords collide). All
// errors this package returns are package sentinels or wrapped values — callers
// never need to import the underlying bcrypt package.
//
// # Choosing a cost
//
// Each step of cost doubles the work, for hashing and for verification alike —
// measured, not merely claimed: on the reference workstation (Intel i5-6600K @
// 3.5GHz) a single hash or verify costs
//
//	cost 10   ~55 ms      (the accepted floor, and bcrypt's own default)
//	cost 11  ~111 ms
//	cost 12  ~222 ms      (this package's default)
//	cost 13  ~443 ms
//	cost 14  ~887 ms
//
// The default is 12 rather than bcrypt's 10 because a library default is what
// every deployment that never revisits the question ships with, and 12 is the
// current industry recommendation for bcrypt. It is four times bcrypt's default
// in both directions: four times the work an offline attacker must spend per
// candidate password, and four times the CPU every login costs the deployment.
// Cost 10 remains inside the accepted range, so a deployment whose login path
// cannot absorb ~222 ms of CPU can ask for it explicitly — deliberately, with
// the number in its own source, rather than by accepting a default.
//
// Verification costs the same as hashing at the same factor, and that is the
// number that matters operationally, because every login pays it while hashing
// happens only at registration and password change.
//
// Measure on the hardware you deploy on — see docs/benchmarks/ for the
// cost-sizing report and BenchmarkHashPasswordCost/BenchmarkCheckPassword to
// reproduce it. Pick the highest cost whose verify latency your login path can
// absorb, and remember that the cost is a per-login CPU multiplier on an endpoint
// an unauthenticated caller can reach: a cost chosen for offline-cracking
// resistance alone turns logins into a denial-of-service amplifier — at the
// default cost of 12, roughly four and a half verifications per second saturate
// one core, so five concurrent login attempts are enough. Rate-limit the login endpoint
// accordingly (ratelimit.(*Limiter).Middleware sheds over-budget requests).
//
// The accepted range's upper bound (31) is bcrypt's own limit, not a
// recommendation: it is 2²¹ times the default, extrapolating to well over a day
// per hash on the reference hardware — a denial of service against yourself.
//
// # Migrating to a stronger hash
//
// For a new system, prefer argon2id (golang.org/x/crypto/argon2) over bcrypt:
// it is memory-hard, so it resists GPU and ASIC cracking far better than
// bcrypt's CPU-bound work factor, and it has no 72-byte input limit. This
// package deliberately stays on bcrypt because that is the algorithm the spec
// froze; argon2id would be a new algorithm surface with its own parameter
// tuning, not a drop-in change.
//
// Either migration — a cost upgrade, or bcrypt → argon2id — follows the same
// verify-and-rehash-on-login pattern, because a stored hash can only be upgraded
// when the plaintext is in hand:
//
//  1. Tag each stored hash with the algorithm that produced it. bcrypt hashes
//     are already self-identifying (they begin "$2a$"/"$2b$"), so a tag column
//     is only needed once a second algorithm is in play.
//  2. On a successful login, check whether the stored hash is below target —
//     for a cost upgrade, Cost(stored) < target; for an algorithm upgrade, the
//     tag names the old algorithm.
//  3. If so, rehash the plaintext just verified at the new cost or algorithm and
//     replace the stored hash. Never rehash a hash: only the plaintext supplied
//     at login can produce the stronger one.
//
// Old hashes therefore upgrade as users log in, and no user is locked out. Hashes
// belonging to users who never return stay at the old strength, so pair this with
// a policy for dormant accounts rather than assuming the store converges.
//
// This is also the path from a store written by v1 of this module, whose default
// was cost 10: those hashes keep verifying unchanged — bcrypt reads the factor
// from the hash itself, so nothing is invalidated by the new default — and they
// move to 12 as their owners log in.
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

// ErrPasswordTooLong is returned (wrapped) by HashPassword and HashPasswordCost
// when pw exceeds bcrypt's 72-byte input limit. It is bcrypt's own sentinel,
// re-exported so a caller can test for it with errors.Is without importing
// bcrypt.
var ErrPasswordTooLong = bcrypt.ErrPasswordTooLong

// ErrInvalidCost is returned (wrapped, with the offending value) by
// HashPasswordCost when cost is outside the accepted range 10–31.
var ErrInvalidCost = errors.New("hash: cost outside the accepted range 10-31")

// defaultCost is the work factor HashPassword produces. It is this package's own
// number, deliberately above bcrypt.DefaultCost (10): a default is what every
// deployment that never revisits the question ships with, so it is set to the
// current recommendation rather than to the weakest value the package accepts.
// The gap between it and minCost is intentional — asking for 10 stays legal, but
// it has to be asked for.
//
// Raising it again is a breaking change under the module's compatibility contract
// (ADR-0042): it is API-compatible but quadruples the CPU every login costs, so it
// belongs to a major. Raising the cost of an existing store does not — see the
// migration section in the package documentation.
const defaultCost = 12

// minCost is the weakest work factor this package will produce. It is
// deliberately stricter than bcrypt's own MinCost of 4, which is far too weak to
// offer meaningful resistance to offline cracking on current hardware; 10 is
// bcrypt's default and the floor OWASP accepts.
//
// The strictness is not cosmetic. bcrypt.GenerateFromPassword silently *promotes*
// a cost below its MinCost of 4 to DefaultCost — so a caller passing 0 (a zero
// value from an unset config field) would get a cost-10 hash and no indication
// its intent was ignored — while a cost of 4 through 9 is accepted verbatim and
// produces a genuinely weak hash. Neither outcome is acceptable silently, so the
// range is enforced here before the value ever reaches bcrypt.
const minCost = 10

// maxCost is bcrypt's own ceiling. See the package documentation: this is a
// hard limit, not a recommendation — the upper end of the range is unusable in
// practice.
const maxCost = bcrypt.MaxCost

// HashPassword returns a salted bcrypt hash of pw at this package's default cost
// (12), safe to store and later pass to CheckPassword. Each call produces a
// different hash (a fresh random salt), and every hash still verifies. It returns
// a wrapped ErrPasswordTooLong if pw is longer than bcrypt's 72-byte limit.
//
// The default costs roughly 222 ms of CPU per call on the reference hardware, and
// verification costs the same, so every login pays it — see "Choosing a cost" in
// the package documentation before deciding this default is right for the login
// path it sits behind. Use HashPasswordCost to choose the work factor explicitly;
// hashes written at any accepted cost keep verifying regardless of what the
// default becomes.
//
// The name stutters as hash.HashPassword but is frozen by spec §5; renaming it
// would break the public API, so the revive stutter check is suppressed.
//
//nolint:revive // name frozen by spec §5 (see above)
func HashPassword(pw string) (string, error) {
	return HashPasswordCost(pw, defaultCost)
}

// HashPasswordCost is HashPassword with an explicit bcrypt work factor. cost
// must be in the range 10–31; anything outside it returns a wrapped
// ErrInvalidCost naming the offending value, and no hash is produced. See the
// package documentation for how to choose a cost, why the upper bound is not a
// recommendation, and how to raise the cost of an existing hash store.
//
// The cost is validated before pw is examined, so an out-of-range cost is
// reported as ErrInvalidCost even when pw would also have been too long.
//
// The name stutters as hash.HashPasswordCost but matches the frozen
// HashPassword; the revive stutter check is suppressed for consistency.
//
//nolint:revive // name matches the spec-frozen HashPassword (see above)
func HashPasswordCost(pw string, cost int) (string, error) {
	if cost < minCost || cost > maxCost {
		return "", fmt.Errorf("hash: cost %d: %w", cost, ErrInvalidCost)
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	if err != nil {
		return "", fmt.Errorf("hash: generate password hash: %w", err)
	}
	return string(b), nil
}

// Cost returns the bcrypt work factor stored in hash, reading it from the hash
// string itself — no password required. It returns a wrapped error when hash is
// not a valid bcrypt hash.
//
// Cost is what makes a cost upgrade actionable: compare it against the cost the
// deployment now targets to decide whether a just-verified password should be
// rehashed (see the package documentation). It is also how a hash store is
// audited for hashes still sitting at an old work factor.
func Cost(hash string) (int, error) {
	c, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return 0, fmt.Errorf("hash: read cost: %w", err)
	}
	return c, nil
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
