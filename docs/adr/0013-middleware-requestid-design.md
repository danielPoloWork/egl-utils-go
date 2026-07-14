# ADR-0013: HTTP middleware foundation — Decorator chain and RequestID design

- **Status:** Accepted
- **Date:** 2026-07-14
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role
- **Related:** spec §2 features 9–12, §4 (decorator chain), §5 (middleware API); ROADMAP
  4.1; ADR-0005 (loud-by-default idiom, functional options); patterns catalogue
  (Decorator); threat model (public HTTP edge); compliance control C-2

## Context

Milestone 4 opens the HTTP middleware surface with `middleware.RequestID`, the first of
four decorators (RequestID, Logger, Recoverer, Cors). Three things are decided here for
the whole package, plus the RequestID-specific design.

The cross-cutting decisions: the **middleware shape** (spec §4 mandates the standard
`func(http.Handler) http.Handler` decorator chain; this is the first adoption of the
Decorator pattern, which §8 requires an ADR to record), how **values cross the chain**
(context keys, and the collision hazard of raw-typed keys), and the **loud-vs-quiet**
posture for a nil handler.

The RequestID-specific decisions are security-relevant, which under the enterprise
posture (AGENTS.md §7) **requires** this ADR: RequestID reads a client-supplied header,
so it introduces the module's **first untrusted input** and a new trust boundary (the
public HTTP edge — now recorded in the threat model). The open questions: which header,
whether to trust an inbound value, how to generate one, and whether to echo it back.

## Decision

**Decorator, with two shapes matching the spec.** Middleware are
`func(http.Handler) http.Handler`. Where a middleware needs no configuration it *is* that
function (`RequestID(next) http.Handler`, `Recoverer(next) http.Handler`); where it is
configured it is a constructor returning that function (`Logger(l) func(...)`,
`Cors(cfg) func(...)`). Both forms compose by ordinary application and interoperate with
any router speaking the same shape. This split is not ours to choose — it is the frozen
public interface in spec §5 — but it is recorded here as the package convention every
later middleware follows.

**Context values under unexported key types.** The resolved request ID is stored with
`context.WithValue` under an unexported zero-size key type (`requestIDKey struct{}`) and
read back through the exported `RequestIDFrom(ctx) string`. An unexported key type cannot
collide with a consumer's or another library's context keys — the standard Go idiom, and
the package convention for every value later middleware propagate.

**Loud on a nil handler.** `RequestID(nil)` panics immediately (ADR-0005's
programming-error idiom) rather than deferring a nil-pointer dereference to the first
request, when it would be far from the wiring bug that caused it.

**RequestID adopts-or-generates, over a sanitized boundary:**

- *Header:* `X-Request-ID` (the de-facto standard), exported as `HeaderName` so consumers
  can align.
- *Adopt an inbound ID only if it is safe.* An inbound value is accepted verbatim only
  when it is non-empty, at most **128 bytes**, and every byte is a visible ASCII character
  (`0x21`–`0x7e`). This excludes control characters — CR and LF above all, which would
  otherwise enable **log- and header-injection** once the ID is echoed in the response or
  written to logs by the Logger middleware (4.2) — and spaces, while still accepting every
  common ID alphabet (UUID, ULID, hex, base64/base64url). The length cap bounds the memory
  and log volume a hostile header can force. A value that fails is **replaced by a
  generated one, not rejected**: a malformed header must never fail the request, only lose
  its (untrustworthy) correlation hint.
- *Generate with `crypto/rand.Text` (Go 1.24).* When no valid inbound ID exists, generate
  one: ≥128 bits over the RFC 4648 base32 alphabet — unguessable, collision-resistant,
  dependency-free, and free of any weak-RNG (gosec G404) concern. Predictable IDs would
  let an attacker forge or collide correlation trails; `crypto/rand` closes that at no
  cost, and the value leaks no server state.
- *Echo the resolved ID* in the response `X-Request-ID` header so callers and proxies can
  correlate. Reflection is safe precisely because the value is always sanitized or freshly
  generated.
- *Not an identity.* The ID is a correlation token derived from untrusted input; the
  godoc and this ADR state it must never be used for authentication or authorization
  (recorded as the Spoofing mitigation in the threat model).

## Alternatives Considered

- **Reject a malformed inbound ID with 400** — surfaces client error, but a correlation
  header is optional metadata; failing the request over it is disproportionate and hands a
  trivial denial vector to anyone who can set a header. Rejected: regenerate instead.
- **Trust the inbound header verbatim (no sanitization)** — matches some minimal
  middleware, but propagating attacker-controlled bytes into logs and response headers is
  the log/header-injection vulnerability this ADR exists to prevent. Rejected.
- **Generate with `math/rand`/v2 or a counter** — cheaper, but predictable IDs are
  forgeable and collision-prone, and `math/rand` trips gosec G404 in production code.
  Rejected for `crypto/rand.Text`.
- **A third-party UUID library (e.g. `google/uuid`)** — ergonomic, but adds a runtime
  dependency outside ADR-0004's budget for a need the stdlib now covers (`crypto/rand.Text`
  landed in the 1.24 floor). Rejected.
- **Exported context-key variable** — lets consumers read the raw value, but invites
  collisions and freezes the key's representation into the public API. Rejected for the
  unexported-key + accessor idiom.
- **Not echoing the ID in the response** — simpler, but forfeits client/proxy correlation
  for no safety gain once the value is sanitized. Rejected.

## Consequences

- The middleware package convention is set: decorator shape, unexported context keys with
  exported accessors, loud nil-handler panics. Logger, Recoverer, and Cors (4.2–4.4)
  inherit it.
- The public HTTP edge is now a documented trust boundary (threat model §1) with a STRIDE
  pass scoped to RequestID; later middleware extend that pass rather than starting it.
- Compliance control **C-2** (untrusted HTTP input handling) is registered, its evidence
  the sanitizer and its tests plus the threat model.
- Every request carries a correlation ID downstream in context — the substrate 4.2
  Logger will log and 9.x diagnostics will trace.
- Catalogued as **Decorator** (in-taxonomy, Structural), first use in the module and the
  shared shape of all of Milestone 4.
- The 128-byte cap and visible-ASCII rule are a deliberate, documented policy, not a
  standard; a consumer needing a different alphabet must wrap or precede this middleware.

## References

- `docs/specs/01_spec_utils.md` §2 (features 9–12), §4, §5.
- ADR-0004 (runtime dependency policy — why no UUID dep), ADR-0005 (loud-by-default).
- `docs/patterns/design-patterns.md` — Decorator (Structural).
- `docs/security/threat-model.md` — public HTTP edge boundary and STRIDE pass.
- `docs/compliance/README.md` — control C-2.
- `crypto/rand.Text` (Go 1.24) — request ID generation.
