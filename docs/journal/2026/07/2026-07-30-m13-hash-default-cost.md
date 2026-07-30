# 2026-07-30 — 13.8: a constant is a contract, and the default is the decision

**`hash.HashPassword` now produces bcrypt cost 12.** ADR-0052 supersedes the *value* of ADR-0024's
default cost and nothing else; ADR-0032 is not superseded at all — its 10–31 range, local enforcement,
`ErrInvalidCost` and `Cost` are consumed exactly as decided. **Ledger item 20 discharged — the last of
the seven.** The ADR-0030 §2 ledger is now empty; 13.10 marks it so.

## Why this was major-only, when nothing in the surface moves

No identifier changes. No signature changes. Nothing anywhere fails to compile. That is precisely why
it needed a major: **the constant is the contract.** Verification costs the same as hashing, and
verification is the side every login pays. Re-measured on the same workstation as 10.5 before the
switch was flipped:

| cost | hash | verify |
|------|------|--------|
| 10 | 57.0 ms | 57.7 ms |
| 12 | 229.9 ms | 232.3 ms |

**×4.03 in both directions**, and the doubling is still exact (2.00–2.03 per step). A consumer who
picked this up in a minor would have found their login endpoint consuming four times the CPU with
their own code untouched — broken by us, silently, with no compiler to say so. ADR-0042 makes exactly
that binding under v1, so the ledger held it.

Worth naming as a class: this is the **second consecutive item `spec_api_lint` could not have
caught** (13.7 was the first, where the signature moved but the identifier did not). Here *neither*
moved. The gate is identifier-level; §5's prose and the ADR are the only records of the semantics, and
that is now twice in a row, not a one-off.

## The item was never about the capability

`HashPasswordCost(pw, 12)` has shipped since v1.1.0. Any deployment that wanted cost 12 has had it for
a month. So the item is not "make it stronger" — it is **what a caller who expresses no opinion gets**,
which is a different question and has a different answer.

The mechanism is the gap between the default and the floor. `minCost` stays at 10, the accepted range
is unchanged, and `HashPasswordCost(pw, 10)` is still legal. **The strong value is what you get by not
choosing; the weak value is the one you have to ask for.** A deployment whose login path genuinely
cannot absorb ~230 ms per attempt still has v1's factor available — but it now writes `10` in its own
source, which is a decision on the record instead of a default nobody examined.

Two symmetrical temptations were both rejected:

- **Raise `minCost` to 12 too**, making cost 10 unreachable. It looks safer and is worse: it turns a
  default into a mandate against deployments that measured and chose 10, spec v2 specifies "min
  accepted 10" so the range is deliberate, and a package that refuses the factor a consumer needs
  pushes them to `x/crypto/bcrypt` directly — where ADR-0032's silent-promotion footgun is waiting.
- **Go to 13 or 14 while the boundary is open.** The same mistake mirrored: a library inventing a
  number for hardware it cannot see, which is ADR-0025's reasoning that ADR-0051 preserved a day ago.
  12 is what the spec specifies; 14 is ~940 ms per verify, about one login per second per core.

## The module stops inheriting its most security-relevant constant

v1 wrote `HashPasswordCost(pw, bcrypt.DefaultCost)`. That reads as deference to upstream, and it is
really **an absent decision**: the work factor protecting a consumer's password store was whatever
`golang.org/x/crypto` last chose, movable by a Dependabot PR. It is now this module's own `defaultCost`
constant, argued in an ADR and pinned by a test.

That reframing is what the internal test had to change to say. It used to assert
`bcrypt.DefaultCost >= minCost` on the grounds that `HashPassword` delegated with it — an assertion
that no longer describes anything. `bcrypt.DefaultCost == 10` is still pinned, but now as *the upstream
value ours deliberately exceeds* and as the value sub-`MinCost` costs are silently promoted to. The
property that assertion used to protect got its own test instead: **a `defaultCost` outside 10–31 would
make `HashPassword` return `ErrInvalidCost` for every password** — a total outage of the hashing path
that no black-box test of the default's *value* would explain.

## No migration exists to run, and the test proves it with a hash it did not make

bcrypt encodes the factor in the hash string, so v1's cost-10 hashes keep verifying untouched and move
to 12 through the verify-and-rehash-on-login pattern the godoc already prescribed (10.5's `Cost` is
what makes it actionable). The obvious way to test that is to hash at 10 and verify it — which proves
nothing about *old* hashes, only that the current code round-trips. The test uses a **captured
literal** cost-10 hash instead, so the assertion is about a string this package did not just produce.

Eager migration is not merely undesirable, it is impossible: a stronger hash can only be derived from
the plaintext, and the plaintext is in hand exactly once per login. Recorded in the ADR because it is
the first thing a reader asks.

## The DoS trade-off is now inherited rather than opted into

At cost 12, roughly **4.3 verifications per second saturate one core**, on an endpoint an
unauthenticated caller can reach. This is the trade-off 10.5 measured and the threat model already
carried; what changed is who chooses it. A deployment that reads nothing now gets it.

So the godoc states the number **at `HashPassword` itself**, not only in the package overview, and
names `ratelimit.(*Limiter).Middleware` (control C-5) there. Raising a work factor without admission
control in front of it converts an offline-cracking defence into an availability risk, and saying so at
the point of the default is part of the change rather than commentary on it. The threat model's DoS row
now records the residual as inherited, and its weakens-over-time row can finally point at a library
default that has **actually moved once** — evidence the prescribed periodic review is a practice and
not an intention.

## `DefaultCost` stayed unexported, and the reason is not the usual one

The usual reason applies too — exporting is additive, hence free to defer (13.2's `Frame.String()`,
13.6's `WriteTo`, 13.7's return value), and ADR-0032 had already deferred exactly this symbol on
surface discipline. But there is a better one here, and it runs the other way from every previous
deferral: **omission is the reversible direction.** A published `hash.DefaultCost` is a promise about a
number this very ADR expects to move again, and a consumer who targets it in their rehash check would
have their login latency raised for them by a future major — the exact surprise this item spent a major
boundary to avoid. A deployment's target work factor is a property of its own CPU budget, so
hardcoding it is arguably correct rather than a wart.

## Small things, recorded because they were decided rather than defaulted

The suite's own cost went **2.6 s → 6.3 s**, because the tests that exercise `HashPassword` now hash at
12. They were deliberately **left on the public default path**: a suite that hashes at a factor no
consumer gets is testing something else. Tests that merely need *a* hash still ask for cost 10
explicitly, as they did before 13.8, so the ×4 is paid only where the default is the subject.

The 10.5 sizing report was **updated in place** (13.6's precedent) with a dated re-verification
section rather than superseded by a second report, so the before and after of the number that drove
the decision live in one document. Its `Reproduce` block also still said `./hash/`, a path 13.1 moved;
fixed while there.

No new dependency, no new pattern, no API change, surface unchanged at **141 identifiers**. Coverage
stayed 100% with two new tests.

## State

Milestone 13 is **8 of 10**, and **the ADR-0030 §2 ledger is empty** — seven items deferred at v1.0.0,
seven discharged in one major, which was the whole argument for opening the boundary once rather than
twice.

Next is **13.9 (contrib → `/v2`)**, which cannot run yet: ADR-0040 forbids a `replace` and requires
the *released* core, so it comes **after the `v2.0.0` tag**. In practice that means **13.10 (the
release) runs first** — `version.go` → 2.0.0, the changelog rolled, and the migration guide, which
carries both the mechanical import table *and* this item's behavioural note, since nothing in 13.8
fails to compile and the release notes are the only thing that will tell a consumer their logins got
four times more expensive.

**Still carried, still not fixed:** `orchestrator/project.yaml` describes the v1 API. 13.8 does not add
a line to that drift — the file never named the cost — but the sweep is still outstanding, and the
release is the natural moment to take it.
