# ADR-0047: cache.Get returns (V, bool) — a miss is an outcome, not an error

- **Status:** Accepted
- **Date:** 2026-07-28
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** ADR-0021 (`Get` signature and constructor name superseded; its expiry model
  preserved), ADR-0030 §2 ledger item 17, ADR-0038 (sharding), ADR-0045 (`/v2` boundary),
  spec §2 feature 17 / §5; ROADMAP 13.3

## Context

ADR-0021 gave `Get` the signature `(V, error)` and a package-level `ErrNotFound` sentinel, so a
caller wrote:

```go
v, err := c.Get(k)
if err == nil {
        use(v)
} else if errors.Is(err, cache.ErrNotFound) {
        // ... the only possible error
}
```

`ErrNotFound` was the *only* error `Get` could ever return, which is the tell: the error channel
carried one bit of information. Go spells that bit comma-ok — `map` lookups, type assertions and
channel receives all do — and item 17 of the ADR-0030 ledger put the change on the `/v2` list.

The constructor was `NewInMemory`, from a time when a second cache kind seemed likely. None arrived,
so the qualifier distinguishes nothing while the package name already says `cache`.

## Decision

**`Get(key K) (V, bool)`.** True means the value is live and usable; false means the zero `V` and
nothing to use.

```go
if v, ok := c.Get(key); ok {
        use(v)
}
```

**A present-but-expired entry reads as absent.** This is where ADR-0021 does load-bearing work and is
preserved deliberately rather than inherited by accident: expiry is judged against the entry's
deadline *at call time*, never against the sweeper's schedule, so there is no window in which `Get`
could return `true` for an entry the caller must not use. Had expiry depended on the sweeper having
run, the boolean would have been a weaker promise than the error was.

**Absence and expiry stay indistinguishable.** They were already one outcome under `ErrNotFound`, so
the bool loses nothing. Separating them (a second bool, an enum) was rejected: it would promise
callers something about *when* eviction happens, which is exactly the implementation freedom ADR-0021
and ADR-0038 rely on — the sweeper's schedule and the per-shard sweep order are not part of the
contract.

**`NewInMemory` becomes `New`.** `cache.New(ttl, opts...)` reads without stutter and matches what
spec v2 asked for. It is a rename, not an alias: two constructors for one type is the kind of
duplication a major exists to remove, not to introduce.

**`ErrNotFound` is removed.** With `Get` returning a bool, nothing in the package produces it. An
exported sentinel no code path returns is worse than an absent one: `errors.Is(err, cache.ErrNotFound)`
would still compile and simply never be true, so a caller who missed the migration gets silence
instead of a compiler error.

## Alternatives Considered

- **Keep `ErrNotFound` exported as a migration landmark.** Rejected: see above — it converts a
  compile-time break into a silent runtime falsehood. A removed identifier fails loudly, which is
  what a major should do.
- **Add `New` as an alias and keep `NewInMemory`.** The literal reading of the ROADMAP line. Rejected:
  it permanently doubles the constructor surface, and every exported name is a v2 SemVer promise.
- **Add `New`, mark `NewInMemory` deprecated.** Softest migration, but a deprecation shipped *in*
  v2.0.0 has no removal window short of v3 — in practice the "keep both" option with a comment.
- **`Get` returning `(V, bool, error)` or a richer result type.** Rejected: there is no second failure
  mode to report. An in-memory map lookup cannot fail for any reason other than the key not being
  usable, and inventing a channel for errors that cannot occur is how `(V, error)` got here.
- **Distinguishing "expired" from "absent".** Rejected as a contract question, not an ergonomics one
  — see Decision.

## Consequences

- **Breaking, three ways**, all mechanical at the call site: `Get`'s second return is `bool` not
  `error`; `NewInMemory` is `New`; `ErrNotFound` is gone. Migration:
  `v, err := c.Get(k); if err == nil` → `v, ok := c.Get(k); if ok`.
- **The new invariant worth a test of its own: a stored zero value is not a miss.** Under `(V, error)`
  this was implicit; under comma-ok it is the reason the signature exists. `Set(k, 0)` followed by
  `Get(k)` must return `(0, true)`, or every caller caching zeros, empty strings or nil slices
  silently re-fetches forever. Pinned by `TestStoredZeroValueIsNotAMiss`.
- **No behavioural change whatsoever.** The condition deciding presence is byte-for-byte the one
  ADR-0021 shipped; only its reporting changed. Sharding, the single sweeper, `Close`'s idempotence
  and the usable-after-Close posture are all untouched.
- **No performance claim, and no benchmark report.** Returning a bool instead of a pre-allocated
  sentinel error cannot plausibly move a measurement, and the existing suite already covers the hot
  paths. Asserting a win here would be manufacturing one.
- **100% statement coverage retained.** No dependency change, no new internal edge, no new pattern —
  `docs/patterns/README.md` is untouched.
- **`errors` is no longer imported by the package**, a small tidiness the removal of the sentinel
  bought.

## References

- `pkg/cache/cache.go`, `pkg/cache/cache_test.go` (incl. `TestStoredZeroValueIsNotAMiss`),
  `pkg/cache/cache_internal_test.go`, `pkg/cache/lifecycle_test.go`.
- ADR-0021 — the design this amends, and whose Get-enforced expiry the boolean's contract rests on.
- ADR-0030 §2 — the ledger, one further entry discharged; ADR-0045 — the `/v2` boundary.
