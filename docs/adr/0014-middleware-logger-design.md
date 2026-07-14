# ADR-0014: middleware.Logger design — ResponseWriter capture, status-derived levels, path-only logging

- **Status:** Accepted
- **Date:** 2026-07-14
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role
- **Related:** spec §2 feature 10, §5 (Logger API); ROADMAP 4.2; ADR-0013 (middleware
  foundation — Decorator, context keys, loud nil); ADR-0005 (loud-by-default); threat
  model (public HTTP edge, Information disclosure); compliance control C-2

## Context

Feature 10 is "HTTP request logging with response-time and bytes-written statistics", API
frozen at intake: `Logger(l *slog.Logger) func(http.Handler) http.Handler` — "logs method,
path, status, duration, bytes". It inherits the package conventions from ADR-0013 (the
decorator shape, loud nil handling), so the open decisions are specific to logging: how to
observe the status and byte count a handler produces without breaking the
`http.ResponseWriter` contract; what level each line gets; what is logged and — the
security-relevant part — what is deliberately *not*; and how a panicking request is
handled.

## Decision

**Capture status and bytes with a wrapping `responseRecorder` that implements `Unwrap`.**
The middleware passes the handler a `*responseRecorder` embedding the real
`http.ResponseWriter`; it records the first status code (`WriteHeader`) and accumulates
the bytes returned by `Write`, defaulting the status to 200 (what net/http sends when a
handler writes a body or returns without an explicit `WriteHeader`). Wrapping a
`ResponseWriter` normally hides the optional interfaces the concrete writer implements
(`http.Flusher`, `http.Hijacker`, `io.ReaderFrom`, `http.Pusher`). Rather than
re-declaring each — brittle and incomplete — the recorder exposes `Unwrap()
http.ResponseWriter`, which `http.ResponseController` (Go 1.20+) follows to reach the
underlying writer. Our 1.24 floor makes this the idiomatic, complete solution: a
downstream handler calling `http.NewResponseController(w).Flush()` still works.

**Level follows status:** 5xx → `Error`, 4xx → `Warn`, else `Info`. A flat level buries
server errors among healthy traffic in aggregation; deriving it makes the signal an
operator wants queryable for free, and any consumer who wants a flat level controls it
through their `slog.Handler`.

**Log the path, never the query string.** Query parameters routinely carry secrets
(tokens, API keys, signed URLs); logging them is a classic information-disclosure leak
into log stores that outlive and out-scope the request. Logger records `r.URL.Path` only.
Likewise it logs no headers and no body — only the five stated fields plus `request_id`.
This is a security-relevant decision and the reason this ADR exists under the enterprise
posture (AGENTS.md §7); it is recorded as an Information-disclosure mitigation in the
threat model and as evidence for control C-2.

**Attach `request_id` when present.** If the request context carries a RequestID
(ADR-0013), its value is logged as `request_id`, correlating this line with the request's
trail; when absent, the attribute is simply omitted (no empty field).

**Log from a deferred call — panic-safe.** The line is emitted in a `defer`, so a request
whose handler panics is logged before the panic propagates. The godoc and this ADR
recommend composing Logger *outside* Recoverer (`Logger(Recoverer(h))`), so the recovered
500 is the status Logger observes; composed the other way, a panic is logged with the
default 200 and then unwinds past Logger.

**Loud nils** (ADR-0013 lineage): `Logger(nil)` panics; the returned decorator panics on a
nil handler.

## Alternatives Considered

- **Re-implement each optional interface on the wrapper** — the pre-1.20 approach; verbose,
  and any interface missed (or added to net/http later) is silently dropped. Rejected for
  `Unwrap` + `ResponseController`.
- **A fixed Info level** — simplest and most predictable, but forfeits the single most
  useful query (show me the 5xx) at the source. Rejected; status-derived leveling is the
  enterprise-sensible default and remains overridable at the handler.
- **Log the full `RequestURI`/query for debuggability** — convenient, but turns the log
  into a secret sink. Rejected outright; a consumer who truly needs query logging opts in
  by wrapping, accepting the risk explicitly.
- **Return a richer log event / take an option struct (fields, level func, clock)** —
  more flexible, but widens a surface the spec froze to one constructor argument. Deferred;
  additive options can arrive later without breaking `Logger(l)`.
- **Log before *and* after the request** — a start line doubles log volume for little gain;
  the single post-request line carries the duration that a start line lacks. Rejected.

## Consequences

- Status and byte counts are exact and the `ResponseWriter` contract is intact: flushing,
  hijacking (WebSockets/SSE), and `ReadFrom` still work through the recorder.
- Every request yields exactly one structured line, at a level that segregates errors,
  carrying `request_id` when the chain seeded one — the substrate `logger.Structured`
  (M6) ships to ElasticSearch/Loki.
- Logs are free of query strings, headers, and bodies by construction; a consumer needing
  more logs it themselves, owning that disclosure decision.
- The recommended chain order (`RequestID → Logger → Recoverer → handler`) is documented;
  the wrong order degrades the logged status on panic but is not unsafe.
- No new pattern: Logger is another **Decorator** (catalogue row 9), the shape ADR-0013
  established.

## References

- `docs/specs/01_spec_utils.md` §2.10, §5.
- ADR-0013 (middleware foundation), ADR-0005 (loud-by-default).
- `docs/security/threat-model.md` — Information-disclosure row (Logger).
- `docs/compliance/README.md` — control C-2.
- `net/http.ResponseController`, `crypto`-free stdlib `log/slog`.
