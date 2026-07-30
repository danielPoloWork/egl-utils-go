# Releases

Per-version release notes for `egl-utils-go`. The release process
([`../workflow/release.md`](../workflow/release.md)) drafts `v<MAJOR>.<MINOR>.<PATCH>.md` here
for each release; the maintainer publishes the matching GitHub Release. The consistency lint's
`version-lockstep` check keeps the latest file here in step with the version constant and the
README badge.

## Index

| Version | Date | Highlights | Notes |
|---------|------|------------|-------|
| v2.0.0  | 2026-07-30 | M13 — the second major: `pkg/` layout and the `/v2` module path (every import changes), plus all seven deferred breaking changes discharged at once; nine dependency modules dropped. One change no compiler reports: bcrypt's default cost 10 → 12 | [v2.0.0.md](v2.0.0.md) |
| v1.1.1  | 2026-07-27 | Two allocations removed from every `RequestID` request via a canonical header key, `HeaderName` unchanged (ADR-0044); the `-race` CI jobs repaired after a day red (BUG-0001). No API change | [v1.1.1.md](v1.1.1.md) |
| v1.1.0  | 2026-07-27 | M10 — spec v2.0 reconciliation (additive bucket): new capability across 7 packages, cache sharded 7.5×, fuzzing + import-graph + coverage gates, the NFR suite, and the contrib/* probe modules | [v1.1.0.md](v1.1.0.md) |
| v1.0.0  | 2026-07-15 | Feature-complete 1.0 — all 25 features (M2–M9); API-stability commitment | [v1.0.0.md](v1.0.0.md) |
| v0.1.0  | 2026-07-12 | M1 bootstrap: module + quality gates live; ADR-0003/0004 | [v0.1.0.md](v0.1.0.md) |
