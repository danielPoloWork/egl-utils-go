# Compliance docs — egl-utils-go

The **control register** for `egl-utils-go`, present because this project runs under the
**enterprise governance posture** (`governance.posture: enterprise`, ADR-0015; see
[`AGENTS.md`](../../AGENTS.md) §3/§7/§10). It records the controls the project commits to and the
**evidence** each one maps to — so a reviewer can trace a claim ("access is authenticated",
"secrets never land in logs") to the artifact that substantiates it, not to a memory.

This is the **enterprise counterpart** to the always-present security surface: `SECURITY.md` is
the policy, [`../security/threat-model.md`](../security/threat-model.md) is the STRIDE analysis,
the audit risk register is the outcome — and this register is the standing map of *controls →
evidence* the raised bar expects to exist between audits.

## How to use it

- **One row per control.** A control is a commitment the project is held to — an authn/authz
  rule, a crypto choice, a data-handling constraint, a dependency-hygiene gate, a trust-boundary
  assumption.
- **Every control names its evidence.** The ADR that decided it (a security-relevant decision
  **requires** an ADR under this posture — `AGENTS.md` §7), plus where it is enforced or verified
  (a test, a CI gate, the threat model, a code path).
- **Same-PR upkeep.** A change that touches a registered control updates its row in the same PR —
  the `consistency_lint.py` posture check keeps this register and the `AGENTS.md` posture
  declaration in lockstep (neither may exist without the other).

## Control register

| # | Control | Decided in (ADR) | Evidence (test / gate / doc) | Status |
|---|---------|------------------|------------------------------|--------|
| C-1 | Runtime dependency policy — stdlib + `golang.org/x/*` + two vetted runtime deps (prometheus client, YAML parser); test-only testify/goleak/rapid | [ADR-0004](../adr/0004-runtime-dependency-policy.md) | `govulncheck` (CI `quality` job) · `go.mod`/`go.sum` review · Dependabot `gomod` weekly | Active |
