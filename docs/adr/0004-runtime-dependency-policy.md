# ADR-0004: Runtime dependency policy

- **Status:** Accepted
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
3. **Vetted third-party, exactly two runtime entries** — `prometheus/client_golang`
   (metrics exposition, feature 23) and one YAML parser for `config.Loader` (selected and
   pinned when Milestone 5 is implemented; `gopkg.in/yaml.v3` or its maintained successor,
   subject to review at that PR).

**Test-only** dependencies (never imported by production code): `testify`, `goleak`,
`rapid`. Any dependency outside these rings requires a superseding ADR before the import
lands. `go.sum` is committed; Dependabot watches `gomod` weekly; `govulncheck` stays a
blocking CI gate.

## Alternatives Considered

- **Strict stdlib + `golang.org/x` only** — zero third-party runtime deps: hand-rolled
  Prometheus text exposition, JSON+env-only config, fully hand-rolled validator, stdlib-only
  tests. Rejected at intake: significant extra milestone work for marginal supply-chain gain;
  the maintainer chose the vetted-few posture explicitly.
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
