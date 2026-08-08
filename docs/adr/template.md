# ADR-NNNN: <Short, descriptive title in imperative or noun form>

- **Status:** Proposed | Accepted | Superseded by ADR-XXXX | Deprecated
- **Date:** YYYY-MM-DD
- **Deciders:** <names or roles>
- **Related:** <ADR-XXXX, spec section, issue, PR>

## Context

What is the situation that motivates this decision? Describe the forces at play: functional
requirements, non-functional constraints (performance, compatibility, portability,
concurrency), prior art in the codebase, and any external pressures. Be specific enough
that a future reader who never saw the original discussion can reconstruct the problem.

## Decision

State the decision in a single declarative paragraph. Use active voice. The decision is the
contract — everything else in this document is supporting evidence.

## Alternatives Considered

For each rejected option, give a one-paragraph summary and the specific reason it lost.

- **Alternative A** — <description>. Rejected because <reason>.
- **Alternative B** — <description>. Rejected because <reason>.

## Consequences

Describe the outcomes — both the wins this decision unlocks and the costs it imposes:

- API / compatibility implications
- Performance trade-offs (with rough magnitudes if known)
- Testing and tooling impact
- Documentation that must be added or updated
- Risks and known limitations
- **Deferred, additive:** <capability this decision deliberately does not build, and which
  could arrive later without breaking anything>. Use this exact marker — `consistency_lint.py`
  keys on it — and add the matching row to [ADR-0057](0057-additive-capability-ledger.md) §A or §B
  in the same PR, with the trigger that would schedule it. Omit the bullet entirely if the
  decision defers nothing. If you cannot name the shape that makes the capability additive
  (a new identifier, a new option, a new signature that breaks none), it is a breaking change
  and belongs in [ADR-0030](0030-spec-v2-reconciliation.md) §2 instead.

## References

- Spec sections, papers, related ADRs, benchmark results, prior incidents.
