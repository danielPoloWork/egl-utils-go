# ADR-0004: Runtime dependency policy

- **Status:** Accepted — **ring 3's membership** is superseded by
  [ADR-0050](0050-metrics-without-the-sdk.md) (two runtime entries → one; `prometheus/client_golang`
  removed in the `/v2` major). The three-ring *policy*, the test-only ring, and the
  superseding-ADR-before-the-import rule all stand, and ADR-0050 tightens rather than loosens them.
  Note also the first rejected alternative below, which ADR-0050 **partially adopts** — annotated in
  place with what the intake estimate got right and wrong.
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), decided at intake (interview Phase 2), recorded here
- **Related:** spec §3 (non-functional requirements), docs/compliance/README.md control C-1,
  AGENTS.md §7 (enterprise posture: security-relevant decisions require an ADR)

## Context

The specification commits to 25 features under an "idiomatic Go, stdlib-first" philosophy,
but three features force a dependency decision: `config.Loader` (YAML parsing),
`validator.Struct` (tag-driven validation), and `metrics.Prometheus` (metrics exposition).
Password hashing requires bcrypt (`golang.org/x/crypto`) in any case. Under the enterprise
governance posture, the supply-chain boundary is a security-relevant decision that must be
an ADR, not a per-PR judgment call. CI already carries `govulncheck` as a blocking gate.

## Decision

Runtime dependencies are limited to three rings, outermost ring closed by default:

1. **Standard library** — always preferred; `log/slog` for logging, `net/http` for
   middleware, `database/sql` for the transaction helper.
2. **`golang.org/x/*`** — treated as extended stdlib (e.g. `x/sync` for the weighted
   semaphore, `x/crypto` for bcrypt, `x/time` where useful).
3. ~~**Vetted third-party, exactly two runtime entries** — `prometheus/client_golang`
   (metrics exposition, feature 23) and one YAML parser for `config.Loader` (selected and
   pinned when Milestone 5 is implemented; `gopkg.in/yaml.v3` or its maintained successor,
   subject to review at that PR).~~
   **Superseded 2026-07-29 by [ADR-0050](0050-metrics-without-the-sdk.md): ring 3 has exactly
   *one* runtime entry, `gopkg.in/yaml.v3`.** `prometheus/client_golang` was removed with the
   `/v2` major — `metrics` writes the exposition format itself — and with it the seven transitive
   modules nothing else in the graph needed. The ring itself is unchanged as a *policy*: a third
   party still requires a superseding ADR before the import lands. Struck rather than rewritten
   because the count was a commitment to consumers about what this module drags in, and the record
   of it shrinking is the point.

**Test-only** dependencies (never imported by production code): `testify`, `goleak`,
`rapid`. Any dependency outside these rings requires a superseding ADR before the import
lands. `go.sum` is committed; Dependabot watches `gomod` weekly; `govulncheck` stays a
blocking CI gate.

*(Enforced since roadmap 10.8 — [ADR-0035](0035-import-graph-enforcement.md): `depguard` rules in
`.golangci.yml` confine each module above to the single package whose ADR justified it, and
`tools/import_graph_lint.py` asserts the rings over the resolved graph, including that no direct
requirement appears outside them and that no test-only dependency reaches production code. Adding a
dependency now means editing both files in the PR that carries the superseding ADR.)*

## Alternatives Considered

- **Strict stdlib + `golang.org/x` only** — zero third-party runtime deps: hand-rolled
  Prometheus text exposition, JSON+env-only config, fully hand-rolled validator, stdlib-only
  tests. Rejected at intake: significant extra milestone work for marginal supply-chain gain;
  the maintainer chose the vetted-few posture explicitly.
  > *Annotated 2026-07-29 (ADR-0050): **this alternative has been adopted in part**, and the
  > estimate that rejected it is worth reading against what happened.* Two of its four elements
  > went this way in the end — the validator was hand-rolled at intake anyway
  > ([ADR-0023](0023-validator-design.md)), and **ADR-0050 hand-rolls the Prometheus text
  > exposition**, removing `client_golang` from ring 3. Neither half of the rejection reason held up
  > once measured: the "marginal supply-chain gain" was **nine of the module's eighteen modules**
  > (`go.sum` 50 lines → 24), because `client_golang` brought seven transitive modules that nothing
  > else needed; and the "significant extra milestone work" was estimated *before*
  > [ADR-0027](0027-metrics-prometheus-design.md) bounded the feature to two metric families and two
  > labels — the work scales with the surface, and that surface turned out to be small enough to
  > write directly, faster and allocation-free. **The estimate was reasonable for an unbounded
  > "metrics" feature and wrong for the bounded one that shipped**; JSON+env-only config and
  > stdlib-only tests remain rejected on their original grounds. See ADR-0050 for the measurements.
- **Permissive per-feature dependencies** — pick the most convenient library per feature.
  Rejected: unbounded supply-chain surface contradicts the enterprise posture and makes
  `govulncheck` findings a moving target.

## Consequences

- Compliance control **C-1** is registered in `docs/compliance/README.md` with this ADR as
  its decision record and the `govulncheck` CI gate + `go.mod`/`go.sum` review + Dependabot
  as its evidence.
- Milestones 5 (config), 8 (validator, hash), and 9 (metrics) implement against a bounded
  dependency budget; a need that exceeds it surfaces as a superseding ADR, visible in review.
- The validator (feature 19) is implemented in-repo against the spec's tag grammar
  (`required, email, min, max, oneof`) rather than importing a validation framework.

## References

- `docs/specs/01_spec_utils.md` §3 (supply-chain NFR), §2 features 13/19/20/23.
- `.github/workflows/ci.yml` — `quality / lint + race + vuln` job (govulncheck).
- `.github/dependabot.yml` — `gomod` weekly.
