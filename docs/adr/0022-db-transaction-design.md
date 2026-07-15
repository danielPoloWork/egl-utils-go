# ADR-0022: db.Transaction design — rollback on error and panic, re-panic, joined rollback errors

- **Status:** Accepted
- **Date:** 2026-07-15
- **Deciders:** Maintainer (Daniel Polo), architect agent
- **Related:** spec §2 feature 18, §5 (`Transaction(ctx, db *sql.DB, fn func(*sql.Tx) error) error`);
  ROADMAP 7.2 (completes Milestone 7); ADR-0005 (loud-by-default); ADR-0016 (Recoverer's
  rollback-then-re-panic precedent)

## Context

Feature 18 is "run SQL inside a transaction with automatic rollback on panic or error", API frozen
at intake as `Transaction(ctx, db *sql.DB, fn func(*sql.Tx) error) error`. The bug this helper
exists to kill is the leaked-open transaction: an early return or a panic in hand-written code that
skips the rollback and leaves the connection pinned. The open decisions are all about the finalize
paths — commit vs rollback, what happens to a panic, and how a rollback that *itself* fails is
reported. The correctness of the panic path is the review point flagged in the ROADMAP.

## Decision

**Three finalize paths, exactly one finalization each:**

- **`fn` returns nil → commit.** A commit error is wrapped (`db: commit transaction: %w`) and
  returned.
- **`fn` returns err → rollback, return err.** If the rollback *also* fails, the two errors are
  combined with `errors.Join(err, rollbackErr)` so neither is lost — the original cause and the
  cleanup failure are both `errors.Is`-reachable. A rollback that reports `sql.ErrTxDone` (the tx was
  already finalized, e.g. by `fn` itself) is treated as a no-op and not joined, so a
  self-managing callback does not produce spurious wrapped errors.
- **`fn` panics → rollback, re-panic.** The rollback runs in a deferred `recover`; the **original
  panic value is re-panicked unchanged** so the caller's `recover` sees exactly what `fn` threw.
  The rollback is best-effort — a failed rollback during a panic is not allowed to mask the panic —
  mirroring Recoverer's rollback-then-propagate posture (ADR-0016) and the stdlib's own
  `database/sql` example.

**The context governs begin and the statements.** `BeginTx(ctx, nil)` ties the transaction's
lifetime to `ctx`; an already-cancelled context fails the begin (wrapped `db: begin transaction: %w`,
`errors.Is(err, context.Canceled)`) before `fn` is ever called. Default `TxOptions` (nil) — isolation
level and read-only are the caller's concern and a spec-frozen signature has no room for them;
an additive `TransactionWithOptions` can arrive later.

**Loud on nil wiring** (ADR-0005): a nil `db` or nil `fn` panics with a clear message, rather than
surfacing later as an opaque nil dereference inside `database/sql`. These are programming errors,
handled the way the rest of the module handles nil dependencies (middleware, cache).

## Alternatives Considered

- **Convert a panic into an error** (recover, rollback, return an `error` wrapping the panic) — reads
  tidy, but it silently changes the caller's control flow: code that panics usually means it, and
  swallowing it here would hide bugs and defeat any outer `recover`/`Recoverer`. Rejected; roll back
  and re-panic.
- **Return only the original error, dropping a rollback failure** — simpler signature semantics, but
  a failed rollback can mean a still-open transaction or a connection problem the operator must see.
  Rejected for `errors.Join`; losing the cleanup failure is the more dangerous silence.
- **Return only the rollback error, dropping the cause** — worse: the reason the transaction failed
  is the more important half. Rejected.
- **Retry on commit failure / serialization errors** — useful for some workloads, but retry policy is
  `retry.Backoff`'s job (3.2) and composes on top; baking it in would overreach a one-shot helper.
  Rejected; compose `retry.Backoff(ctx, policy, func(ctx) error { return db.Transaction(...) })`.
- **Return an error on nil db/fn instead of panicking** — arguably friendlier, but inconsistent with
  the module's loud-by-default treatment of nil dependencies and would force every caller to handle an
  error that only a wiring bug can produce. Rejected for the panic, per ADR-0005.
- **Accept `TxOptions`** — more capable, but not the frozen signature; deferred as an additive
  constructor.

## Consequences

- A transaction is never leaked open: every path out of `fn` commits or rolls back exactly once, and
  a panic still rolls back before it unwinds.
- Error information is complete: cause preserved, commit/rollback/begin failures wrapped and (for a
  failed rollback) joined; all `errors.Is`-inspectable.
- Composes with `retry.Backoff` for retryable transactions and with `Recoverer` at the HTTP edge (a
  handler panic rolls back here, then propagates to Recoverer's clean 500).
- No new dependency; tested against a minimal in-repo `database/sql/driver` fake via `sql.OpenDB`
  (no `sqlmock` — ADR-0004 permits only testify/goleak/rapid as test deps). 100% coverage.
- **Milestone 7 (caching & data helpers) is complete.**
- Deferred, additive: `TxOptions`/isolation-level variant, a generic `Transaction[T]` returning a
  value.

## References

- `docs/specs/01_spec_utils.md` §2.18, §5.
- ADR-0005 (loud-by-default), ADR-0016 (rollback-then-re-panic precedent), ADR-0004 (test-dep budget).
- `database/sql` — `BeginTx`, `Tx.Commit`/`Rollback`, `sql.ErrTxDone`; `errors.Join`.
