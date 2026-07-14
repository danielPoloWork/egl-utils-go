# ADR-0015: Enterprise governance posture — a raised compliance bar orthogonal to the domain

- **Status:** Accepted
- **Date:** 2026-07-12 (decision, init phase Q0.5); recorded 2026-07-15 (backfilled record —
  the decision was live and referenced from init, but its ADR was never written until now)
- **Deciders:** Maintainer (Daniel Polo), enterprise-architect role
- **Related:** init-phase decision Q0.5 (`orchestrator/project.yaml` → `governance.posture:
  enterprise`); AGENTS.md §1, §3, §7, §10; `docs/compliance/README.md` (control register);
  `docs/security/threat-model.md`; consistency-lint check 8 (posture ↔ register)

## Context

At project init (question Q0.5) the maintainer set `governance.posture: enterprise` in
`orchestrator/project.yaml`. That choice has shaped the repository from the first commit —
it is cited in AGENTS.md §3, enforced by AGENTS.md §7/§10, backed by the
`docs/compliance/` control register, and guarded by a dedicated `consistency_lint.py` check
(posture ↔ register, both directions). Five artifacts already reference **"ADR-0015"** by
name as the record of this posture, but the ADR itself was never authored — a documentation
debt: the decision was real and in force, its record was missing.

This ADR backfills that record. It records no new decision; it documents the existing one
so the references resolve to an actual file and the ADR index is complete. Its number
(0015) is the one those references already reserve; it is authored now, ahead of ADR-0016
(middleware.Recoverer), which is the PR that surfaced the gap. Numbering is assignment-order,
not date-order (ADR §7), so a late-authored record of an early decision is expected.

The posture is **orthogonal to the domain**: it says nothing about Go, concurrency, or HTTP —
it raises the *governance* bar. The alternative postures an init could select (e.g. a
lightweight OSS-utility posture) would drop the mandatory-security-ADR and compliance-register
obligations; `enterprise` keeps them.

## Decision

**Adopt the enterprise governance posture for `egl-utils-go`.** Concretely, over and above
the baseline every EGL project carries, the enterprise posture commits the project to:

- **Mandatory ADRs for security-relevant decisions** (AGENTS.md §7/§10). Any decision touching
  authn/authz, cryptography, data handling, a trust boundary, or a dependency with a known CVE
  surface **requires an ADR** — it is never an undocumented judgment call. (This is why
  RequestID carried ADR-0013, Logger ADR-0014, and Recoverer carries ADR-0016.)
- **A maintained compliance-docs surface** — `docs/compliance/` with a control register mapping
  each committed control to its evidence (the deciding ADR plus where it is enforced or verified).
  A PR that touches a registered control updates its row in the same PR.
- **Stricter review** (AGENTS.md §10): two approving reviews before merge, and a
  security-relevant change additionally requires the `security-auditor` role's sign-off.
- **The posture is not prose-only.** `consistency_lint.py` check 8 asserts the AGENTS.md
  declaration and the compliance register co-exist — neither may drift out on its own.

The posture rides alongside, and does not replace, the always-present security surface
(`SECURITY.md` policy, `docs/security/threat-model.md` STRIDE analysis, the audit risk register).

## Alternatives Considered

- **A lightweight (non-enterprise) posture** — no mandatory security ADRs, no compliance register,
  single-review merges. Lower ceremony, faster iteration. Rejected at init: `egl-utils-go` is an
  Enterprise-Grade Libraries reference artifact whose purpose includes *demonstrating* the raised
  bar; the ceremony is a feature, not overhead, for this project's goals.
- **Leaving the posture prose-only in AGENTS.md** (no ADR) — the status quo that created this debt.
  Rejected: an unrecorded governance decision that five artifacts cite by ADR number is exactly the
  "undocumented judgment call" the posture itself forbids.
- **Renumbering / dropping the "ADR-0015" references** to free the number for another decision —
  considered when the Recoverer PR hit the sequential-numbering rule. Rejected: the references
  describe a specific, in-force decision; the honest fix is to write the record they point to,
  not to erase the pointer.

## Consequences

- The five existing "ADR-0015" references (AGENTS.md §3, `docs/compliance/README.md`,
  `docs/README.md`, `orchestrator/project.yaml` Q0.5, `consistency_lint.py` check 8) now resolve
  to a real record; the ADR index is gap-free through 0015, unblocking ADR-0016.
- No behavioural or code change: this ratifies a posture that has governed every PR since init.
  The compliance register (C-1, C-2) and the mandatory-security-ADR practice are its standing
  evidence.
- Future security-relevant work continues to carry its own ADR; this one is the umbrella that
  says *why* those are mandatory here and not merely encouraged.

## References

- `orchestrator/project.yaml` — `governance.posture: enterprise` (Q0.5).
- AGENTS.md §1 (persona), §3 (posture declaration), §7 (documentation maintenance — mandatory
  security ADRs, compliance docs), §10 (enterprise quality bar — review, security ADR, compliance).
- `docs/compliance/README.md` — the control register this posture requires.
- `docs/security/threat-model.md` — the STRIDE surface the posture keeps current.
- `tools/consistency_lint.py` — check 8 (enterprise posture ↔ compliance register).
