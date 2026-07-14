# Threat model — egl-utils-go

> **Owner:** the **security-auditor** role (it drafts here; findings feed the audit risk
> register). Produced and kept current by the **audit threat-modeling sub-mode**
> (`/eados security` → `/eados audit`). Method: **STRIDE**. Scaffolded empty on purpose —
> an explicit `n/a` with a reason is honest; an unexamined boundary is not.

## 1. Scope & trust boundaries

List every boundary an attacker could stand on either side of — network edges, process/privilege
boundaries, tenancy separation, third-party services — and for each: the **untrusted inputs**
that cross it, and the **assumptions** the design makes about it.

| Boundary | Untrusted inputs crossing it | Assumptions |
|---|---|---|
| **Public HTTP edge** (a consuming service's inbound requests, guarded by this module's `middleware`) | request headers (`X-Request-ID`, `Origin`, `Access-Control-Request-*`), method, path (logged by 4.2), body | TLS terminated upstream; this module is a library composed into the consumer's handler chain, not a standalone server; the consumer owns authn/authz and body-size limits |

_First populated by ROADMAP 4.1 (`middleware.RequestID`, ADR-0013); Milestone 4 completed the
HTTP-edge middleware (Logger 4.2, Recoverer 4.3, Cors 4.4). Boundaries and inputs grow as the
diagnostics surface (M9) lands; each such PR extends the rows below rather than starting fresh._

## 2. STRIDE pass

Work the six categories (**S**-**T**-**R**-**I**-**D**-**E**) **per boundary/component** above.
Every cell gets an entry — a threat, a mitigation, or an explicit `n/a (reason)`; never a blank.

| Category | Threat considered | Boundary / component | Mitigation / control | Status |
|---|---|---|---|---|
| Spoofing — is the caller who it claims? | Caller sets `X-Request-ID` to impersonate another request's trail, or a consumer mistakes the ID for identity | HTTP edge / `middleware.RequestID` | The ID is documented (godoc + ADR-0013) as a **correlation token only, never for authn/authz**; it grants no authority, so spoofing it achieves nothing | ☑ |
| Tampering — can data/code be altered in flight or at rest? | Attacker injects CR/LF or control bytes into `X-Request-ID` to forge log lines or split response headers when the ID is echoed/logged | HTTP edge / `middleware.RequestID` | Inbound ID accepted only if visible-ASCII (`0x21–0x7e`) and ≤128 bytes; otherwise regenerated. CR/LF/NUL can never reach logs or the reflected header (ADR-0013, control C-2) | ☑ |
| Repudiation — can an action be denied for lack of a trail? | A request has no correlatable identifier, so its handling cannot be traced | HTTP edge / `middleware.RequestID` | Every request carries an ID — adopted if valid, else generated — propagated in context for downstream logging (4.2) | ☑ |
| Information disclosure — can data leak across a boundary? | A generated ID leaks server state (time, sequence, host) or is predictable | HTTP edge / `middleware.RequestID` | Generated with `crypto/rand.Text` (≥128 bits, RFC 4648 base32); derived from no server state and unguessable (ADR-0013) | ☑ |
| Information disclosure — can data leak across a boundary? | Request logging leaks secrets carried in the URL query string (tokens, API keys, signed URLs), or in headers/body, into log stores that outlive and out-scope the request | HTTP edge / `middleware.Logger` | Logger records `r.URL.Path` only — never the query string, headers, or body; the logged fields are a fixed, secret-free set (method, path, status, duration, bytes, `request_id`). A consumer needing more logs it themselves, owning that disclosure (ADR-0014, control C-2) | ☑ |
| Information disclosure — can data leak across a boundary? | A panicking handler leaks a stack trace (source paths, symbols, internal structure) or the panic value (possibly secrets in flight) into the HTTP response | HTTP edge / `middleware.Recoverer` | Recoverer writes only a generic `500 Internal Server Error` to the client; the panic value and stack are logged server-side (`slog.Default`, Error) and never written to the response. Its log line is path-only, like Logger (ADR-0016, control C-2) | ☑ |
| Spoofing / Information disclosure — can a foreign origin read cross-origin responses or send credentials it should not? | An over-broad CORS policy lets any website read authenticated responses or make credentialed cross-origin requests (e.g. `Access-Control-Allow-Origin: *` with credentials, or reflecting every `Origin`) | HTTP edge / `middleware.Cors` | Deny-by-default (`CorsConfig` zero value allows no origin); a specific allowed origin is echoed with `Vary: Origin`, `*` is emitted only without credentials, and the Fetch-forbidden credentials+`*` combination is refused at construction (panic). Subdomain/regex origin patterns are deliberately not offered (ADR-0017, control C-3) | ☑ |
| Denial of service — can the surface be exhausted? | Oversized or high-volume `X-Request-ID` headers inflate memory and log storage; an unrecovered handler panic drops the connection and can unwind past the handler | HTTP edge / `middleware.RequestID`, `middleware.Recoverer` | 128-byte cap on the adopted ID bounds per-request cost (RequestID); Recoverer contains a handler panic and turns it into a bounded 500 rather than a dropped connection or a crash. Broader request/body-size and rate limits remain the consumer's and `ratelimit`'s concern | ◑ partial (per-request panics contained and IDs bounded; whole-request/body-size DoS deferred to consumer) |
| Elevation of privilege — can a caller gain authority it was not granted? | Caller uses the request ID to gain access | HTTP edge / `middleware.RequestID` | n/a — the ID confers no authority by construction; RequestID makes no authorization decision | ☑ |

## 3. Findings → the risk register

A threat that survives analysis lands in the audit **risk register** with its severity
(low/medium/high/critical), affected component, realistic impact, and a concrete mitigation — the
same record shape the audit phase emits. A confirmed, reproducible defect additionally becomes a
[bug-ledger](../bugs/README.md) record; a vulnerability needing coordinated disclosure becomes a
**draft** advisory the human publishes.
