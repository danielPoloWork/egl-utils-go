# Benchmark Report: bcrypt cost sizing (hash / verify per work factor)

- **Date:** 2026-07-26
- **Version / commit:** v1.0.0 + unreleased M10 work (branch `feat/hash-password-cost`, roadmap
  10.5; parent `master` @ `60671ed`)
- **Environment:** Intel Core i5-6600K @ 3.50GHz (4 cores), 32 GB RAM, Windows 10 Pro
  (10.0.19045), go1.26.5 windows/amd64, `golang.org/x/crypto v0.48.0`, default (release) build.
  Developer workstation — numbers are informational, not a gating baseline.
- **Command:** `go test -run '^$' -bench 'BenchmarkHashPasswordCost|BenchmarkCheckPassword|BenchmarkCost' -benchmem -count 3 ./hash/`

## Scenario

Spec v2 §7 requires the bcrypt cost factor to be "documented so deployers can size it". bcrypt is
adaptive: the work factor is a deliberate, tunable cost, and the right value is a property of the
deployment's hardware and latency budget, not something a library can pick. This report is the
evidence a deployer sizes against, and it is what `hash.HashPasswordCost` (roadmap 10.5,
[ADR-0032](../adr/0032-hash-password-cost-design.md)) exists to let them act on.

Three paths are measured across costs 10–14: hashing (`HashPasswordCost`, paid at registration and
password change), verification (`CheckPassword`, paid at **every login**), and reading the stored
factor (`Cost`, the cheap accessor that drives rehash-on-login).

The sweep stops at 14 on purpose. The doubling is exact enough that higher factors extrapolate
trivially, and a full sweep to bcrypt's ceiling of 31 would take days — see Interpretation.

## Results

Median of 3 runs; spread is min–max of the 3.

| Cost | Hash (`HashPasswordCost`) | Verify (`CheckPassword`) | Spread (hash) |
|------|---------------------------|--------------------------|---------------|
| 10 (default) | 55.46 ms/op, 5.3 kB/op, 10 allocs/op | 55.50 ms/op, 5.3 kB/op, 11 allocs/op | 55.37–55.67 ms |
| 11 | 110.67 ms/op, 5.3 kB/op, 10 allocs/op | 110.94 ms/op, 5.3 kB/op, 11 allocs/op | 110.62–110.71 ms |
| 12 | 221.52 ms/op, 5.4 kB/op, 10 allocs/op | 221.55 ms/op, 5.4 kB/op, 11 allocs/op | 221.40–222.33 ms |
| 13 | 443.26 ms/op, 5.5 kB/op, 11 allocs/op | 443.34 ms/op, 5.6 kB/op, 13 allocs/op | 442.75–444.16 ms |
| 14 | 887.04 ms/op, 5.6 kB/op, 12 allocs/op | 892.82 ms/op, 5.7 kB/op, 12 allocs/op | 885.64–887.27 ms |

| Metric | Value | Spread |
|--------|-------|--------|
| `Cost` (read the factor from a stored hash) | 112.4 ns/op, 120 B/op, 3 allocs/op | 110.6–113.4 ns/op |

## Interpretation

**The doubling is exact.** Successive ratios are 1.996, 2.002, 2.001, 2.001 — so a deployer can read
any cost off this table by doubling from the nearest measured point, and does not need a benchmark
per candidate value. Extrapolating to bcrypt's ceiling: cost 31 is 2²¹ × cost 10 ≈ **32 hours per
hash** on this hardware. The upper bound of the accepted range is a hard limit inherited from bcrypt,
**not a recommendation**; treating it as one is a denial of service against yourself.

**Verify costs the same as hash.** This is the operationally important finding, and it is why the cost
knob is a security *trade-off* rather than a free strengthening. Hashing happens at registration and
password change; verification happens on every login attempt, including failed ones, on an endpoint an
unauthenticated caller can reach. At cost 12, ~4.5 verifications/second saturate one core — five
concurrent login attempts occupy this whole 4-core box. Raising the cost hardens a leaked hash store
against offline cracking *and* multiplies the CPU an attacker can consume per request.

That is not an argument against a high cost; it is an argument for pairing it with admission control.
`ratelimit.(*Limiter).Middleware` (roadmap 10.4, ADR-0031) sheds over-budget requests before they
reach the handler, which is exactly the mitigation this amplification needs — the two milestone items
compose, and compliance control C-5 is the recorded mitigation for the C-4 cost knob.

**Allocation behaviour is irrelevant here and is reported only for completeness.** ~5.3 kB and ~10
allocations sit alongside 55 ms of deliberate CPU work; bcrypt's cost dwarfs allocator effects by five
orders of magnitude, so there is nothing to optimise and no allocation target to hold. The mild upward
drift in allocation counts with cost is benchmark-harness noise at single-digit iteration counts, not
a property of the algorithm.

**`Cost` is free.** At ~112 ns it parses the hash prefix and nothing more, so checking every stored
hash against the deployment's target on login adds no measurable latency next to the verification that
must happen anyway. Rehash-on-login has no performance excuse.

## Sizing guidance

1. Decide the verify-latency budget for a login request (a common target is ≤ 250 ms of hashing).
2. Run this benchmark on the deployment hardware; pick the highest cost that fits the budget.
3. Divide a core's throughput by the expected peak login rate to size headroom — at cost 12 on this
   box, one core serves ~4.5 logins/s.
4. Rate-limit the login endpoint so the cost cannot be weaponised.
5. Re-run at hardware refresh: the same cost gets cheaper as CPUs improve, which means the *same
   configuration silently weakens over time*. Costs should be reviewed on a schedule, and
   `hash.Cost` plus rehash-on-login is how a store is moved forward without a flag day.

## Reproduce

```bash
git checkout feat/hash-password-cost   # or master once the 10.5 PR merges
go test -run '^$' -bench 'BenchmarkHashPasswordCost|BenchmarkCheckPassword|BenchmarkCost' -benchmem -count 3 ./hash/
```

Runtime is ~70 s: the benchmark is measuring work that is expensive by design.
