package db_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/danielPoloWork/egl-utils-go/db"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// --- a minimal in-memory database/sql driver -------------------------------
//
// Built on sql.OpenDB + a driver.Connector so there is no global sql.Register
// (no name collisions across tests) and no third-party mock — ADR-0004 permits
// only testify/goleak/rapid as test dependencies. Each field injects a failure
// at one step; the counters record how the transaction was finalized.

type fakeConn struct {
	beginErr, commitErr, rollbackErr, execErr error
	begins, commits, rollbacks, execs         int
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fakeConn: Prepare unsupported")
}
func (c *fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.begins++
	if c.beginErr != nil {
		return nil, c.beginErr
	}
	return &fakeTx{c}, nil
}

func (c *fakeConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.execs++
	if c.execErr != nil {
		return nil, c.execErr
	}
	return driver.RowsAffected(0), nil
}

type fakeTx struct{ c *fakeConn }

func (t *fakeTx) Commit() error   { t.c.commits++; return t.c.commitErr }
func (t *fakeTx) Rollback() error { t.c.rollbacks++; return t.c.rollbackErr }

type fakeConnector struct{ c *fakeConn }

func (fc *fakeConnector) Connect(context.Context) (driver.Conn, error) { return fc.c, nil }
func (fc *fakeConnector) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use OpenDB") }

func newDB(conn *fakeConn) *sql.DB { return sql.OpenDB(&fakeConnector{c: conn}) }

// closeDB closes sdb and asserts it succeeded. Deferred after goleak.VerifyNone
// so it runs first (defers are LIFO) — otherwise goleak would see sql's
// still-live connection-opener goroutine.
func closeDB(t *testing.T, sdb *sql.DB) {
	t.Helper()
	require.NoError(t, sdb.Close())
}

// --- tests -----------------------------------------------------------------

func TestTransactionCommitsOnSuccess(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	err := db.Transaction(context.Background(), sdb, func(*sql.Tx) error { return nil })
	require.NoError(t, err)
	require.Equal(t, 1, conn.commits)
	require.Equal(t, 0, conn.rollbacks)
}

func TestTransactionRollsBackOnError(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	errBoom := errors.New("fn failed")
	err := db.Transaction(context.Background(), sdb, func(*sql.Tx) error { return errBoom })
	require.ErrorIs(t, err, errBoom)
	require.Equal(t, 0, conn.commits)
	require.Equal(t, 1, conn.rollbacks)
}

func TestTransactionRollsBackAndRepanicsOnPanic(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	require.PanicsWithValue(t, "boom", func() {
		_ = db.Transaction(context.Background(), sdb, func(*sql.Tx) error {
			panic("boom")
		})
	})
	require.Equal(t, 1, conn.rollbacks, "a panicking transaction must still roll back")
	require.Equal(t, 0, conn.commits)
}

func TestTransactionBeginError(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{beginErr: errors.New("connect refused")}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	called := false
	err := db.Transaction(context.Background(), sdb, func(*sql.Tx) error {
		called = true
		return nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "begin transaction")
	require.False(t, called, "fn must not run when the transaction cannot begin")
	require.Equal(t, 0, conn.commits)
	require.Equal(t, 0, conn.rollbacks)
}

func TestTransactionCommitError(t *testing.T) {
	defer goleak.VerifyNone(t)
	errCommit := errors.New("commit failed")
	conn := &fakeConn{commitErr: errCommit}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	err := db.Transaction(context.Background(), sdb, func(*sql.Tx) error { return nil })
	require.ErrorIs(t, err, errCommit)
	require.Contains(t, err.Error(), "commit transaction")
}

func TestTransactionJoinsRollbackError(t *testing.T) {
	defer goleak.VerifyNone(t)
	errFn := errors.New("fn failed")
	errRollback := errors.New("rollback failed")
	conn := &fakeConn{rollbackErr: errRollback}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	err := db.Transaction(context.Background(), sdb, func(*sql.Tx) error { return errFn })
	require.ErrorIs(t, err, errFn, "the original error is preserved")
	require.ErrorIs(t, err, errRollback, "the rollback failure is joined, not swallowed")
}

func TestTransactionRunsStatement(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	err := db.Transaction(context.Background(), sdb, func(tx *sql.Tx) error {
		_, execErr := tx.ExecContext(context.Background(), "UPDATE t SET x = 1")
		return execErr
	})
	require.NoError(t, err)
	require.Equal(t, 1, conn.execs, "the statement ran on the transaction")
	require.Equal(t, 1, conn.commits)
}

func TestTransactionContextCancelled(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before we begin

	called := false
	err := db.Transaction(ctx, sdb, func(*sql.Tx) error {
		called = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, called, "fn must not run when the context is already cancelled")
	require.Equal(t, 0, conn.begins, "the driver is never reached")
}

func TestTransactionNilDBPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	require.PanicsWithValue(t, "db: nil database", func() {
		_ = db.Transaction(context.Background(), nil, func(*sql.Tx) error { return nil })
	})
}

func TestTransactionNilFnPanics(t *testing.T) {
	defer goleak.VerifyNone(t)
	conn := &fakeConn{}
	sdb := newDB(conn)
	defer closeDB(t, sdb)

	require.PanicsWithValue(t, "db: nil function", func() {
		_ = db.Transaction(context.Background(), sdb, nil)
	})
}
