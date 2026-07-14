# ADR-0017: middleware.Cors design — CorsConfig shape, deny-by-default, loud credential/wildcard guard

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role
- **Related:** spec §2 feature 12, §5 (`Cors(cfg CorsConfig) func(http.Handler) http.Handler`);
  ROADMAP 4.4 (completes Milestone 4); ADR-0013 (middleware foundation — Decorator, loud nil,
  constructor-when-configured shape); ADR-0015 (enterprise posture — why this security-relevant
  decision carries an ADR); threat model (public HTTP edge); compliance control C-3 (new)

## Context

Feature 12 is "robust, configurable CORS header handling", with the constructor signature frozen
at intake — `Cors(cfg CorsConfig) func(http.Handler) http.Handler` — but the **shape of
`CorsConfig` left to design**. CORS is a browser security mechanism: it governs which foreign
origins may read cross-origin responses and send credentials. The design decisions are therefore
security-relevant, which under the enterprise posture (ADR-0015, AGENTS.md §7) requires this ADR:
what the config exposes, what the safe defaults are, how preflight is answered, and — the part
with real teeth — which misconfigurations are refused outright.

Cors is the constructor-configured form of the package's Decorator shape (ADR-0013); it adopts no
new pattern and inherits the loud-nil-handler convention.

## Decision

**`CorsConfig` — six fields, zero value denies all.** `AllowedOrigins`, `AllowedMethods`,
`AllowedHeaders`, `ExposedHeaders []string`; `AllowCredentials bool`; `MaxAge time.Duration`. The
zero value (no allowed origins) denies every cross-origin request — the safe default; a consumer
opts in by naming what it trusts. A plain struct (not functional options) is used because the
spec froze `CorsConfig` as the argument.

**Three request paths.**
- *No `Origin` header* → not a cross-origin request; forward to `next` untouched, no CORS headers.
- *Preflight* (an `OPTIONS` carrying `Access-Control-Request-Method`) → answered **directly with
  `204 No Content`** and the negotiated `Access-Control-*` headers; **not forwarded to `next`**.
  A preflight is CORS machinery, not application traffic; terminating it is the robust default and
  keeps handlers free of OPTIONS noise. A bare `OPTIONS` without the request-method header is a
  real request and *is* forwarded.
- *Actual cross-origin request* → forwarded to `next`; `Access-Control-Allow-Origin` (and the
  credential/expose headers) are added when the origin is allowed and **omitted when it is not**.
  A disallowed origin's request still runs — CORS is enforced by the *browser*, which withholds the
  response from the page when the header is absent; the server does not fail the request.

**Two misconfigurations are refused loudly at construction** (the security teeth, ADR-0005 idiom):
- `AllowCredentials` together with a `"*"` origin **panics** — the Fetch spec forbids returning
  `Access-Control-Allow-Origin: *` with credentials, and silently "fixing" it by reflecting every
  origin would turn credentialed CORS into a whole-web open door. The consumer must name origins.
- A **negative `MaxAge` panics** — a nonsensical value better caught at wiring than emitted.

**Origin handling.** Matching is **exact and case-sensitive** against the configured set;
configure origins as browsers send them. `"*"` allows all. `Access-Control-Allow-Origin` is `"*"`
only in the wildcard-without-credentials case; otherwise the specific origin is **echoed** and
`Vary: Origin` is added so shared caches key on it. (Credentials + wildcard cannot reach this path —
it panicked at construction.)

**Header/method negotiation.** `AllowedMethods` empty defaults to the CORS-safelisted `GET, HEAD,
POST`. `AllowedHeaders` empty or `"*"` **reflects** the browser's `Access-Control-Request-Headers`
verbatim (the ergonomic, widely-used default); an explicit list is sent as-is and a reflected value
is used only when present. `MaxAge` is emitted as whole seconds and **omitted when zero** (browser
default applies). A preflight's `Vary` always lists `Origin, Access-Control-Request-Method,
Access-Control-Request-Headers` so a cache never serves one negotiation to another.

**Loud nil handler** (ADR-0013 lineage): the returned decorator panics on a nil `next`.

## Alternatives Considered

- **Reflect any origin when credentials are enabled** (no explicit list required) — maximally
  convenient, catastrophic security: any site could make credentialed requests. Rejected; require a
  named origin list and panic on credentials+`"*"`.
- **Subdomain / regex / suffix origin patterns** (`*.example.com`) — common in mature CORS libraries
  and genuinely useful, but a wider, error-prone surface (a bad pattern is an open door). Deferred:
  additive to `CorsConfig` later without breaking the signature.
- **Case-insensitive origin matching** — tolerant of misconfiguration, but origins are
  case-sensitive except for the ASCII-lowercased scheme/host browsers already send; fuzzy matching
  invites a mismatch between intent and effect. Rejected for exact match, documented.
- **Forward the preflight to `next` (passthrough)** — lets an app own OPTIONS, but the robust,
  least-surprising default is for the CORS layer to answer its own preflights. Rejected as the
  default; a consumer wanting passthrough can order middleware to handle OPTIONS first.
- **Functional options (`Cors(opts ...Option)`)** — the workerpool/pubsub idiom, but the spec froze
  a `CorsConfig` value. A struct it is; options can wrap later if needed.
- **`200 OK` for preflight** — some servers do; `204 No Content` is the accurate status for a body-less
  preflight answer. Chosen 204.

## Consequences

- The public surface gains `CorsConfig` (six fields) and `Cors`; the zero value is safe (deny-all),
  and the two footgun combinations cannot be constructed silently.
- CORS negotiation is correct and cache-safe (echo + `Vary` for specific origins, `"*"` only without
  credentials, full preflight `Vary`), and handlers never see preflights.
- A new compliance control **C-3** (CORS origin/credential policy) records the credentials+wildcard
  refusal and deny-by-default; the threat model's public-HTTP-edge boundary gains a Cors row
  (Spoofing/Information-disclosure — foreign-origin read access).
- No new pattern: Cors is another **Decorator** (catalogue row 9), constructor-configured per ADR-0013.
- **Milestone 4 (HTTP middleware) is complete** — RequestID, Logger, Recoverer, Cors all landed.
- Deferred surface (subdomain patterns, options-based construction, preflight passthrough) is additive
  and can arrive without a breaking change.

## References

- `docs/specs/01_spec_utils.md` §2.12, §5.
- ADR-0013 (middleware foundation), ADR-0015 (enterprise posture), ADR-0005 (loud-by-default).
- Fetch Standard (CORS protocol) — credentials vs. wildcard origin; preflight; `Vary`.
- `docs/security/threat-model.md` — public HTTP edge, Cors row.
- `docs/compliance/README.md` — control C-3.
