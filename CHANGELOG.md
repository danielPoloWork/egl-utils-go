# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

- `circuitbreaker.State` / `(*Breaker).State()` — observable breaker state (roadmap 10.2, spec v2
  item 6): an exported `State` type (`StateClosed`/`StateOpen`/`StateHalfOpen`, with `String()`)
  and a `State()` accessor. `State()` is a pure read-only observer — it reflects the lazy,
  time-based transition (an open breaker whose cool-down has elapsed reports `StateHalfOpen`)
  without performing it, so polling it for metrics never admits a probe, mutates the breaker, or
  advances the generation. Additive; the existing `Do`/`ErrOpen` surface is unchanged (ADR-0030,
  lifting ADR-0010's deferral).
- `lifecycle.Trigger()` — programmatic shutdown (roadmap 10.3, spec v2 item 21): unblocks a pending
  `WaitForSignals` exactly as a termination signal would, for code that decides to stop the process
  itself (a fatal background error, an admin endpoint, a supervisor command). Idempotent and safe for
  concurrent use; a `Trigger` that arrives before `WaitForSignals` latches instead of being lost.
  Additive — `Register`, `Shutdown`, and `WaitForSignals` keep their signatures (ADR-0030).
- `ratelimit.(*Limiter).Middleware()` and `ratelimit.ErrLimited` — HTTP admission middleware (roadmap
  10.4, spec v2 item 8): a standard `func(http.Handler) http.Handler` decorator that admits each
  request through the limiter and answers `429 Too Many Requests` with a `Retry-After` of
  `ceil(1/rate)` seconds when the bucket is empty. Admission uses `Allow`, never `Wait`, so an
  over-budget burst is shed rather than queued into parked goroutines and held connections; the
  refusal body is the generic status text and discloses no limiter state, and denials are not logged
  by the library (they surface as ordinary 429s to `middleware.Logger`). The admit path allocates
  nothing (~30 ns/op, 0 allocs). One limiter bounds total throughput, not any one client's share —
  per-client limiting stays the consumer's decision. `ErrLimited` is the matching sentinel for
  callers gating their own work on `Allow`. Additive; the engine is unchanged (ADR-0031, ADR-0030).
- `hash.HashPasswordCost()`, `hash.Cost()`, and `hash.ErrInvalidCost` — configurable bcrypt work
  factor (roadmap 10.5, spec v2 item 20 and §7). `HashPasswordCost(pw, cost)` accepts cost **10–31**
  and validates the range **locally**, before the value reaches bcrypt: upstream silently promotes a
  cost below its own `MinCost` of 4 to the default (discarding the caller's intent) and honours costs
  4–9 verbatim, so a misconfigured cost would otherwise produce a weak hash that looks normal. An
  out-of-range cost returns a wrapped `ErrInvalidCost` naming the value and produces no hash at all.
  `Cost(hash)` reads the factor back out of a stored hash (~112 ns) so a store can be upgraded by
  verify-and-rehash-on-login. The package documentation adds the measured cost table, the
  argon2id-for-new-systems recommendation with its migration path, and the login-DoS warning —
  verification costs the same as hashing, so the cost is a per-login CPU multiplier; pair a high cost
  with `ratelimit.(*Limiter).Middleware`. Additive: `HashPassword` still produces cost 10, now by
  delegating to `HashPasswordCost` (ADR-0032, extending ADR-0024; control C-4 extended). Spec v2's
  default-cost change 10 → 12 remains deferred to `/v2` as breaking — reachable today via
  `HashPasswordCost(pw, 12)`.
- `config.WithStructValidation()` — opt-in tag-based validation of a loaded config (roadmap 10.6, spec
  v2 item 13): runs `validator.Struct` over the decoded value, so `validate:"required,email,min,max,
  oneof"` tags are enforced in the same call that loads the file. Failures come back as a wrapped
  `validator.ValidationErrors`, so `errors.As` reaches each `*validator.FieldError`. Tags run **before**
  the existing `Validator` interface and a tag failure skips `Validate`, so a `Validate` method may
  assume each field is individually well-formed and check only cross-field invariants. Opt-in by
  design — a struct with no `validate` tags would pass vacuously, so enabling it implicitly would imply
  a guarantee that is not there. Additive: a `Load` call without the option behaves exactly as before
  (ADR-0033).

### Changed

### Deprecated

### Removed

### Fixed

- `go.mod` / `go.sum`: re-tidied after the `prometheus/client_golang` 1.24.1 bump (#44), which left
  the transitive `prometheus/common`, `prometheus/procfs`, and `protobuf` pins at their pre-bump
  versions and `go.sum` without the matching entries — `go build ./...` failed on a clean module
  cache with *missing go.sum entry for go.mod file*.

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v1.0.0](docs/changelog/v1/v1.0.0.md) | 2026-07-15 | Feature-complete 1.0 — M2–M9: concurrency, resilience, HTTP middleware, config, structured logging, caching & DB, validation & bcrypt, diagnostics & lifecycle; API-stability commitment |
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
