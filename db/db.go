// Package db provides a transaction helper that runs a function inside a SQL
// transaction, committing on success and rolling back on error or panic.
//
// The helper exists to make the one thing that is easy to get wrong — a
// transaction leaked open because an early return or a panic skipped the
// rollback — impossible: every path out of the callback either commits or
// rolls back exactly once.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Transaction begins a transaction on db, runs fn with it, and finalizes:
//
//   - fn returns nil  → the transaction is committed; a commit error is returned.
//   - fn returns err  → the transaction is rolled back and err is returned (joined
//     with the rollback error if the rollback itself fails).
//   - fn panics       → the transaction is rolled back and the panic propagates
//     unchanged, so the caller's recover sees the original value.
//
// The context governs both the BeginTx call and, through the *sql.Tx, the
// statements fn runs; cancelling it aborts the transaction. Transaction panics
// if db or fn is nil — a wiring error, caught here rather than as an opaque nil
// dereference later (ADR-0005 idiom).
func Transaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) (err error) {
	if db == nil {
		panic("db: nil database")
	}
	if fn == nil {
		panic("db: nil function")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin transaction: %w", err)
	}

	// A panic in fn must still roll back before it unwinds past us. The rollback
	// is best-effort — the panic is the signal that matters — and the original
	// value is re-panicked so the caller's recover is unaffected.
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err = fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("db: rollback after error: %w", rbErr))
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("db: commit transaction: %w", err)
	}
	return nil
}
