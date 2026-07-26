# ADR-0032: configurable bcrypt cost — floor of 10 enforced locally, error not panic, rehash-on-login

- **Status:** Accepted
- **Date:** 2026-07-26
- **Deciders:** senior project architect (agent), maintainer (Daniel Polo), **security-auditor role (sign-off required)**
- **Related:** ADR-0024 (the hash package: bcrypt at default cost — this ADR extends it), ADR-0030 (spec v2.0 reconciliation: 10.5 adopted additively, the default-cost bump deferred to `/v2`), ADR-0005 (loud-by-default), ADR-0018 (`config.Load` — the error-over-panic precedent), ADR-0031 / control C-5 (rate-limit shedding, the mitigation for this cost knob), spec v2.0 item 20 and §7, roadmap 10.5, compliance control C-4 (extended), threat model (credential store)

## Context

ADR-0024 shipped `hash` with bcrypt at the default cost (10) because the spec-frozen
`HashPassword(pw string)` signature carries no cost parameter, and explicitly deferred "a configurable
cost parameter" and "a cost-upgrade helper (`bcrypt.Cost` + rehash-on-login)" as clean additive
extensions. Spec v2.0 item 20 now asks for exactly that: "configurable cost, **default 12** (min
accepted 10)", plus a doc note recommending argon2id for new systems "with a stated migration path
(algorithm tag in stored hash → verify-and-rehash on login)", and §7 requires a "bcrypt cost-factor
benchmark documented so deployers can size it".

ADR-0030 split that request in two. The **default** change 10 → 12 alters documented behaviour of an
existing function, so it is breaking under the v1.0.0 stability commitment and sits in the `/v2`
ledger. The **capability** is additive and ships here, in v1.1.0: a caller who wants cost 12 today
writes `HashPasswordCost(pw, 12)`.

This is a security-relevant change under the enterprise posture (ADR-0015), so it requires this ADR
and the security-auditor's sign-off. Four decisions are open: the accepted range, how an invalid cost
is reported, whether a cost *reader* is part of the surface, and what the documentation must say so the
knob is used safely rather than merely made available.

The decisive input is upstream behaviour, which was verified empirically against
`golang.org/x/crypto v0.48.0` rather than assumed:

| Requested cost | What `bcrypt.GenerateFromPassword` actually does |
|---|---|
| −1, 0, 3 (below `MinCost` = 4) | **Silently promoted to `DefaultCost` (10).** No error, no signal; the caller's intent is discarded. |
| 4 … 9 | **Honoured verbatim.** Produces a real cost-4 … cost-9 hash — up to 64× cheaper to crack than the default — and the hash looks entirely normal. |
| 10 … 31 | Honoured. |
| 32+ | `InvalidCostError`, returned before any work. |

So bcrypt enforces its ceiling but not a meaningful floor, and its one "safety" behaviour — promoting
sub-`MinCost` values — is *silent*, which is worse than either accepting or rejecting them: a zero
value from an unset config field produces a cost-10 hash and reports success, so a misconfiguration
that was meant to be caught looks like it worked.

## Decision

`HashPasswordCost(pw string, cost int) (string, error)` accepts **cost 10–31 and validates the range
locally, before the value reaches bcrypt**, returning a wrapped `ErrInvalidCost` naming the offending
value; `HashPassword` becomes `HashPasswordCost(pw, bcrypt.DefaultCost)` so there is exactly one code
path; `Cost(hash string) (int, error)` reads the work factor back out of a stored hash so a cost
upgrade is actionable without importing bcrypt; and the package documentation carries the measured
cost table, the login-DoS warning with its `ratelimit` mitigation, the argon2id recommendation, and the
verify-and-rehash-on-login procedure — with the cost-sizing benchmark (§7) reported under
`docs/benchmarks/`.

The floor is **10, not bcrypt's 4**, and the range is enforced here rather than delegated, precisely
because upstream would silently accept every weak value in the table above.

## Alternatives Considered

- **Delegating range validation to bcrypt** — less code, one source of truth. Rejected on the
  evidence: bcrypt has no floor worth the name (cost 4 is accepted, cost 0 is silently rewritten), so
  delegating would ship the exact silent-weakening failure this function exists to prevent. An internal
  test (`TestBcryptWouldAcceptWeakCosts`) pins that upstream behaviour so the justification stays
  checkable and a future upstream change surfaces as a test failure rather than a stale comment.
- **Panicking on an out-of-range cost**, per ADR-0005's loud-by-default and the precedent of
  `NewLimiter`, `cache.NewInMemory`, and `Cors`, which all panic on invalid configuration. Rejected,
  and the distinction is structural rather than stylistic: those are **constructors with no error
  channel**, so a panic is their only loud option, and they run once at wiring time. `HashPasswordCost`
  already returns an error, is called per hash rather than per process, and sits on a request path
  (registration, password change). `config.Load` is the closer in-repo precedent — configuration-shaped
  invalidity reported as `ErrUnsupportedFormat` because an error channel exists. A returned error is
  not silent (errcheck gates it), so loud-by-default is satisfied without turning a misconfigured cost
  into a panic inside a live handler.
- **A `cost <= 0 means default` convenience** — ergonomic for callers threading an optional config
  field. Rejected: it reproduces bcrypt's silent promotion, the single worst behaviour in the table
  above. A caller who wants the default calls `HashPassword`; a zero cost is a misconfiguration and
  is reported as one.
- **Adopting v2's default of 12 for `HashPassword` now** — what spec v2 literally asks for. Rejected
  as breaking: the cost is documented behaviour and a silent 4× latency increase on every login would
  land in a patch upgrade. Deferred to the `/v2` ledger per ADR-0030; reachable today via
  `HashPasswordCost(pw, 12)`.
- **Exporting `MinCost` / `MaxCost` / `DefaultCost` constants** so a caller can pre-validate an
  operator-supplied cost. Genuinely useful, and rejected only on surface discipline: three more
  symbols permanently frozen by the v1 commitment, when the range is documented, stated in the error
  message, and cheaply discovered by calling the function. Deferred and additive.
- **Omitting `Cost` and telling callers to use `bcrypt.Cost`** — the strict reading of roadmap 10.5,
  which lists only the constructor, the doc note, and the benchmark. Rejected because it makes the
  *required* deliverable incoherent: the mandated migration note prescribes rehash-on-login, which
  needs the stored factor, while the package's own contract promises callers "never need to import the
  underlying bcrypt package" (ADR-0024's self-contained error surface, stated in the package doc).
  Documenting a procedure the package cannot perform, or that breaks its own encapsulation promise,
  is worse than one small additive accessor. This is the one deliberate widening of 10.5's literal
  scope and is flagged as such; it is one function and one test to remove if the maintainer disagrees.
- **`NeedsRehash(hash string, target int) bool`** instead of `Cost` — narrower and more opinionated.
  Rejected as less composable: `Cost` answers the rehash question, supports auditing a store for
  hashes at obsolete factors, and commits the package to no policy of its own.
- **Switching to argon2id** — the better algorithm (memory-hard, GPU/ASIC-resistant, no 72-byte
  limit), and the one the documentation now recommends for new systems. Rejected here as ADR-0024
  already rejected it: the spec froze bcrypt, and argon2id is a new algorithm surface with its own
  parameter-tuning and portability burden, not a drop-in. The documented migration path is how a
  consumer gets there deliberately.

## Consequences

- **API:** `hash` gains `HashPasswordCost`, `Cost`, and `ErrInvalidCost`. Additive; `HashPassword`,
  `CheckPassword`, `ErrMismatch`, and `ErrPasswordTooLong` keep their behaviour exactly — `HashPassword`
  still produces cost 10, now by delegation, asserted by a test that reads the factor back rather than
  by matching a string prefix. No new dependency; the Go 1.24 floor and the x/crypto v0.48.0 pin
  (ADR-0024) are untouched.
- **Control C-4 is extended** from "cost ≥ 10" to "cost ≥ 10 *enforced locally*, because the upstream
  library accepts weaker factors silently", with the weak-cost rejection tests and the pinned-upstream
  test as evidence.
- **The cost knob is a documented trade-off, not a free strengthening.** Verification costs the same as
  hashing at the same factor (measured), and every login pays it on an endpoint an unauthenticated
  caller can reach: at cost 12, roughly 4.5 verifications per second saturate a core. Raising the cost
  hardens a leaked store against offline cracking *and* multiplies the CPU an attacker consumes per
  request. Recorded as a threat-model row whose mitigation is ADR-0031's admission middleware
  (control C-5) — the two Milestone 10 items compose deliberately.
- **Cost drift is named as an operational risk.** The same cost gets cheaper as hardware improves, so a
  fixed configuration silently weakens over time; the documentation prescribes periodic review, and
  `Cost` plus rehash-on-login is the mechanism for moving a store forward without a flag day. Hashes
  belonging to users who never log in again stay at the old factor — stated rather than glossed, since
  a store does not converge on its own.
- **Performance:** unchanged for existing callers. `Cost` is ~112 ns, so checking every stored hash
  against the target at login is free next to the verification that must happen anyway. Full
  measurements, including the exact doubling per step and the ~32-hour extrapolation for cost 31, are in
  [`docs/benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md`](../benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md),
  satisfying spec §7's documented-cost-factor requirement.
- **Testing:** `hash` stays at 100% coverage. The suite deliberately hashes only at costs 10 and 11 —
  the cheapest pair that proves the argument is honoured — because a sweep at higher factors would cost
  the suite seconds per assertion for no additional guarantee. Cost 31 is verified as *inside* the
  accepted range without ever hashing at it, by pairing it with an over-long password so bcrypt refuses
  on length before doing work; that keeps the documented ceiling honest without a multi-hour test.
- **Risk accepted:** a caller can still choose a cost that is too low for their threat model (10 is a
  floor, not a recommendation) or too high for their latency budget. The library's job here is to
  refuse the indefensible, measure the trade-off, and document it; the choice inside the range is the
  deployment's.

## References

- Spec v2.0 item 20 (configurable cost, min accepted 10, argon2id note + migration path), §7 (documented
  cost-factor benchmark); ADR-0030 (why the default-cost bump is deferred while the capability ships).
- ADR-0024 (bcrypt, default cost, self-contained error surface — extended here), ADR-0005
  (loud-by-default), ADR-0018 (`ErrUnsupportedFormat`: the error-over-panic precedent for
  configuration-shaped invalidity), ADR-0031 + control C-5 (admission control as the mitigation).
- `golang.org/x/crypto/bcrypt` v0.48.0 — `GenerateFromPassword`, `Cost`, `MinCost`/`DefaultCost`/`MaxCost`,
  and `newFromPassword`'s silent sub-`MinCost` promotion (`bcrypt.go`).
- OWASP Password Storage Cheat Sheet (bcrypt work-factor floor; argon2id preference for new systems).
- `docs/benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md`; `hash/cost_test.go`,
  `hash/cost_internal_test.go`.
