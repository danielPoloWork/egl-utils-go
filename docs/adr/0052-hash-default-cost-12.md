# ADR-0052: bcrypt's default work factor becomes 12 — the module owns its default

- **Status:** Accepted
- **Date:** 2026-07-30
- **Deciders:** Maintainer (Daniel Polo), architect agent, security auditor
- **Related:** [ADR-0024](0024-hash-password-design.md) (the `hash` design — superseded on the
  *value* of the default cost and on nothing else), [ADR-0032](0032-hash-password-cost-design.md)
  (the cost knob and the 10–31 range, both unchanged; its measurements are what made this decision
  arguable), [ADR-0030](0030-spec-v2-reconciliation.md) §2 ledger item 20,
  [ADR-0042](0042-post-1.0-compatibility-contract.md) (why a documented behavioural change is
  major-only), [ADR-0045](0045-pkg-layout-and-v2.md) (the `/v2` boundary), ADR-0031 / control C-5
  (the admission control this default leans on); spec §2 feature 20, §5, §7; control
  [C-4](../compliance/README.md); ROADMAP 13.8

## Context

Ledger item 20 is the last of the seven deferred deltas: **spec v2 specifies a default bcrypt cost of
12** where v1 shipped bcrypt's own default of 10. Its gap column reads
`🟠 default cost · 🟡 configurability · 🟡 doc note`; the two amber items were discharged additively in
10.5 (`HashPasswordCost`, the argon2id note, the cost-sizing benchmark). Only the default was left,
and only because it could not be shipped in a minor.

**Why a constant is a breaking change.** Nothing in the signature moves. What moves is a documented
guarantee about the work a call performs — and 10.5 measured how much. Re-verified today on the same
reference workstation (Intel i5-6600K @ 3.5 GHz, `-benchtime=5x`):

| cost | hash | verify |
|------|------|--------|
| 10 | 57.0 ms | 57.7 ms |
| 12 | 229.9 ms | 232.3 ms |

**×4.03 in both directions.** Verification costs the same as hashing, and verification is the side
every login pays, so this default multiplies the CPU of a login by four. ADR-0042 makes a documented
behavioural contract binding under v1, and spec §5's own versioning clause says the same; a consumer
who upgrades a patch release and finds their login endpoint consuming four times the CPU has been
broken by us, even though their code still compiles. So the ledger held it for a major, and this is
the major.

The change is therefore not "make it stronger" — the capability to be stronger already shipped, and
any deployment that wanted cost 12 has had `HashPasswordCost(pw, 12)` since v1.1.0. The question this
ADR answers is **what a caller who expresses no opinion should get**, which is a different question
with a different answer.

## Decision

**`HashPassword` produces cost 12.** The value lives in an unexported `defaultCost` constant in
`pkg/hash`, and `HashPassword` delegates to `HashPasswordCost(pw, defaultCost)` as it has since 10.5.

**The module now owns its default instead of inheriting one.** v1 wrote
`HashPasswordCost(pw, bcrypt.DefaultCost)`, which read as deference to upstream but was really an
absence of a decision: the number a consumer's password store is protected by was whatever
`golang.org/x/crypto` last chose, changeable by a dependency bump. It is now this module's own
number, defended in this ADR and pinned by a test.

**A default is not a floor, and the gap between them is deliberate.** `minCost` stays at 10, so the
accepted range is unchanged at 10–31 and `HashPasswordCost(pw, 10)` remains legal. A deployment whose
login path genuinely cannot absorb ~230 ms of CPU per attempt can still have v1's factor — but it now
has to write `10` in its own source, which is a decision on the record rather than a default nobody
examined. This asymmetry is the whole mechanism of the change: **the strong value is the one you get
by not choosing, and the weak value is the one you have to ask for.**

**No stored hash is invalidated, and there is no data migration.** bcrypt encodes the factor in the
hash string, so a store written by v1 at cost 10 keeps verifying untouched — pinned by a test against
a *captured* cost-10 hash rather than one the current code produces. Those hashes move to 12 through
the verify-and-rehash-on-login pattern the package documentation already prescribes (ADR-0032's
`Cost` is what makes it actionable), so the upgrade is gradual and no user is locked out.

**The denial-of-service trade-off is now the default posture, so it is documented as such.** At cost
12, roughly 4.3 verifications per second saturate one core, and login is an endpoint an
unauthenticated caller can reach. This is the same trade-off ADR-0032 measured and the threat model
already records — what changes is that a deployment inherits it by default instead of opting into it.
The godoc therefore states the number at `HashPassword` itself, not only in the package overview, and
points at `ratelimit.(*Limiter).Middleware` (control C-5) as the in-module mitigation. Raising a work
factor without admission control in front of it converts an offline-cracking defence into an online
availability risk; saying so at the point of the default is part of the change, not commentary on it.

**`defaultCost` stays unexported.** Exporting it is additive, hence free to defer (the rule 13.2 used
for `Frame.String()`, 13.6 for `WriteTo` and 13.7 for a `WaitForSignals` return value) — and unlike
those, exporting it is the *irreversible* direction, because a published constant is a promise about
a number this ADR expects to move again. See the alternatives.

## Alternatives Considered

- **Export `hash.DefaultCost` so callers can target the library's default in their rehash check.**
  Genuinely useful: the documented migration pattern is `Cost(stored) < target`, and every consumer
  currently hardcodes its own `12`. Rejected for now on two grounds. It is additive, so nothing is
  lost by waiting — ADR-0032 already deferred exactly this, as "Exporting `MinCost` / `MaxCost` /
  `DefaultCost` constants", on surface discipline, and a `/v2` boundary changes nothing about that
  argument since the symbol would be additive in v2.1 too — and the surface stays at 141 identifiers.
  The new argument against is stronger than the old one: coupling a deployment's target to our
  default means a future major silently raises *their* login latency the moment they upgrade — the
  exact surprise this ADR spent a major boundary to avoid. A deployment's target work factor is a
  property of its own CPU budget, so hardcoding it is arguably correct rather than a wart.
- **Raise `minCost` to 12 as well, so cost 10 becomes unreachable.** Maximally safe on paper.
  Rejected: it would invalidate no stored hash but would break every deployment that *deliberately*
  chose 10 after measuring, turning a default into a mandate. Spec v2 says "min accepted 10", so the
  range is specified, not incidental. Worse, a package that refuses the factor a consumer needs
  pushes them to `golang.org/x/crypto/bcrypt` directly, where the silent-promotion footgun
  ADR-0032 exists to guard is waiting.
- **Go straight to 13 or 14, since a major boundary is expensive and this one is open.** Rejected:
  12 is what the spec specifies and what the current industry guidance for bcrypt says, and 14 is
  ~940 ms per verify — roughly one login per second per core. Choosing a stronger-than-specified
  default here would be the same mistake in the other direction, a library inventing a number for
  hardware it cannot see (the reasoning ADR-0025 supplied and ADR-0051 preserved).
- **Keep 10 as the default and rely on documentation to steer deployers to 12.** The status quo, and
  it *is* documented — 10.5 shipped the sizing report and the godoc guidance. Rejected because a
  default is not advice: it is what ships when nobody reads the advice, and item 20 exists because
  the spec judged bcrypt's default too weak to be that.
- **Make the default configurable at the package level** (a `SetDefaultCost` or an env var).
  Rejected on ADR-0025's singleton grounds and ADR-0005's: mutable package state makes the strength
  of the hash store depend on initialisation order, and the per-call parameter already covers the
  need without the ambiguity.
- **Migrate stored hashes eagerly** (rehash at startup, or on read). Impossible, not merely
  undesirable: a stronger hash can only be derived from the plaintext, which is in hand exactly once
  per login. Recorded because it is the first thing a reader asks.

## Consequences

- **Breaking behaviourally, invisible at compile time** — the one shape of break that needs the
  release notes to do the work a compiler usually does. Nothing to edit; what a consumer must do is
  decide whether their login path can absorb ×4 CPU, and if not, pass `10` explicitly. That
  instruction goes in the v2 migration guide (13.10), not only in this ADR.
- **ADR-0024 is superseded on the value of the default and nothing else.** Salting, the constant-time
  verify, `ErrPasswordTooLong` over truncation, the sentinel translation, the
  no-detail-to-the-end-user guidance and the bcrypt-not-argon2id scope all stand. **ADR-0032 is not
  superseded at all** — the range, the local enforcement of it, `ErrInvalidCost`, `Cost`, and the
  measurements are exactly as decided; this change consumes them.
- **Control C-4 tightens**: the compliance claim moves from "cost ≥ 10" to "cost 12 by default, 10
  the accepted floor", with the deliberate default-above-floor gap recorded as the mechanism.
- **Three threat-model rows change** (offline cracking, the work-factor DoS, and the
  weakens-over-time row): the offline-cracking mitigation gets four times stronger by default, the
  DoS row's residual is now inherited rather than opted into, and the periodic-review row now has a
  library default that has actually moved once — evidence the review is real.
- **The test suite pays the ×4 too, and it was cheaper to accept than to hide**: `pkg/hash` goes from
  2.6 s to 6.3 s because the tests exercising `HashPassword` now hash at 12. They were left on the
  public default path deliberately — a suite that hashes at a cost no consumer gets is testing
  something else. Tests that only need *a* hash still ask for cost 10 explicitly, as they did before.
- **No new dependency, no new pattern, no API change. Surface unchanged at 141 identifiers** — the
  second consecutive item `spec_api_lint` could not have caught (13.7 was the first), because here
  neither the identifier nor the signature moved. §5's prose and this ADR carry the semantics.
- **100% statement coverage retained**, with two new tests: a captured v1-era cost-10 hash still
  verifies and still reports 10, and `defaultCost` is inside the accepted range — the latter guarding
  a failure a black-box test would not explain, since a default outside 10–31 would make
  `HashPassword` return `ErrInvalidCost` for *every* password.
- **The next move on this number is already framed**: raising it again is major-only for the same
  reason this was, so the mechanism to reach for meanwhile is the deployment's own
  `HashPasswordCost` plus rehash-on-login, not a library bump.

## References

- `pkg/hash/hash.go`, `pkg/hash/cost_test.go`, `pkg/hash/cost_internal_test.go`,
  `pkg/hash/hash_test.go`.
- [Cost-sizing report](../benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md) — 10.5's measurements,
  extended with today's re-verification of the 10 → 12 step.
- ADR-0030 §2 — the ledger, item 20 discharged (the last of the seven);
  `docs/specs/02_spec_v2_gap_analysis.md` row 20.
- Control [C-4](../compliance/README.md); [threat model](../security/threat-model.md) — credential
  store rows.
