# 2026-07-26 — Milestone 10.4: ratelimit.Middleware() + ErrLimited

## What got done

- **Roadmap 10.4 `ratelimit.(*Limiter).Middleware()` + `ratelimit.ErrLimited`** (branch
  `feat/ratelimit-middleware`): the HTTP ergonomics spec v2 item 8 asks for, adopted additively per
  ADR-0030. The token-bucket engine ADR-0012 froze is untouched; the new file is
  `ratelimit/middleware.go`. **New [ADR-0031](../../../adr/0031-ratelimit-middleware-design.md)** —
  the first ADR of Milestone 10 (10.2 and 10.3 rode on ADR-0030).
- **Placement resolved against the layered import graph.** Spec §3 puts `ratelimit` at L2 and
  `middleware.*` at L3 with arrows pointing down only, which reads like an argument for
  `middleware.RateLimit(l)`. Rejected: spec §5 names the method *on the Limiter*, and the layer rule
  governs intra-module imports — `net/http` is stdlib, so an L2 package importing it takes on no
  ADR-0004 dependency budget. The alternative would also have made `middleware` the only package
  importing a sibling feature package, an exception 10.8's depguard rules would have to carve out.
- **`Allow`, never `Wait` — the design crux.** Both admission modes can front an HTTP handler, but
  blocking is a denial-of-service foot-gun: each parked request holds a goroutine and its connection
  for the wait, so an over-budget burst converts into an unbounded queue and the failure mode becomes
  resource exhaustion instead of a clean refusal. Shedding load is what a limiter is *for*, so the
  middleware sheds. A consumer that genuinely wants queueing calls `Wait` with a deadline in its own
  handler.
- **Three smaller decisions, each with a security edge.** (1) `Retry-After` is the constant
  `ceil(1/rate)` — the worst-case wait for one token — precomputed once per decorator, deliberately
  *not* the tighter `(1-tokens)/rate`: that would need the lock on the deny path and would leak the
  live bucket level to an untrusted client, and RFC 9110 requires whole `delta-seconds` anyway.
  (2) The body is the generic status text; nothing about the limiter's configuration or level is
  disclosed (same posture as `Recoverer`/`health.Handler`). (3) **Denials are not logged.** Unlike
  `Recoverer`, which logs a genuine bug, a denial is a normal operating condition — and a
  client-triggerable log line is a flooding amplifier, letting the very traffic being limited inflate
  log volume and cost. They are already observable as ordinary 429s to `middleware.Logger`, which is
  the composition this decorator family exists for.
- **`ErrLimited` given a real job rather than being decoration.** An `http.Handler` cannot return an
  error, so the sentinel is defined as the canonical name for the fail-fast admission condition: what
  a consumer gating its own work on `Allow` returns so *its* callers can `errors.Is` it, while the
  middleware expresses the same condition in the only vocabulary HTTP has.
- **The honest limitation, stated rather than papered over:** one limiter bounds *total* throughput,
  not any one client's share, so a heavy caller can drain the budget for everyone. Per-key limiting
  needs a key-extraction policy (whether `X-Forwarded-For` is trustworthy depends on the consumer's
  proxy topology) and an eviction policy for idle keys; presuming either would silently create an
  IP-spoofing bypass or a memory leak. Documented in the godoc, ADR-0031, and a threat-model row, and
  carried as an accepted partial in new **control C-5**.
- Tests: package coverage **97.7% → 98.1%** (every new statement covered; the residual is a
  pre-existing defensive branch in `wait`'s production path). External tests cover admit-within-burst,
  refusal, the `Retry-After` table (including the `1/0.3 → 4` rounding case), header absence on the
  admit path, two decorators sharing one bucket, request/writer pass-through, and the sentinel's
  message. An **internal** test drives refill on the existing fake clock — refused at the same
  instant, still refused one millisecond short of a full token, admitted exactly at it — so
  re-admission cannot come from the wall clock. `testing.AllocsPerRun` asserts the zero-allocation
  admit path directly rather than trusting the benchmark. goleak throughout.
- Benchmarks: admit **~30 ns/op, 0 allocs/op**; refuse ~386 ns/op, 4 allocs (`http.Error`'s header
  write and body format), paid only by refused requests. The refuse benchmark initially read
  ~1394 ns/12 allocs because it built an `httptest.NewRecorder()` per iteration — replaced with a
  discarding `http.ResponseWriter` so the number measures the middleware, not the recorder.
- Local gauntlet green (portable Go 1.26.5): build, `go vet ./...`, full `go test ./...`, gofumpt
  clean, golangci-lint v2 0 issues, govulncheck 0 affecting, `consistency_lint.py` OK. `-race` is
  CI-only locally.

## Where the project stands

v1.0.0 shipped. **Milestone 10 in progress (4 of 13)**: 10.1 (#37), 10.2 (#38), and 10.3 (#46)
merged; 10.4 drafted on `feat/ratelimit-middleware`, awaiting the maintainer to open and merge. M10
releases as v1.1.0.

## How the next session resumes

Wait for the 10.4 PR to merge. Then **10.5 `hash.HashPasswordCost`** — the security-relevant item of
the milestone: an explicit-cost sibling of `HashPassword` accepting bcrypt cost 10–31 (reject below
10, and note that bcrypt's own ceiling is 31), plus an argon2id migration note in the godoc and a
cost-sizing benchmark so operators can pick a cost against their own hardware budget. It extends
ADR-0024 and control C-4, so it needs the security-auditor pass and a threat-model/compliance update,
not just code. The v2 default-cost bump 10→12 stays in the `/v2` ledger (documented behaviour, hence
breaking per ADR-0030) — `HashPasswordCost` is the capability that makes it reachable in v1.
Standard footprint per PR (tests + goleak + coverage, CHANGELOG `[Unreleased]`, ROADMAP checkbox,
journal, lint). Portable Go under `%TEMP%\go-portable` — in the Bash tool add it as the *unix* path
`/c/Users/work/AppData/Local/Temp/go-portable/go/bin`; golangci-lint needs the `/v2` module path;
`-race` is CI-only.
