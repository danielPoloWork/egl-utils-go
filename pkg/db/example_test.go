package db_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/danielPoloWork/egl-utils-go/v2/pkg/db"
)

// errInsufficientFunds stands in for a caller's own domain error — the ordinary
// reason a transaction is abandoned.
var errInsufficientFunds = errors.New("insufficient funds")

// Transaction begins, runs the callback, and finalizes: commit on nil, roll
// back on an error, roll back and re-panic on a panic. Every path out of the
// callback does exactly one of the two, which is the point — a transaction left
// open by an early return is the bug this helper removes.
//
// A real program opens its database with a driver:
//
//	sdb, err := sql.Open("pgx", dsn)
//
// These examples use an in-memory stub connector instead, because this module
// deliberately depends on no driver (ADR-0004) — see the bottom of this file.
// The stub counts commits and rollbacks, so the examples can show what the
// helper did rather than assert that it did something.
func ExampleTransaction() {
	conn := &stubConn{}
	sdb := sql.OpenDB(&stubConnector{conn: conn})
	defer func() { _ = sdb.Close() }()

	err := db.Transaction(context.Background(), sdb, func(tx *sql.Tx) error {
		// Use the tx, never the *sql.DB, inside the callback: a statement issued
		// on the DB takes a different connection and is not part of this
		// transaction, so it commits on its own and survives the rollback.
		if _, err := tx.ExecContext(context.Background(),
			"UPDATE accounts SET balance = balance - 100 WHERE id = 1"); err != nil {
			return err
		}
		_, err := tx.ExecContext(context.Background(),
			"UPDATE accounts SET balance = balance + 100 WHERE id = 2")
		return err
	})

	fmt.Println(err == nil)
	fmt.Println("commits:", conn.commits, "rollbacks:", conn.rollbacks)
	// Output:
	// true
	// commits: 1 rollbacks: 0
}

// An error from the callback rolls the transaction back and is returned
// unchanged — not wrapped — so the caller's errors.Is and errors.As keep
// working on its own domain errors.
func ExampleTransaction_rollback() {
	conn := &stubConn{}
	sdb := sql.OpenDB(&stubConnector{conn: conn})
	defer func() { _ = sdb.Close() }()

	err := db.Transaction(context.Background(), sdb, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(context.Background(),
			"UPDATE accounts SET balance = balance - 100 WHERE id = 1"); err != nil {
			return err
		}
		return errInsufficientFunds // the debit above is undone by the rollback
	})

	fmt.Println(errors.Is(err, errInsufficientFunds))
	fmt.Println("commits:", conn.commits, "rollbacks:", conn.rollbacks)
	// Output:
	// true
	// commits: 0 rollbacks: 1
}

// A panic in the callback still rolls back, and the original value is
// re-panicked so the caller's recover sees exactly what was thrown. This is the
// path a hand-written defer usually gets wrong: it is also the one where an
// open transaction holds its locks until the connection dies.
func ExampleTransaction_panic() {
	conn := &stubConn{}
	sdb := sql.OpenDB(&stubConnector{conn: conn})
	defer func() { _ = sdb.Close() }()

	func() {
		defer func() {
			fmt.Println("recovered:", recover())
			fmt.Println("commits:", conn.commits, "rollbacks:", conn.rollbacks)
		}()
		_ = db.Transaction(context.Background(), sdb, func(*sql.Tx) error {
			panic("nil pointer in the row mapper")
		})
	}()
	// Output:
	// recovered: nil pointer in the row mapper
	// commits: 0 rollbacks: 1
}

// --- the stub connector ----------------------------------------------------
//
// database/sql/driver is standard library, so a stub costs no dependency and
// keeps the examples runnable in a module that ships no driver. It is the
// smallest thing sql.OpenDB will accept: begin a transaction, execute a
// statement, commit or roll back. Nothing here is API a consumer writes — their
// *sql.DB comes from sql.Open with a real driver.

type stubConn struct {
	commits, rollbacks int
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("stub: use ExecContext")
}
func (c *stubConn) Close() error { return nil }

func (c *stubConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *stubConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &stubTx{c}, nil
}

func (c *stubConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

type stubTx struct{ c *stubConn }

func (t *stubTx) Commit() error   { t.c.commits++; return nil }
func (t *stubTx) Rollback() error { t.c.rollbacks++; return nil }

type stubConnector struct{ conn *stubConn }

func (sc *stubConnector) Connect(context.Context) (driver.Conn, error) { return sc.conn, nil }
func (sc *stubConnector) Driver() driver.Driver                        { return stubDriver{} }

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) { return nil, errors.New("stub: use sql.OpenDB") }
