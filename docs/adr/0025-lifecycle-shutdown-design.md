# ADR-0025: lifecycle.GracefulShutdown design — LIFO hooks, exactly-once convergent Shutdown, no hidden timeout

- **Status:** Accepted — the **"no hidden timeout"** decision and the rejected default-timeout
  alternative are superseded by [ADR-0051](0051-lifecycle-shutdown-timeout.md):
  `WaitForSignals` takes a `timeout` as its first parameter. **The reasoning below is preserved
  rather than replaced** — a library must not invent a deadline the operator did not set, which is
  why the timeout is a required parameter and why `0` still means no deadline at all. Everything
  else here stands: LIFO ordering, sequential hooks, run-every-hook with `errors.Join`,
  exactly-once convergent `Shutdown`, the loud panics, zero owned goroutines, and the
  singleton-with-seams testability pattern.
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §1 (zero goroutine leaks), §2 feature 21, §3 (the intake's usage example), §5
  (`Register(fn func(ctx) error)`, `WaitForSignals(sig ...os.Signal)`, `Shutdown(ctx) error`);
  ROADMAP 9.1 (opens Milestone 9); ADR-0005 (loud-by-default); ADR-0016/0020 (`slog.Default`
  precedent); ADR-0006 (graceful-shutdown posture)

## Context

Feature 21 coordinates "the ordered shutdown of HTTP servers, databases, and queues on termination
signals (SIGINT/SIGTERM)", with three **package-level** functions frozen at intake (the spec's own
§3 example calls `lifecycle.Register(...)` / `lifecycle.WaitForSignals(...)` directly). The open
decisions: what "ordered" means, what happens when hooks fail or Shutdown is called twice (or
concurrently — a signal racing a programmatic shutdown is the realistic case), whether a timeout is
imposed, how signals are handled portably (Windows has no `kill(2)`), and how a package-level
singleton stays testable.

## Decision

**Hooks run in reverse registration order (LIFO).** Wiring order is dependency order — a database
is created before the HTTP server that uses it — so shutdown must be the reverse: stop accepting
traffic first, close the storage it depended on last. LIFO is exactly Go's `defer` intuition,
applied at process scope. Hooks run **sequentially** (the spec's word is *ordered*; parallel
shutdown would forfeit the dependency guarantee that is this feature's point).

**Every hook runs; errors are joined.** A failing HTTP-server shutdown must not leave the database
connection pool dangling: a hook's error is collected, the remaining hooks still run, and `Shutdown`
returns the `errors.Join` of every failure (nil when all succeed). Cancelling the context does not
skip hooks either — each hook receives the cancelled context and decides for itself how to abort
(that context reaches e.g. `http.Server.Shutdown`, which honors it natively).

**Exactly-once, convergent Shutdown.** The first `Shutdown` call runs the hooks; every later or
*concurrent* call — a SIGTERM arriving while a programmatic shutdown is mid-flight — blocks until
that first run completes and returns **its** result. Implemented with a mutex-guarded `started`
flag plus a `finished` channel whose close publishes the result (write-before-close gives the
happens-before edge); no hook can ever run twice, and no caller can observe a half-finished result.

**Register after shutdown panics.** A hook registered after shutdown began would silently never run
— a wiring error, made loud per ADR-0005. A nil hook panics likewise.

**`WaitForSignals` blocks in place — the package owns zero goroutines.** It subscribes the given
signals (defaulting to `os.Interrupt` + `syscall.SIGTERM` when called with none — the pair the
spec's example names), blocks receiving on the channel, then runs `Shutdown(context.Background())`
and logs any joined error at Error on `slog.Default` (the ADR-0016 wiring) before returning. There
is no watcher goroutine to leak (spec §1). On Windows, `syscall.SIGTERM` exists and is accepted but
never delivered; only Interrupt (Ctrl+C) fires — documented, and the subscription is harmless.

**No hidden timeout.** `WaitForSignals` gives hooks a background context: the platform's own kill
escalation (systemd's `TimeoutStopSec`, Kubernetes' grace period, then SIGKILL) is the real upper
bound, and a library-invented deadline would silently truncate shutdowns that the operator
configured the platform to allow. A consumer wanting its own bound calls `Shutdown` with a deadline
context (e.g. via `signal.NotifyContext`) instead.

**A documented singleton, testable via seams.** The frozen API is package-level, so the state is a
package-level `coordinator` instance (the `slog.Default` shape). Two internal seams keep it fully
testable without global fallout: tests swap the coordinator (fresh instance per test), and
`signal.Notify`/`signal.Stop` are injectable variables, so signal delivery is faked deterministically
— no real process signals (impossible portably: Windows has no `kill(2)`), and no `os/signal`
process-wide goroutine ever starts under test (goleak stays exact).

## Alternatives Considered

- **FIFO hook order** — registration order feels "natural" until the first dependency inversion
  (closing the DB while the server still handles requests). Rejected for LIFO.
- **Parallel hook execution** — faster shutdown, but forfeits ordering, which is the feature's
  contract ("ordered shutdown"). Rejected; a consumer can parallelize inside one hook.
- **Fail-fast on the first hook error** — leaves every later resource unreleased on the first
  hiccup. Rejected; run all, join errors.
- **A default shutdown timeout in WaitForSignals** — protects against a hung hook, but invents a
  deadline the operator didn't set and the platform already enforces one level up. Rejected;
  documented escape hatch (deadline context + `Shutdown`).
  > *Annotated 2026-07-29 (ADR-0051): **a timeout was subsequently added, and this entry names
  > precisely why it took a major to do it.*** The objection here is to a **default** — a deadline
  > "the operator didn't set". ADR-0051 does not add one: the timeout is a **required first
  > parameter**, so there is nothing to inherit by accident, and **`0` still means no deadline**,
  > which keeps the platform-escalation posture this entry defends available and documented as the
  > right choice under systemd or Kubernetes. So the conclusion reversed while the reason held —
  > and the reason is what shaped the replacement. Adding it in v1 would have required either a
  > default (this objection) or a second function (rejected in ADR-0048's terms), which is exactly
  > why it waited for a signature change.
- **An exported `Coordinator` struct** — better isolation than a singleton, but not the frozen API.
  Additive later if multi-coordinator scenarios appear; the internal type already exists.
- **Force-exit on a second signal** (the "impatient Ctrl+C" convention) — useful interactively, but
  `os.Exit` from library code bypasses every remaining hook; deferred as an additive option rather
  than default behaviour.
- **Allowing (ignoring) Register after shutdown** — quiet, and quietly wrong: the component believes
  its cleanup is scheduled. Rejected for the loud panic.

## Consequences

- Shutdown is deterministic: reverse-dependency order, all hooks, exactly once, one converged
  result — under signals, programmatic calls, or both racing.
- The package owns no goroutines (blocking receive), keeping the module's zero-leak invariant
  trivially true; goleak gates it. *(Amended in roadmap 10.3 — ADR-0030: `WaitForSignals` now blocks
  in a `select` over the signal channel and the `Trigger` channel. Still a blocking wait in the
  caller's goroutine, so the no-goroutines property is unchanged.)*
- Signal behaviour is portable and honest about Windows (Interrupt only), and tests are fully
  deterministic on every platform via the injected signal seam.
- Milestone 9 opens; health (9.2), metrics (9.3), syncpool (9.4), and errors (9.5) complete it.
- Deferred, additive: an exported `Coordinator`, force-exit-on-second-signal, per-hook timeouts.
  *(Roadmap 10.3 adds `Trigger()` — a programmatic wake for a pending `WaitForSignals`, latched in a
  once-closed channel scoped to the coordinator — without exporting the coordinator itself.)*

## References

- `docs/specs/01_spec_utils.md` §2.21, §3 (usage example), §5.
- ADR-0005 (loud-by-default), ADR-0016/0020 (`slog.Default`), ADR-0006 (graceful posture).
- `os/signal` — `Notify`/`Stop`, Windows signal semantics; `errors.Join`.
