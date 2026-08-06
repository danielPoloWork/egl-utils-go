# 2026-08-06 — 14.8: the job was already running, and the number it published was not a latency

Milestone 14 reaches **8/12**. The roadmap line for 14.8 was two sentences and both of its premises were
false. The first cost nothing to correct. The second took NFR-06's verdict with it.

## What shipped

- **NFR-02's `Submit` p99: 176 ns against a 2 µs target — met, by 11×.** Conservatively, for a reason
  measured below.
- **NFR-06's p99: 887 ns against a 200 ns target — not met**, with oversubscription removed entirely.
  The expected story was "the runner is short four cores"; the numbers do not support it.
- **`BenchmarkNFR06GetTailPerCore`** — the same 90/10 mix at exactly one goroutine per core, because a
  wall-clock batch timed inside one of eight goroutines on four cores does not measure what it looks
  like it measures.
- **`nfr-nightly.yml` prints the tail percentiles to the run summary *and* the job log**, and warns when
  a tail refuses to publish on a runner whose clock should manage it.
- **[ADR-0037](../../../adr/0037-nfr-benchmark-methodology.md) amended three times** and the
  [2026-07-26 report](../../../benchmarks/2026-07-26-nfr-suite.md) updated in place, the way 10.11 and
  ADR-0044 did — before and after in one document.
- **A spec question handed to the maintainer rather than answered**: whether NFR-06's 200 ns means the
  uncontended `Get` (32.9 ns, met) or the loaded one (~775 ns, not met).

## No job needed adding

The roadmap said "Linux CI has a usable clock, as it did for `-race` and `-fuzz`. Add the job."

`nfr-nightly.yml` has run the whole NFR suite on `ubuntu-24.04` at `-count 10` since 10.10 built it.
Both tail benchmarks were in it. `clock-res-ns` came back **29–30**, `observed-tick-ns` **10–20**, and
the percentiles were published on every run. Last night's was the eleventh consecutive green one.

Nobody had opened one. The numbers went into the `nfr-results-<run-id>` artifact and the job summary
carried only the benchstat table, so the measurement existed for eleven nights and the *result* did not.
That distinction is the one worth carrying out of this item, and it is not specific to benchmarks:

> **A job that runs is not a number that has been read.**

It is 14.7's failure one layer further out. There a gate ran and policed the wrong subject; here a job
ran, measured the right thing, and wrote the answer somewhere with no reader. Both pass every "is CI
green?" test that anyone actually performs.

The fix is a `grep` into `$GITHUB_STEP_SUMMARY` — and into the step's stdout as well, which is the part
worth noting. A summary is not reachable from `gh run view --log`; writing the numbers only there would
have been the same mistake in a smaller room. Both destinations, deliberately.

There is also a warning now for a tail coming back `tail-unmeasurable` on Linux. Deliberately not a perf
gate — ADR-0037 is explicit that a percentile from a shared runner must never block a merge — but a
*refusal to publish* is a different event from a slow number. It means the measurement has gone blind: a
coarsened clock, or a batch too short for it. That is hardware-independent in kind, which is the category
ADR-0037 permits signalling. Both branches were checked against the real artifact, the negative on the
run's own output and the positive on a doctored copy, because a `grep` that has only ever been observed
not to fire is not evidence of anything.

## The number that was not a latency

NFR-06 asks for a `Get` p99 ≤ 200 ns at 1 M entries under a 90/10 mix across eight goroutines. The
nightly reports, on one line:

```
BenchmarkNFR06GetTail-4   12364836   96.35 ns/op   30.00 clock-res-ns   8.000 goroutines
        61.00 observed-tick-ns   737.6 p50-ns/op   1575 p99-ns/op   4.000 procs   0 allocs/op
```

Read carelessly: a p99 of 1604 ns (median of ten) against a 200 ns target, on a cache 10.11 had just
sharded to fix exactly this. Publishing it would have been the Windows-tick mistake in a new costume — a
number that looks like data.

The same line refutes it. The aggregate is **97.1 ns/op** and the p50 batch mean is **743 ns/op**, and
those are two measurements of the same work. 97.1 × 8 = 777, with the p50 sitting a little under a mean
the 1604 p99 pulls upward. The factor is the goroutine count.

The reason is arithmetic, not noise. Over wall time *T* with *N* goroutines each looping, every one
completes *T/L* operations, so the aggregate `ns/op` the framework reports is *L/N*. Invert it: the wall
time an operation occupies from its caller's view is the aggregate times the goroutine count. When
*N > M* cores, part of that *L* is the goroutine sitting runnable-but-descheduled. A batch timed with
`time.Now()` from *inside* one of those goroutines measures **residency**; `ns/op` measures aggregate
throughput across all four cores. They differ by 8×, and neither is wrong.

So two corrections rather than one caveat. The 8-goroutine number is published **as a residency time**,
with the arithmetic printed next to it. And `BenchmarkNFR06GetTailPerCore` runs the identical mix at
*N = M*, where a batch mean is a service time again — verified by the same identity at a second point:
117.3 × 4 = 469 against a measured p50 of 455.

## Where the expected answer died

The diagnostic was built expecting it to say "the code is fine, the runner is short four cores". It says
the opposite:

| | goroutines | aggregate ns/op | p50 ns/op | p99 ns/op |
|---|---|---|---|---|
| `GetTail` | 8 | 97.1 | 743.5 | 1604 |
| `GetTailPerCore` | 4 | 117.3 | 455.4 | **886.6** |

**887 ns at the p99 with no oversubscription at all.** Scheduling removed, four goroutines on four
cores, and the shortfall against 200 ns is still 4.4×. An 8-core reference machine does not close that;
it is not a hardware gap.

One detail in that table is worth its own sentence: **four goroutines measured *worse* aggregate
throughput than eight** — 117.3 ns/op against 96.4. Not noise. Random access across 1 M entries stalls
on memory, and oversubscribing the cores hides those stalls behind other goroutines' work. Throughput
improves while per-operation latency degrades, in the same benchmark, from the same change. It is the
clearest illustration I have of why the two numbers must never be substituted for one another.

Which is exactly what had happened. **10.11's recorded verdict — "NFR-06 met at the mean, 46.6 ns" —
compares an aggregate throughput figure against a latency target.** Every parallel benchmark here
reports ns/op that way. At eight goroutines a 90/10 mix measuring ~96 ns/op has each individual `Get`
occupying ~775 ns of its caller's wall clock. ADR-0038's sharding result is untouched: 7.5× more
throughput on the contended path is 7.5× more throughput, and the Get-only/mixed convergence still shows
write contention stopped being the bottleneck. What does not survive is reading that number as the
latency NFR-06 asks about.

That is flagged and not fixed, because the live question is a spec one and not a measurement one.
`GetHit` uncontended is **32.9 ns**, a genuine latency, comfortably inside 200 ns. The same operation
under an 8-way 90/10 mix costs ~775 ns. If NFR-06 means the first, it is met and the suite has been
measuring the wrong quantity for a month; if it means the second, it is unreachable on any hardware this
project has measured and the target needs the treatment NFR-01's 0-allocs target got in ADR-0030 §3.
Handed to the maintainer.

## NFR-02 is met, and the first explanation of why was wrong

`SubmitP99` reports a p99 of **176 ns** against 2 µs. Met by 11×, with the clock four orders of
magnitude finer than the samples.

The benchmark's doc comment claimed it measured `Submit` "uncontended: one submitting goroutine, a queue
deep enough that Submit never waits on a worker". That is false, and the first attempt to show it was
also false — worth recording, because the second attempt is the one that generalises.

**The wrong argument.** The nightly showed a p50 of 67.5 ns/op against a 107 ns/op mean, which reads
beautifully: 67.5 is the unblocked `Submit`, the gap is back-pressure. Then the branch run measured a
p50 of **111.4** — above its own mean — on the same commit and the same image. `Submit`'s *distribution*
varies by runner instance, not merely its centre. A decomposition that flips sign on a second sample is
not a finding.

**The argument that holds** needed no new run:

| | queue | ns/op |
|---|---|---|
| `NFR02Throughput` (empty task) | 8192 | 106.0 |
| `NFR02ThroughputCounted` (atomic per task) | 8192 | 144.2 |
| `NFR02SubmitP99` | 65536 | 107.1 |

Moving an atomic increment *inside the task body* slows **submission** by 38 ns/op. A producer that
never waited on the queue could not be slowed by what the consumers do with what it queued — so the
queue is full and `Submit` is blocking on a worker. Queue depth confirms it from the other side: 8× the
room, the same 107 ns/op, so depth is not what binds; the drain rate of eight workers on one shared
channel is.

Which makes the verdict better than it looks rather than worse. The NFR asks about the *uncontended*
tail, and the measured 176 ns has contention folded in — an upper bound on the thing being claimed. The
batch-mean dilution pulls the other way, and 11× of margin means neither correction is load-bearing.

## What is not done

- **The NFR-06 target question**, above, is the maintainer's.
- **NFR-06's p99 at eight *concurrent* goroutines** still wants eight cores, but it is no longer the
  blocker it looked like: the 4-core service time already fails the target, so the 8-core number would
  refine a verdict rather than produce one.
- **The batch-mean dilution** is unchanged and remains stated: per-operation sampling needs a
  `time.Now()` cheaper than the operation being timed, and at 30–100 ns per operation it is not, on any
  platform. The tails are lower bounds by construction — which for NFR-06 means 887 ns is the
  *optimistic* reading.
