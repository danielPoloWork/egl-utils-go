# ADR-0024: hash password design — bcrypt at default cost, per-hash salt, constant-time verify

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role (sign-off required)
- **Related:** spec §2 feature 20, §5 (`HashPassword(pw string) (string, error)`,
  `CheckPassword(pw, hash string) error`); ROADMAP 8.2 (completes Milestone 8); ADR-0004 (runtime
  dependency policy — bcrypt via `golang.org/x/crypto` is pre-approved ring 2); ADR-0005
  (loud-by-default); threat model (credential store); compliance control C-4 (new)

## Context

Feature 20 is password hashing and verification, frozen at intake as
`HashPassword(pw string) (string, error)` and `CheckPassword(pw, hash string) error`, and the spec
(§2.20) names **bcrypt**. This is a security-relevant decision, so under the enterprise posture
(ADR-0015, AGENTS.md §7/§10) it **requires this ADR and the security-auditor's sign-off**. ADR-0004
already budgeted bcrypt: "Password hashing requires bcrypt (`golang.org/x/crypto`) in any case" —
ring 2 (`golang.org/x/*`, extended stdlib), no new dependency ring. The open decisions are the
work factor, salting, input limits, verification behaviour, and the error surface.

## Decision

**bcrypt, at bcrypt's default cost (10).** bcrypt is adaptive (a tunable work factor), salted, and
deliberately slow — the properties that make an offline attack on a leaked hash store expensive.
The frozen `HashPassword(pw string)` signature carries no cost parameter, so the default cost (10)
is used: bcrypt's own default, and within OWASP's accepted range. A higher cost strengthens the hash
but multiplies per-login CPU and thus the DoS surface of an unauthenticated login endpoint; 10 is the
balanced default. A configurable-cost variant (`HashPasswordCost`) is a clean additive extension and
is deferred.

**Per-hash random salt, embedded in the output.** bcrypt generates a fresh salt per call and encodes
the algorithm, cost, and salt into the returned string, so every call yields a different hash and no
separate salt column is needed. Tests assert both (distinct hashes for the same password; both
verify).

**Reject over-long input, never truncate.** bcrypt hashes at most 72 bytes; Go's implementation
returns an error rather than silently truncating (which would let two distinct long passwords
collide). `HashPassword` surfaces this as a wrapped `ErrPasswordTooLong`.

**Constant-time verification, generic mismatch.** `CheckPassword` uses
`bcrypt.CompareHashAndPassword`, which compares in constant time relative to the hash, avoiding a
timing oracle. A wrong password returns the package sentinel `ErrMismatch`; a malformed hash returns
a distinct wrapped error (an operational problem, not a wrong password). The godoc instructs callers
to surface only a **generic** failure to the end user — never revealing whether the identifier or
the password was wrong (which would enable user enumeration).

**Self-contained error surface.** `ErrMismatch` is our own sentinel (translated from bcrypt's);
`ErrPasswordTooLong` is bcrypt's sentinel re-exported. Either way a caller uses `errors.Is` against
`hash.*` without importing bcrypt.

**Dependency pinning preserves the 1.24 floor.** `golang.org/x/crypto` is pinned at **v0.48.0** —
the newest release whose `go` directive is still `1.24.0`; v0.50.0+ declare `go 1.25.0` and would
raise the module's floor. Adding it normalized our directive `go 1.24` → `go 1.24.0` (the same
1.24 floor, made explicit), exactly the discipline ADR-0009 used for `golang.org/x/sync`. `go.sum`
committed; `govulncheck` reports **0 called vulnerabilities** (only `bcrypt` is imported; advisories
in x/crypto's unused packages such as `ssh` are not reachable).

## Alternatives Considered

- **argon2id / scrypt** — argon2id is the current best-practice memory-hard KDF, but the spec names
  bcrypt, bcrypt is in the pre-approved ring, and argon2id's parameters are a larger tuning/portability
  surface. Rejected for spec-named bcrypt; a future algorithm migration is a MAJOR-version concern.
- **A configurable cost parameter now** — more flexible, but not the frozen signature; deferred as an
  additive `HashPasswordCost`.
- **A higher fixed cost (12+)** — stronger, but meaningfully raises login latency and the DoS surface;
  a library should not impose that unilaterally. Rejected for the bcrypt default, tunable later.
- **Silently truncating at 72 bytes** — bcrypt's historical footgun (distinct long passwords collide);
  rejected for the explicit `ErrPasswordTooLong`.
- **Returning bcrypt's raw errors** — leaks the bcrypt dependency into every caller's error handling;
  rejected for the translated/re-exported `hash.*` sentinels.
- **Pinning the latest x/crypto (v0.54.0)** — newest, but raises the module floor to Go 1.25, dropping
  the documented 1.24 support. Rejected for v0.48.0.

## Consequences

- The module gains the `hash` package (`HashPassword`, `CheckPassword`, `ErrMismatch`,
  `ErrPasswordTooLong`) and its first new runtime dependency since yaml.v3: `golang.org/x/crypto`
  (ring 2). The 1.24 floor is preserved.
- Passwords are stored only as salted, adaptive, cost-10 bcrypt hashes; verification is constant-time;
  plaintext is never logged or persisted by this package — recorded as compliance control **C-4** and
  a threat-model row (offline cracking / timing oracle / user enumeration).
- **Milestone 8 (validation & security) is complete.**
- Deferred, additive: configurable cost, a cost-upgrade helper (`bcrypt.Cost` + rehash-on-login),
  argon2id behind a MAJOR bump.

## References

- `docs/specs/01_spec_utils.md` §2.20, §5.
- ADR-0004 (bcrypt pre-approved, ring 2), ADR-0009 (x/sync floor-preserving pin precedent), ADR-0015
  (enterprise posture — why this needs an ADR + auditor sign-off), ADR-0005 (loud-by-default).
- `docs/security/threat-model.md` — credential-store rows. `docs/compliance/README.md` — control C-4.
- `golang.org/x/crypto/bcrypt` — `GenerateFromPassword`, `CompareHashAndPassword`, `DefaultCost`.
