# ADR-0005: workerpool design — bounded pool, blocking-first admission, loud panics

- **Status:** Accepted
- **Date:** 2026-07-12
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 1, §5 (workerpool API), §6 (verification); ROADMAP 2.1/2.6;
  patterns catalogue (Thread Pool, Functional Options); ADR-0004 (test-only deps)

## Context

Feature 1 commits to a "configurable goroutine pool with a bounded task queue" whose
`Submit` "blocks or rejects per policy when the queue is full" and whose `Stop` "drains and
joins all workers" — under the module-wide NFRs: zero goroutine leaks, race-detector clean,
allocation-conscious. The public surface was fixed at intake:
`New(workers, queueSize int, opts ...Option) *Pool`, `Submit(ctx, Task) error`,
`Stop(ctx) error`, `ErrQueueFull`. Several semantics were left open: shutdown-vs-Submit
races, which context a task receives, panic containment, and how leak-freedom is asserted
in tests while the goleak dependency (ADR-0004, test-only ring) cannot yet be added — this
environment has no Go toolchain, so a trustworthy `go.sum` cannot be produced in this PR.

## Decision

1. **Lifecycle.** A `sync.RWMutex` guards a `closed` flag: every `Submit` body runs under
   the read lock; `Stop` flips `closed` under the write lock and only then closes the queue
   channel. A send on a closed channel is therefore provably impossible, and workers drain
   with a plain `for task := range queue`. `Stop` is idempotent and concurrent-safe; every
   caller waits for the drain.
2. **Admission policy.** Default `Submit` blocks until space frees or its ctx fires
   (backpressure); `WithNonBlockingSubmit` switches to fail-fast `ErrQueueFull` — the
   spec's "blocks or rejects per policy" as a construction-time choice.
3. **Task context.** Tasks receive the pool's execution context, not the Submit context: a
   queued task may legitimately outlive the request that enqueued it. The execution context
   is canceled only when `Stop`'s deadline expires (hard stop).
4. **Panic policy.** Without a handler, a task panic propagates untouched — an unobserved
   bug stays loud (standard Go semantics). `WithPanicHandler` opts into recovery, keeping
   the worker alive and handing the recovered value to the handler.
5. **Leak assertion (interim).** Tests carry an in-repo goroutine-count leak guard with a
   stack dump on failure. ROADMAP 2.6 migrates it to goleak once `go mod tidy` can be run
   from a Go-equipped environment; hand-writing `go.sum` hashes is rejected as a
   supply-chain risk. Spec §6 wording is updated in the same PR (spec-sync rule).
6. **Pattern naming.** The taxonomy's canonical name is **Thread Pool** (Concurrency); the
   catalogue row is renamed accordingly, noting "worker pool" as the Go community name.
   **Functional Options** is a Go-idiomatic construction pattern absent from the series
   taxonomy (closest kin: Builder, Creational); it is catalogued under its
   community-canonical name, and this ADR records that deviation explicitly rather than
   hiding the idiom behind a misleading rename. The pool's internal structure is a
   Producer-Consumer buffer (taxonomy §6) — an implementation detail, not catalogued
   separately.

## Alternatives Considered

- **Atomic flag + `select` on a done channel for admission** — lock-free, but leaves an
  unavoidable race window where a Submit can enqueue after the final drain check, stranding
  an accepted task. Rejected: correctness over micro-optimization; the RWMutex is
  uncontended in the steady state.
- **Tasks receive the Submit context** — natural for request-scoped work, but a queued task
  would be canceled the moment its submitter's request ended, making the queue unreliable.
  Rejected; callers needing request-scoped cancellation can capture their ctx in the task
  closure deliberately.
- **Always recover panics (log-and-continue default)** — hides bugs in production and
  couples the pool to a logging choice. Rejected in favor of loud-by-default with opt-in
  containment.
- **Error-returning tasks (`func(ctx) error`)** — invites an error-aggregation API the spec
  does not ask for; fire-and-forget pools observe failures via the task's own instruments.
  Rejected for v1; revisitable behind a new option without breaking `Task`.

## Consequences

- Blocking `Submit` holds the read lock while waiting; `Stop` (write lock) therefore waits
  for blocked Submits to admit — their tasks run, which matches the drain contract. New
  Submits arriving after `Stop` fail with `ErrPoolClosed` (Go's RWMutex blocks new readers
  once a writer waits, so `Stop` cannot starve).
- A task that ignores its execution context can delay full shutdown past `Stop`'s deadline;
  `Stop` returns `ctx.Err()` and the worker exits when the task does. Documented as the
  task's bug.
- The interim leak guard forbids `t.Parallel()` in this package's tests (process-global
  goroutine count); the constraint disappears with goleak (2.6).
- Benchmark `BenchmarkSubmit` exists from day one; no performance claim is recorded under
  `docs/benchmarks/` until numbers from a reproducible environment exist.

## References

- `docs/specs/01_spec_utils.md` §2.1, §5, §6.
- `docs/patterns/design-patterns.md` §6 (Thread Pool, Producer-Consumer).
- uber-go/goleak (target of ROADMAP 2.6).
