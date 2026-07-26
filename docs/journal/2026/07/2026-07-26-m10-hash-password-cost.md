# 2026-07-26 — Milestone 10.5: hash.HashPasswordCost (security-relevant)

## What got done

- **Roadmap 10.5 `hash.HashPasswordCost` + `hash.Cost` + `hash.ErrInvalidCost`** (branch
  `feat/hash-password-cost`): the configurable work factor spec v2 item 20 asks for, plus the
  argon2id migration note and the §7 cost-sizing benchmark. **New
  [ADR-0032](../../../adr/0032-hash-password-cost-design.md)**, extending ADR-0024. This is the
  milestone's **security-relevant** item, so it carries the ADR, the extended control C-4, three new
  threat-model rows, and the security-auditor framing under the enterprise posture (ADR-0015).
- **The decisive finding: bcrypt's own validation is unsafe, so the range is enforced locally.**
  Verified empirically against `x/crypto v0.48.0` rather than assumed, and the result changed the
  design:

  | Requested cost | What bcrypt actually does |
  |---|---|
  | −1, 0, 3 (below `MinCost` = 4) | **silently promoted to `DefaultCost` (10)** — intent discarded, no error |
  | 4 … 9 | **honoured verbatim** — a real cost-4…9 hash, up to 64× cheaper to crack, looks normal |
  | 32+ | `InvalidCostError`, before any work |

  So upstream enforces its ceiling but not a meaningful floor, and its one "safety" behaviour is
  *silent* — which is worse than accepting or rejecting, because a zero value from an unset config
  field yields a cost-10 hash and **reports success**. `HashPasswordCost` therefore validates 10–31
  itself, before the value reaches bcrypt, and an internal test
  (`TestBcryptWouldAcceptWeakCosts`) pins the upstream behaviour so the justification stays checkable
  and an upstream change surfaces as a test failure rather than a stale comment.
- **Error, not panic — and the reasoning is structural, not stylistic.** ADR-0005 is loud-by-default,
  and `NewLimiter`/`cache.NewInMemory`/`Cors` all panic on invalid configuration. But those are
  **constructors with no error channel**, called once at wiring time; a panic is their only loud
  option. `HashPasswordCost` already returns an error, runs per hash, and sits on a request path
  (registration, password change), so a panic would crash a live handler. `config.Load`'s
  `ErrUnsupportedFormat` is the closer in-repo precedent — configuration-shaped invalidity reported as
  an error because a channel exists. A returned error is not silent (errcheck gates it), so
  loud-by-default is satisfied.
- **Rejected the `cost <= 0 means default` convenience** outright: it would reproduce bcrypt's silent
  promotion, the single worst behaviour in the table above. A caller wanting the default calls
  `HashPassword`; a zero cost is a misconfiguration and is reported as one.
- **`HashPassword` now delegates** to `HashPasswordCost(pw, bcrypt.DefaultCost)` — one code path, no
  behaviour change (still cost 10), and its test now reads the factor back via `Cost` instead of
  matching a `"$2a$10$"` string prefix.
- **One deliberate widening of the item's literal scope, flagged as such:** `Cost(hash) (int, error)`.
  Roadmap 10.5 lists only the constructor, the doc note, and the benchmark — but the *mandated*
  migration note prescribes rehash-on-login, which needs the stored factor, while the package's own
  contract promises callers "never need to import the underlying bcrypt package" (ADR-0024). Without
  `Cost`, the required deliverable is either unactionable or breaks its own encapsulation promise.
  It is one function and one test to remove if the maintainer disagrees; ADR-0032 records the call and
  the alternative (`NeedsRehash`) that lost.
- **The cost is documented as a trade-off, not a free strengthening.** The measured report shows
  verification costs the *same* as hashing at a given factor, and every login pays it on an endpoint an
  unauthenticated caller can reach: at cost 12, ~4.5 verifications/second saturate a core. Raising the
  cost hardens a leaked store *and* multiplies the CPU an attacker consumes per request. The mitigation
  composes in-module — 10.4's `ratelimit.(*Limiter).Middleware` (control C-5) is exactly the admission
  control this amplification needs, so the two milestone items now reference each other deliberately.
- **Cost drift named as an operational risk:** the same factor gets cheaper as hardware improves, so a
  fixed configuration silently weakens over time. The godoc prescribes periodic review against a re-run
  of the benchmark, and states plainly that hashes belonging to users who never return stay at the old
  factor — so rehash-on-login must be paired with a dormant-account policy rather than assumed to
  converge.
- **Cost-sizing benchmark + report** ([docs/benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md](../../../benchmarks/2026-07-26-hash-bcrypt-cost-sizing.md)),
  satisfying spec §7's "documented so deployers can size it". `BenchmarkHashPasswordCost` and
  `BenchmarkCheckPassword` sweep costs 10–14, `BenchmarkCost` covers the accessor. Median of 3:
  **55.46 / 110.67 / 221.52 / 443.26 / 887.04 ms** for hash, verify within noise of it, `Cost` at
  112 ns. Successive ratios 1.996–2.002 — the doubling is exact, so any cost extrapolates and cost 31
  works out to **~32 hours per hash**, which is why the godoc calls the ceiling a hard limit and not a
  recommendation.
- Tests: `hash` stays at **100% coverage**, suite ~2.6 s. The suite hashes only at costs 10 and 11 —
  the cheapest pair that proves the argument is honoured — since higher factors cost seconds per
  assertion for no extra guarantee. **Cost 31 is verified as inside the accepted range without ever
  hashing at it**, by pairing it with an over-long password so bcrypt refuses on length before doing
  work; that keeps the documented ceiling honest without a multi-hour test. `TestCostUpgradeOnLogin`
  walks the documented migration path end to end, so the procedure is executable rather than
  aspirational.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...`, gofumpt clean,
  golangci-lint v2 0 issues, govulncheck 0 affecting, `consistency_lint.py` OK. No dependency change —
  the x/crypto v0.48.0 pin and the Go 1.24 floor (ADR-0024) are untouched. `-race` is CI-only locally.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (5 of 13)**: 10.1 (#37), 10.2 (#38), 10.3 (#46), and 10.4
(#47) merged; 10.5 drafted on `feat/hash-password-cost`, awaiting the maintainer to open and merge —
and, being security-relevant, the auditor sign-off ADR-0032 records as required. M10 releases as
v1.1.0.

## How the next session resumes

Wait for the 10.5 PR to merge. Then **10.6 `config.WithStructValidation()`** — the milestone's
smallest item (roadmap tags it *low*): wire `validator.Struct` into `config.Load` as a functional
option, so a loaded config is tag-validated in the same call. `config.Load` already supports a
`Validator` interface (`Validate() error`, invoked via `any(&cfg).(Validator)` so a pointer receiver is
found — ADR-0018); the new option adds the reflection/tag path from ADR-0023 alongside it, and the
interaction between the two needs deciding and documenting: both, either, or validator-then-Validate.
Watch the import graph — `config` and `validator` are both L2, and 10.8's depguard will enforce
ADR-0004's allowlist, so confirm this edge is legal before building on it. Standard footprint per PR
(tests + goleak + coverage, CHANGELOG `[Unreleased]`, ROADMAP checkbox, journal, lint). Portable Go
under `%TEMP%\go-portable` — in the Bash tool add it as the *unix* path
`/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs the `/v2` module path;
`-race` is CI-only.
