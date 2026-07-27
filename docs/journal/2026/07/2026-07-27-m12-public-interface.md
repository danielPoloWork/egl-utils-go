# 2026-07-27 — M12 opens: §5 reconciled with the real exported surface (12.1)

## What got done

- **Milestone 12 planned and opened.** It exists because the M11 read-through found nine spec
  divergences and 11.2 only closed four; the remaining five were held back deliberately, because §5
  is a rewrite and its closing line is a question about the promise made to consumers rather than a
  stale number. 12.1 lands here; 12.2 (the §4 isolation claim, §6/§3 understatements) and 12.3
  (`tools/spec_api_lint.py`) are planned and unstarted.
- **§5 rebuilt from `go doc` output, not by hand.** It had never been updated after Milestone 10, so
  **twelve identifiers shipped in v1.1.0 were missing** — `(*Breaker).State` and the `State` type
  with its three constants and `String()`, `lifecycle.Trigger`, `(*Limiter).Middleware` and
  `ErrLimited`, `HashPasswordCost`/`Cost`/`ErrInvalidCost`, `config.WithStructValidation`,
  `pubsub.WithDropOldest`. Pre-M10 omissions came out with them: `middleware.HeaderName`,
  `errors.StackTracer`, `validator.ValidationErrors`/`FieldError`, `logger.Field` and its five
  constructors, `workerpool.Task`, every `WithX` option constructor, and the root package's
  `Version`.
- **One listed signature was wrong, not merely absent.** §5 gave
  `pubsub.NewBroker[T](opts ...Option)`, where the real option type is **generic** — `Option[T]`.
  A consumer writing an option against the spec would not compile. That is the failure mode a
  documentation-only divergence is supposed to be too harmless to have.
- **The substantive half is the versioning clause.** It read "SemVer over all exported identifiers
  **above**", which makes the enumeration *the boundary of the promise* — so every identifier the
  list had drifted out of was, as written, outside the stability commitment. The v1.0.0 changelog
  had promised "API stability for every exported identifier", so **the spec was narrower than what
  was published**. It now binds the module's whole exported surface, names `go doc` as the
  authority, states that the list is a reader's map rather than the boundary, and excludes
  `contrib/*` per ADR-0040. This is an **amendment, not an ADR**: it widens the spec to match a
  promise already made, so there is no new decision to record — the opposite of 11.2's
  compatibility clause, which took an ADR precisely because the promise itself had changed.
- **The rewrite was verified mechanically rather than by eye**, with a throwaway script that
  extracts every exported identifier from `go doc -all` for all 21 packages and greps §5 for each:
  **110 identifiers checked, none missing.** Writing that script is also the argument for 12.3 —
  it took a few minutes and would have caught all twelve M10 omissions the day they landed.

## Where the project stands

**Milestone 12 in progress (1/3).** Documentation only so far: no code, no version bump, `version.go`
stays at 1.1.0, no CHANGELOG entry. M11 merged as `804ae14` and its GitHub milestone is closed.

## How the next session resumes

**12.2** is the smaller of the two remaining items and has no open questions: supersede §4's
"packages compose only through stdlib contracts … each is adoptable in isolation" with a marker
pointing at the **existing** [ADR-0033](../../../adr/0033-config-struct-validation.md) — no new ADR,
since that one already holds the decision — and correct §6's test-strategy counts (rapid runs in
eight packages, not three; benchmarks exist in seven, not four) and §3's dependency sentence, which
omits `prometheus/client_model`.

**12.3** is the one that matters beyond this milestone. The throwaway script above should become
`tools/spec_api_lint.py`, failing in **both** directions: an identifier shipped but unlisted (how
M10's twelve accumulated silently) and an identifier listed but gone (a stale promise to consumers).
It must parse identifiers rather than whole lines, since §5 carries prose around each signature, and
must skip `contrib/*`. It then joins `consistency_lint.py`, `import_graph_lint.py` and
`coverage_gate.py` as the fourth policy tool run before every PR.

Unchanged carry-overs: the EADOS bundle's `go.yaml` profile still asserts the removed source tree
and must be fixed upstream (`.eados-core/` is gitignored); `egl-util-cpp` ships `it/d4np/util`
against ADR-0041's `utils`; the ADR-0030 `/v2` ledger; the NFR-01 spec amendment; the
`middleware.HeaderName` canonicalisation decision; and first tags for the two contrib modules.

## Addendum — roadmap 12.2, §4 superseded and the §6/§3 understatements corrected (same session)

12.1 merged as `7cb90b3`; 12.2 follows on `docs/spec-architecture-and-tests`.

- **§4's absolute is struck in place and superseded by the *existing* [ADR-0033](../../../adr/0033-config-struct-validation.md).**
  The claim was "packages compose only through stdlib contracts (context.Context,
  net/http.Handler, error), so each is adoptable in isolation"; `go list` shows `config` imports
  `validator`. **No new ADR was minted** — that is the decision worth recording here. 11.2's
  compatibility clause needed ADR-0042 because no prior record held the replacement rule; this one
  is fully decided in ADR-0033, so a fourth governance document would contribute a pointer and no
  information. The supersede marker *is* the pointer.
- **The replacement wording says "governed exception", not "one exception".** That distinction is
  the whole reason the edge is tolerable: `import_graph_lint.py` fails **in both directions** — when
  an unsanctioned edge appears *and* when `config → validator` disappears (ADR-0035) — and the rule
  it enforces is "same-layer edges only where the spec mandates the composition", not "L2 is a
  free-for-all". Spec item 13 is what mandates this one, by requiring validation inside
  configuration loading. Written loosely, the amendment would have read as the architecture rule
  eroding; it is the opposite, an exception with a lock on it.
- **The isolation property is qualified, not dropped.** Every other package is still adoptable
  alone, and adopting `config` also brings `validator` — which costs nothing, since both ship in
  this module.
- **§6 and §3 were understatements rather than false claims**, and corrected against `grep`/
  `go list` rather than memory: rapid was credited with three areas and runs in **eight** packages
  (adds circuitbreaker, middleware, ratelimit, validator); benchmarks were credited to four
  packages and exist in **seven** (adds cache, hash, pubsub); §3's dependency sentence omitted
  `prometheus/client_model`, a direct `go.mod` require that no non-test file imports. A spec that
  under-describes its own gates invites the next reader to re-derive them.

**Milestone 12 now stands at 2/3.** The remaining item, 12.3, is the reason all four of these
accumulated: none was caught by a gate.
