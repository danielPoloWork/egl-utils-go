# Roadmap — egl-utils-go

The project's plan as a numbered, checkbox-driven list. When an item completes in a PR,
flip its checkbox (`- [ ]` → `- [x]`) **in the same PR**. New work goes at the bottom of
its section with a fresh `<milestone>.<task>` number; never renumber.

- **Versioning start:** pre-1.0 milestone-driven.
- **Session journal:** see [`docs/journal/`](docs/journal/). Latest checkpoint: _none yet_.

---

## Milestone 1 — Project bootstrap & CI

The thinnest slice that compiles, tests, and ships under the full quality bar.

- [ ] 1.1 Lay down the build system (go build (go modules)) and a buildable skeleton under
      `src/main/go/it/d4np/utils/`.
- [ ] 1.2 Wire the test framework (go test (+ testify; rapid for property tests)) with one passing smoke test under
      `src/test/go/it/d4np/utils/`.
- [ ] 1.3 Add formatter + linter configs (gofumpt (gofmt superset), golangci-lint (govet, staticcheck, errcheck, revive, gosec)) at the repo root.
- [ ] 1.4 Stand up the CI matrix (Linux / Windows / macOS on Go 1.25 & 1.26 (module floor 1.24)) with build + test + format + lint.
- [ ] 1.5 Seed the version constant (const Version = "X.Y.Z") in `version.go`.
- [ ] Record the Go module layout decision as an ADR: module path github.com/danielPoloWork/egl-utils-go, go.mod placement vs the normative src/main/go tree, consumer import ergonomics
- [ ] Record the dependency policy as an ADR: runtime = stdlib + golang.org/x/* + vetted few (prometheus/client_golang, YAML parser); test-only = testify, goleak, rapid; govulncheck as the supply-chain gate


---

## Milestone 2 — Concurrency primitives

The five channel-native concurrency building blocks, leak-free and race-clean

- [ ] 2.1 workerpool.Pool — bounded-queue goroutine pool with Submit/Stop contract (leak, race, bench coverage)
- [ ] 2.2 pubsub.Broker — filtered-subscription in-memory broker (property tests for delivery)
- [ ] 2.3 fanin.Merge — multi-channel merge (completeness property tests)
- [ ] 2.4 fanout.Split — parallel channel distribution (completeness property tests)
- [ ] 2.5 semaphore.Weighted — weighted admission wrapper over x/sync/semaphore


---

## Milestone 3 — Resilience patterns

Fail-fast, retry, and rate-limit protection for outbound calls

- [ ] 3.1 circuitbreaker.Breaker — closed/open/half-open state machine with configurable thresholds
- [ ] 3.2 retry.Backoff — exponential backoff with jitter and context cancellation (bound invariant tests)
- [ ] 3.3 ratelimit.Limiter — token bucket on Go timers (deterministic-clock tests, bench)


---

## Milestone 4 — HTTP middleware

The four production middleware, composable as a standard decorator chain

- [ ] 4.1 middleware.RequestID — extract-or-generate request ID into the context
- [ ] 4.2 middleware.Logger — request logging with duration and bytes-written stats
- [ ] 4.3 middleware.Recoverer — panic recovery with clean 500 responses
- [ ] 4.4 middleware.Cors — configurable CORS header handling


---

## Milestone 5 — Configuration & environment

Safe configuration ingestion from files and environment

- [ ] 5.1 config.Loader — JSON/YAML/env loading with validation hooks
- [ ] 5.2 env.GetDefault — typed env reads with safe fallbacks


---

## Milestone 6 — Structured logging

JSON logging wired for aggregation and context propagation

- [ ] 6.1 logger.Structured — JSON logger for ElasticSearch / Loki ingestion
- [ ] 6.2 logger.Context — logger fields carried in context.Context


---

## Milestone 7 — Caching & data helpers

TTL caching and transactional SQL ergonomics

- [ ] 7.1 cache.InMemory — TTL cache with periodic cleanup goroutine (leak-checked, bench)
- [ ] 7.2 db.Transaction — auto-rollback transaction helper (panic-path tests)


---

## Milestone 8 — Validation & security

Tag-driven validation and password hashing

- [ ] 8.1 validator.Struct — tag-driven struct validation (required, email, min, max, oneof)
- [ ] 8.2 hash.HashPassword / hash.CheckPassword — bcrypt hashing and verification


---

## Milestone 9 — Diagnostics & lifecycle

Graceful shutdown, health, metrics, and the core utility pair

- [ ] 9.1 lifecycle.GracefulShutdown — signal-coordinated ordered shutdown (SIGINT/SIGTERM)
- [ ] 9.2 health.Handler — dependency-probing health endpoint
- [ ] 9.3 metrics.Prometheus — latency/request-count middleware with Prometheus exposition
- [ ] 9.4 syncpool.BufferPool — bytes.Buffer pooling (zero steady-state allocations, bench)
- [ ] 9.5 errors.Wrap — stack-preserving error context helpers



---

## Spec Coverage Map

Tracks which spec section is fulfilled by which roadmap item(s). Every spec section has a
row with at least one fulfilling item and a status glyph. Legend: ⏳ not started · 🚧 in
progress · ✅ done · ❎ N/A.

| Spec § | Requirement | Roadmap items | Status |
|--------|-------------|---------------|--------|
| §1 | Objective & business context | 1.1 | ⏳ |
| §2 | Functional requirements | 1.1, 1.2 | ⏳ |
| §3 | Non-functional requirements | 1.3, 1.4 | ⏳ |
| §4 | Logical architecture | 1.1 | ⏳ |
| §5 | Public interface | 1.2 | ⏳ |
| §6 | Verification & test strategy | 1.2, 1.4 | ⏳ |
