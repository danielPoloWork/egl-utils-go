# ADR-0016: middleware.Recoverer design — panic-to-500, no stack to the client, ErrAbortHandler passthrough

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role
- **Related:** spec §2 feature 11, §5 (Recoverer API); ROADMAP 4.3; ADR-0013 (middleware
  foundation — Decorator, loud nil, direct-vs-constructor shape); ADR-0014 (Logger —
  `responseRecorder`/`Unwrap`, path-only logging); ADR-0015 (enterprise posture — why this
  security-relevant decision carries an ADR); threat model (public HTTP edge, Information
  disclosure); compliance control C-2

## Context

Feature 11 is "handle internal panics in HTTP controllers by sending clean 500 responses and
avoiding a server crash", API frozen at intake as the unconfigured decorator
`Recoverer(next http.Handler) http.Handler` (ADR-0013's direct shape). It inherits the package
conventions from ADR-0013/0014 (decorator shape, loud nil, the `responseRecorder` with
`Unwrap`), so the open decisions are specific to panic handling: what the *client* is allowed
to see, what the *operator* must see and where, how to behave when the response is already
committed, and which panics must not be recovered. Two of these are security-relevant
(information disclosure), which under the enterprise posture (ADR-0015, AGENTS.md §7) requires
this ADR.

## Decision

**Recover, then write a generic 500 — never the panic detail.** The middleware runs `next`
under a deferred `recover()`. On a non-nil recovered value it writes
`http.Error(w, http.StatusText(500), 500)` — the generic status text, `Content-Type:
text/plain`, and `X-Content-Type-Options: nosniff` that `http.Error` sets. The **panic value and
stack trace are never written to the response**: a leaked stack exposes source paths, symbol
names, and internal structure — a textbook information-disclosure vector — and a leaked panic
message can carry secrets the handler was mid-processing. The client sees only that something
failed.

**Log the panic server-side at Error via `slog.Default`.** The operator does need the detail, so
`Recoverer` logs one Error-level record ("http handler panic") to the process default slog logger,
carrying `method`, `path`, the `panic` value, a `stack` trace (`runtime/debug.Stack`), and
`request_id` when the chain seeded one (ADR-0013). Two consequences of the frozen unconfigured
signature drive `slog.Default`: `Recoverer(next)` takes no logger argument, and the module has
already standardized on `log/slog` (ADR-0014). A consumer redirects the output by calling
`slog.SetDefault`. Like Logger, it logs **`r.URL.Path` only, never the query string** (same
Information-disclosure rule, ADR-0014). The stack is captured server-side only.

**Do not recover `http.ErrAbortHandler`.** net/http uses `panic(http.ErrAbortHandler)` as a
sentinel meaning "abort this handler silently" (used by, e.g., `ReverseProxy` and streaming
handlers). Swallowing it would convert an intentional silent abort into a logged 500 and defeat
the server's own control flow. `Recoverer` detects that exact value and **re-panics it unchanged**;
net/http's own outer recover then does the right thing. It is neither logged nor turned into a 500.

**Leave an already-committed response untouched.** If `next` wrote a status or body before
panicking, the response is committed and the status can no longer be changed. `Recoverer` uses the
package's `responseRecorder` (ADR-0014) — which tracks `wroteHeader` and exposes `Unwrap` so
`http.ResponseController` still reaches the underlying Flusher/Hijacker — to detect this: it writes
the 500 only when nothing has been committed yet. Either way the panic is logged. This reuses the
existing recorder rather than adding a second near-identical wrapper.

**Compose innermost.** The recommended chain is `RequestID → Logger → Recoverer → handler`. With
Recoverer innermost, the recovered 500 is the status the outer Logger observes (ADR-0014), and the
`request_id` seeded by RequestID is in context for Recoverer's log line. The godoc states this.

**Loud nils** (ADR-0013 lineage): `Recoverer(nil)` panics at setup.

## Alternatives Considered

- **Write the stack/panic message into the 500 body** (some dev-mode middleware do) — convenient
  for local debugging, but ships internal detail to any client in production. Rejected outright;
  the stack goes to the log, never the wire. A consumer wanting a dev-mode echo wraps it themselves.
- **Recover everything, including `ErrAbortHandler`** — simpler branch, but breaks net/http's
  silent-abort contract and mislabels intentional aborts as 500s. Rejected.
- **Take a `*slog.Logger` (constructor form `Recoverer(l) func(...)`)** — more explicit than
  `slog.Default`, and symmetric with Logger. But it contradicts the spec-frozen unconfigured
  signature (§5, ADR-0013). Deferred: an additive `RecovererWithLogger` can arrive later without
  breaking `Recoverer(next)`.
- **Log via the standard `log` package / write to `os.Stderr`** — dependency-free, but forfeits
  structured fields and the `request_id` correlation the rest of the middleware chain provides.
  Rejected for `slog`.
- **Skip the `responseRecorder` and always call `WriteHeader(500)`** — smaller, but emits a
  "superfluous WriteHeader call" and a corrupt 200-then-500 response when the handler had already
  committed. Rejected; commit-detection via the recorder is correct and reuses existing code.
- **Return an option struct (custom status, custom body, on-panic hook)** — more flexible, widens a
  surface the spec froze. Deferred; additive options can arrive without breaking `Recoverer(next)`.

## Consequences

- A panicking handler yields a clean generic 500 (or a preserved committed response) and a crash is
  contained; the client learns nothing about internals.
- Every recovered panic produces one structured Error line (value + stack + `request_id`) on the
  default slog logger — the operator's signal, correlated with Logger's request line by
  `request_id`. `ErrAbortHandler` produces neither a line nor a 500.
- The Information-disclosure control (no stack/panic to the client; path-only logging) is added to
  the threat model's Logger/Recoverer rows and to compliance **C-2**.
- No new pattern: Recoverer is another **Decorator** (catalogue row 9), the shape ADR-0013
  established; it reuses the `responseRecorder` ADR-0014 introduced.
- Milestone 4 is one item from complete (4.4 Cors remains).

## References

- `docs/specs/01_spec_utils.md` §2.11, §5.
- ADR-0013 (middleware foundation), ADR-0014 (Logger — recorder/Unwrap, path-only), ADR-0015
  (enterprise posture).
- `docs/security/threat-model.md` — Information-disclosure rows (Logger, Recoverer).
- `docs/compliance/README.md` — control C-2.
- `net/http.ErrAbortHandler`, `net/http.ResponseController`, `runtime/debug.Stack`, `log/slog`.
