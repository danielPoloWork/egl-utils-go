# ADR-0044: Use a canonical header key for map access, keeping HeaderName's value

- **Status:** Accepted
- **Date:** 2026-07-27
- **Deciders:** Maintainer (Daniel Polo), with the architect agent
- **Related:** ADR-0013 (RequestID design), ADR-0037 (NFR methodology), ADR-0042 (post-1.0 compatibility), ROADMAP 10.10

## Context

Milestone 10.10's NFR suite measured `middleware.RequestID` at 6 allocations per request and
attributed two of them to a single cause: `middleware.HeaderName` is `"X-Request-ID"`, which is not
Go's canonical header form. `http.Header`'s `Get` and `Set` both run
`textproto.CanonicalMIMEHeaderKey` on the key they are given, and that function allocates a fresh
string whenever the key is not already canonical — Go canonicalises each dash-separated token to
title case, giving `"X-Request-Id"`. Two allocations per request, buying nothing: the wire format is
identical either way, because `net/http` canonicalises header names when writing.

10.10 recorded the finding and did not act on it, on the reasoning that `HeaderName` is an exported
constant whose value is part of the public surface — under what is now [ADR-0042](0042-post-1.0-compatibility-contract.md),
changing a documented value is a MAJOR-only event. The item was carried forward in the ROADMAP, in
the v1.1.0 release notes, and in the NFR report as *"measured but not changed"*, waiting on a `/v2`
that ADR-0041 and ADR-0042 both describe as unscheduled.

That reasoning conflated two separate things: **the value of the constant, and the cost of using it
as a map key.** Only the first is a compatibility question.

## Decision

`RequestID` performs its header map access through an unexported
`canonicalHeaderName = "X-Request-Id"`, while the exported `HeaderName` keeps its documented value
`"X-Request-ID"`. The allocations disappear and the public surface does not move, so this ships as a
PATCH (v1.1.1) rather than waiting for a major.

Nothing observable changes, and the reason is that `http.Header.Set` already canonicalises before
storing: **the header map has been keyed by `"X-Request-Id"` all along**, on the wire and in memory.
A consumer calling `Get(middleware.HeaderName)` is unaffected — their `Get` canonicalises their key
too — and one reaching into the raw map would have needed the canonical spelling before this change
as much as after it.

## Alternatives Considered

- **Change `HeaderName` to `"X-Request-Id"`.** The obvious reading of 10.10's finding, and what the
  ROADMAP assumed. Rejected: it buys the same two allocations at the price of a MAJOR release, since
  ADR-0042 counts a documented value as breaking surface. Having established that the ledger is the
  only destination for a breaking change, spending a major on a spelling would be a poor first
  withdrawal. The rename remains available at a `/v2` boundary if one ever opens, where it costs
  nothing extra — and by then it is cosmetic, because the performance argument is already spent.
- **Leave it, as 10.10 decided.** Rejected once the separability was noticed: two allocations per
  request on the most widely used middleware in the module, indefinitely, in exchange for nothing.
- **Deprecate `HeaderName` and add a canonically-spelled exported constant.** Additive and therefore
  MINOR-legal, but it puts two exported constants with the same meaning in the public surface
  forever, and consumers gain nothing by switching — the allocation was never theirs to pay. It
  solves a naming preference nobody raised at the cost of permanent API clutter.

## Consequences

- **Measured, hardware-independent:** `RequestID` 6 → **4** allocations per request,
  `Chain` 8 → **6**, `ChainWithLogger` 11 → **9**, `RequestIDGenerated` 7 → **5**. Bytes per
  operation fall with them (`RequestID` 432 → 400 B). The isolated mechanism benchmarks at 3
  allocs/369.9 ns for a non-canonical `Set`+`Get` pair against 1 alloc/198.2 ns for the canonical
  one.
- **The NFR-01 ratchet is lowered to the new floor** (`allocBudget`: RequestID 4, Chain 6,
  ChainWithLogger 9) and verified to bind there — lowering RequestID to 3 fails the gate. A ratchet
  that is not tightened after an improvement silently re-admits the regression it just removed.
- **Latency improved but is not claimed as a number.** The chain measures ~675 ns against 938 ns
  before, consistent with removing two allocations, but ADR-0037's methodology reports rather than
  gates hardware-dependent timings and this workstation's spread is wide (`ChainWithLogger` ranged
  3.2–15.7 µs across three runs). The allocation counts carry the claim; the nanoseconds are
  context.
- **Three documents carried this as blocked and no longer should:** ROADMAP 10.10 and the Milestone
  10 preamble, and `docs/benchmarks/2026-07-26-nfr-suite.md`. Corrected in the same PR. The v1.1.0
  release notes keep the original wording — they describe what shipped in that release and are not
  rewritten.
- **`HeaderName` remains exported and unchanged**, so spec §5 and `tools/spec_api_lint.py` are
  unaffected, and no `/v2` obligation is created or discharged.
- **The generalisable point, which is why this is an ADR and not a comment:** a finding recorded as
  "blocked behind a breaking change" deserves re-reading before it is inherited. This one was
  carried through three documents and two releases on an attribution that was half right — the cost
  was real, the blocker was not.

## References

- ADR-0013 — the RequestID design that introduced `HeaderName`.
- ADR-0037 — NFR methodology; NFR-01's ratchet budget and the original finding.
- ADR-0042 — why changing an exported constant's value would be MAJOR-only.
- `docs/benchmarks/2026-07-26-nfr-suite.md` — NFR-01 figures, updated in place with the after state.
- Go: `net/textproto.CanonicalMIMEHeaderKey`, `net/http.Header.Get`/`Set`.
