# ADR-0027: metrics.Prometheus design — bounded-cardinality labels, client_golang pin, uncalled-vuln trade-off

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent, security-auditor role
- **Related:** spec §2 feature 23, §5 (`Prometheus(reg prometheus.Registerer) func(http.Handler)
  http.Handler`, `Handler() http.Handler`); ROADMAP 9.3; ADR-0004 (runtime dependency policy —
  `prometheus/client_golang` is the pre-approved ring-3 entry); ADR-0009/0024 (floor-preserving
  dependency pins); ADR-0013 (Decorator middleware shape)

## Context

Feature 23 is Prometheus instrumentation, frozen at intake as
`Prometheus(reg prometheus.Registerer) func(http.Handler) http.Handler` plus `Handler()
http.Handler`. It brings in `github.com/prometheus/client_golang` — the last of ADR-0004's two
budgeted third-party runtime dependencies. The open decisions: what to measure and, critically,
**how to label it without letting metric cardinality explode** (the classic Prometheus production
failure, and an attacker-reachable one), how the two frozen functions relate, and how to pin the
dependency without disturbing the module's Go 1.24 floor.

## Decision

**Two metrics, labelled `(method, code)` only.** A counter `http_requests_total` and a histogram
`http_request_duration_seconds` (`DefBuckets`), each labelled by request method and response status
code. **The request path is never a label** — it is unbounded (every URL a distinct series) and is
the textbook cause of Prometheus memory blow-ups; a consumer wanting per-route metrics uses a
router that labels by *route template*, not raw path, and instruments per-route itself.

**The method label is normalized to bound cardinality against untrusted input.** `r.Method` is
client-controlled and can be any token, so an attacker could mint unbounded distinct methods to
inflate cardinality (a memory-exhaustion vector). `Prometheus` maps the method to the nine known
HTTP methods, bucketing anything else to `"other"`, so the `method` label is bounded by
construction regardless of what a client sends. Status code is bounded by the finite HTTP code set.

**Status capture via an `Unwrap`-aware `statusRecorder`.** The middleware wraps the writer to
observe the status (defaulting to 200), and exposes `Unwrap()` so `http.ResponseController` still
reaches the underlying Flusher/Hijacker — the same technique as the middleware package (ADR-0014),
re-implemented here because the type is unexported and packages cannot share it.

**`Prometheus(reg)` registers on the caller's registry; `Handler()` serves the default one.** The
two frozen functions are intentionally decoupled: `Prometheus(reg)` registers the collectors on any
`prometheus.Registerer` (via `MustRegister`, which panics on a double install — a loud wiring
error), and `Handler()` returns `promhttp.Handler()` over the default registry. The documented
pairing is `Prometheus(prometheus.DefaultRegisterer)` + `Handler()`; for a custom registry a
consumer exposes it with `promhttp.HandlerFor` directly. Loud panic on a nil registerer or nil
handler (ADR-0005).

**Dependency pin: `client_golang` v1.23.2, floor preserved.** v1.23.2 (latest stable) declares
`go 1.23.0`, comfortably under the module's 1.24 floor, so it does not raise the `go` directive —
no floor-preservation gymnastics were needed (unlike x/crypto, ADR-0024). It is ADR-0004's
pre-approved ring-3 dependency; `go.sum` committed, Dependabot watches it.

**Accepted uncalled advisory: GO-2026-5024.** The transitive dependency `golang.org/x/sys@v0.41.0`
(pulled by `prometheus/procfs`) carries GO-2026-5024 (an integer overflow in
`x/sys/windows.NewNTUnicodeString`). `govulncheck` confirms **our code does not call it** (0 called
vulnerabilities). Its fix lands only in `x/sys@v0.44.0`, whose `go` directive is **1.25.0** — bumping
to it would raise the module floor to Go 1.25 and drop documented 1.24 support. The trade-off favours
the floor: an **uncalled**, Windows-only advisory does not justify dropping a supported Go version.
The pin is left at v0.41.0 and this is revisited when the floor moves to 1.25 (or if the advisory
becomes reachable). The CI `govulncheck` gate (called vulnerabilities) stays green.

## Alternatives Considered

- **Label by request path** — the per-endpoint breakdown everyone first wants, and the classic
  cardinality bomb (and an attacker-driven one via arbitrary paths). Rejected; route-template
  labelling is the consumer's router's job.
- **Label by status *class* (2xx/4xx/5xx)** — lower cardinality still, but exact code is bounded and
  far more useful operationally (distinguishing 404 from 429 from 400). Chose exact code; method
  normalization already caps the cardinality product.
- **Not normalizing the method** — simpler, but leaves an attacker-controlled unbounded label — a
  memory-exhaustion vector. Rejected.
- **`promauto` registration** — terser, but implicit; explicit `NewCounterVec` + `MustRegister`
  makes the registry relationship and the double-register panic obvious. Minor; either is fine.
- **A third-party instrumentation middleware** — client_golang already ships promhttp instrumentation
  helpers, but the frozen API is our own thin decorator; no extra dependency.
- **Bumping `x/sys` to clear GO-2026-5024** — clears an uncalled advisory at the cost of the Go 1.24
  floor (x/sys v0.44.0 needs Go 1.25). Rejected for now; the floor is worth more than clearing an
  unreachable finding.

## Consequences

- A drop-in RED-metrics middleware (requests + duration by method and code) with cardinality bounded
  by construction — safe against both accidental and adversarial label explosion.
- `prometheus/client_golang` v1.23.2 is a direct runtime dependency (ring 3, ADR-0004), completing
  the module's dependency budget; the Go 1.24 floor is unchanged.
- One uncalled advisory (GO-2026-5024) is knowingly carried to preserve the floor, documented here
  and revisited at the 1.25 floor move; a threat-model DoS row records the cardinality mitigation.
- Milestone 9 continues: syncpool (9.4) and errors (9.5) remain.
- Deferred, additive: an in-flight-requests gauge, a response-size histogram, configurable buckets.

## References

- `docs/specs/01_spec_utils.md` §2.23, §5.
- ADR-0004 (dependency policy — prometheus pre-approved), ADR-0024/0009 (floor-preserving pins),
  ADR-0013/0014 (Decorator + `Unwrap` recorder), ADR-0005 (loud-by-default).
- `docs/security/threat-model.md` — metric-cardinality DoS row.
- `github.com/prometheus/client_golang` (`prometheus`, `promhttp`); `govulncheck` GO-2026-5024.
