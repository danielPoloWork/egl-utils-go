# Changelog

All notable changes to `egl-utils-go` are documented here, following
[Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning 2.0.0](https://semver.org/).

Every PR that introduces a user-visible change adds a line to `[Unreleased]` in the same
PR. A release PR moves the `[Unreleased]` entries into a new per-version file under
`docs/changelog/v<MAJOR>/v<X.Y.Z>.md` and adds an index row below.

## [Unreleased]

### Added

- `workerpool.Pool` — bounded-queue worker pool (roadmap 2.1): blocking or fail-fast
  `Submit`, context-aware `Stop` with full drain, opt-in panic containment (ADR-0005).
- `pubsub.Broker[T]` — filtered in-memory publish-subscribe broker (roadmap 2.2):
  at-most-once buffered delivery with observable drops, no broker goroutines, additive
  `Close` (ADR-0006).
- `fanin.Merge[T]` — multi-channel merge with completeness and per-input ordering
  guarantees, cancel-or-drain consumer contract (roadmap 2.3, ADR-0007).
- `fanout.Split[T]` — exactly-once multi-channel work distribution with per-output input
  ordering; forwarder-per-output, closes the outputs on input-close or cancel
  (roadmap 2.4, ADR-0008).
- `semaphore.Weighted` — weighted admission control, a thin house-contract adapter over
  `golang.org/x/sync/semaphore` with loud panics on misuse (roadmap 2.5, ADR-0009). Adds
  the module's first runtime dependency, `golang.org/x/sync` v0.16.0 (newest release on a
  `go 1.23` directive, so the module's `go 1.24` floor is preserved unchanged).
- `circuitbreaker.Breaker` — closed/open/half-open circuit breaker (roadmap 3.1):
  consecutive-failure tripping, lazy timerless state transitions (no goroutines, no
  timers), generation-guarded outcome accounting, half-open probe budget equal to the
  success threshold (ADR-0010).
- `retry.Backoff` — retrying execution with exponential backoff and proportional jitter
  (roadmap 3.2): total-attempt budget, hard `MaxDelay` cap that survives jitter,
  overflow-safe doubling, context cancellation honored before the first call and during
  every sleep (ADR-0011).
- `ratelimit.Limiter` — token-bucket rate limiter (roadmap 3.3, completes Milestone 3):
  lazy refill with no background goroutines or timers, fail-fast `Allow` and queueing
  `Wait` with arrival-order reservations, canceled waits repay their token; ~25ns
  zero-allocation admission (first report under `docs/benchmarks/`) (ADR-0012).
- `middleware.RequestID` / `middleware.RequestIDFrom` — request-correlation middleware
  (roadmap 4.1, opens Milestone 4): adopts a valid inbound `X-Request-ID` or generates one
  with `crypto/rand.Text`, stores it in the request context, and echoes it in the
  response. Inbound IDs are sanitized (visible-ASCII, ≤128 bytes) to prevent log/header
  injection; the first HTTP trust boundary is recorded in the threat model and compliance
  control C-2 (ADR-0013).
- `middleware.Logger` — request-logging middleware (roadmap 4.2): emits one structured
  `log/slog` line per request (method, path, status, duration, bytes-written), at a level
  derived from the status (5xx→Error, 4xx→Warn, else Info), attaching `request_id` when the
  chain seeded one. Status and byte counts are captured by a `responseRecorder` that
  implements `Unwrap`, so `http.ResponseController` still reaches the underlying Flusher /
  Hijacker. Logs the path only — never the query string, headers, or body — so secrets in
  query parameters cannot leak into log stores (extends the threat model's Info-disclosure
  row, compliance control C-2). Logged from a deferred call, so a panicking request is still
  logged before the panic propagates (ADR-0014).
- `middleware.Recoverer` — panic-recovery middleware (roadmap 4.3): recovers a panic from a
  downstream handler and writes a clean generic `500 Internal Server Error`, containing the
  failure instead of dropping the connection. The panic value and stack trace are **never**
  sent to the client (information-disclosure control C-2); they are logged server-side at
  Error level on `slog.Default` with the method, path (query string omitted), panic value,
  stack, and `request_id` when the chain seeded one. `http.ErrAbortHandler` is re-panicked
  unchanged (net/http's silent-abort sentinel), and an already-committed response is left
  intact. Reuses the Logger `responseRecorder` (Unwrap-aware, so `http.ResponseController`
  still reaches the underlying Flusher/Hijacker). Recommended chain:
  `RequestID → Logger → Recoverer → handler` (ADR-0016).
- `middleware.Cors` / `middleware.CorsConfig` — configurable CORS middleware (roadmap 4.4,
  completes Milestone 4): answers preflight `OPTIONS` requests directly with `204` and the
  negotiated `Access-Control-*` headers, and annotates cross-origin responses with
  `Access-Control-Allow-Origin` (echoing a specific allowed origin with `Vary: Origin`, or
  `*` only when credentials are off). `CorsConfig`'s zero value denies all origins (safe
  default); `AllowedMethods` defaults to `GET, HEAD, POST`, `AllowedHeaders` reflects the
  request's when empty or `*`. Two footgun misconfigurations panic at construction:
  `AllowCredentials` with a `*` origin (forbidden by the Fetch spec) and a negative `MaxAge`
  (new compliance control C-3, ADR-0017).
- `config.Load[T]` — generic configuration loader (roadmap 5.1, opens Milestone 5): decodes a
  JSON or YAML file (chosen by extension) straight into the consumer's type `T`, expands
  `${VAR}`/`$VAR` environment references before parsing (disable with `WithoutEnvExpansion`),
  and runs the decoded value's `Validate` method when it implements `Validator`. Unknown
  extensions fail with the sentinel `ErrUnsupportedFormat`. Selects and pins
  `gopkg.in/yaml.v3` as the YAML parser under ADR-0004's dependency budget — a promotion of an
  existing indirect dependency to a direct one, no new supply-chain surface (ADR-0018).
- `env.GetDefault` / `env.GetInt` / `env.GetBool` / `env.GetDuration` — typed environment
  reads with safe fallbacks (roadmap 5.2, completes Milestone 5): each returns the parsed value
  when the variable is set to a non-empty, well-formed string, and the caller's fallback for an
  unset, empty, or malformed value. No error is returned — a malformed value is treated as "not
  configured", the safe default. Complements `config.Load` for individual optional settings.
- `logger.NewStructured` — JSON structured logger tuned for aggregation (roadmap 6.1, opens
  Milestone 6): returns a `*slog.Logger` backed by slog's JSON handler (one object per line, the
  format ElasticSearch / Grafana Loki ingest), with functional options `WithWriter` (default
  `os.Stdout`), `WithLevel` (default Info, accepts a `*slog.LevelVar` for runtime adjustment),
  `WithSource`, and `WithAttrs` (base fields like service/version stamped on every record). slog's
  default `time`/`level`/`msg` keys are kept as the aggregator lingua franca; composes directly with
  `middleware.Logger` (ADR-0019).
- `logger.WithFields` / `logger.FromContext` / `logger.Field` (+ `String`/`Int`/`Bool`/`Duration`/
  `Any` constructors) — context-carried logger fields (roadmap 6.2, completes Milestone 6):
  `WithFields` attaches fields to a `context.Context`, accumulating (copy-on-write) with any set by
  an outer scope; `FromContext` returns a `*slog.Logger` derived from `slog.Default` with those
  fields applied. `Field` is a type alias for `slog.Attr`. Pairs with `NewStructured` via
  `slog.SetDefault` for structured, per-request-enriched logging (ADR-0020).
- `cache.InMemory` — generic TTL cache (roadmap 7.1, opens Milestone 7):
  `NewInMemory[K comparable, V any](ttl, opts...)` with `Get`/`Set`/`Delete`, sentinel
  `ErrNotFound`, and a `Close` that deterministically stops the cache's single cleanup goroutine
  (idempotent, goleak-verified). Expiry is enforced by `Get` against each entry's deadline, so a
  stale read is impossible no matter when the sweeper last ran — the sweeper only reclaims memory,
  on a tunable `WithCleanupInterval` (default ttl). Loud panics on non-positive ttl/interval.
  Zero-allocation hot paths (~28 ns Get-hit, ~51 ns Set on the reference box) (ADR-0021).
- `db.Transaction` — auto-rollback SQL transaction helper (roadmap 7.2, completes Milestone 7):
  runs `fn(*sql.Tx)` inside a `BeginTx`, committing when it returns nil and rolling back when it
  returns an error or panics. A panic is re-panicked after rollback (the caller's `recover` sees the
  original value); a rollback that itself fails is joined onto the returned error via `errors.Join`,
  never swallowed. The context governs both begin and the statements. Panics on a nil db or fn
  (ADR-0022).
- `validator.Struct` — tag-driven struct validation (roadmap 8.1, opens Milestone 8):
  validates a struct (or non-nil pointer to one) against `validate:"..."` tags with the rules
  `required`, `email`, `min=N`, `max=N`, and `oneof=a b c`, recursing into nested structs with
  dotted field paths and aggregating every failure into a `ValidationErrors` (an
  `errors.As`-inspectable `[]*FieldError`). Rules apply literally (no implicit optional; a pointer
  field expresses optionality). A malformed tag — an unknown rule, a rule on an incompatible type,
  a non-numeric bound — panics as the programming error it is, keeping the returned error purely
  about data. Hand-rolled reflection, no third-party dependency (ADR-0023, ADR-0004).

### Changed

- Test infrastructure (roadmap 2.6, dev-facing only — no change to the consumer surface):
  adopted the ADR-0004 test-only dependencies. The interim in-repo goroutine-leak guard
  (`internal/leakcheck`) is retired in favor of `go.uber.org/goleak`; the randomized
  completeness/delivery tests for `fanin`, `fanout`, and `pubsub` are rewritten as
  `pgregory.net/rapid` properties (automatic shrinking); assertions use
  `github.com/stretchr/testify`.

### Deprecated

### Removed

### Fixed

### Security

---

## Released versions

| Version | Date | Highlights |
|---------|------|------------|
| [v0.1.0](docs/changelog/v0/v0.1.0.md) | 2026-07-12 | M1 — project bootstrap & CI: module, quality gates, ADR-0003/0004 |
