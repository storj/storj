// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"database/sql/driver"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

// stubConn implements connAll so it can be wrapped by tidbConn. Every method
// is a no-op except ExecContext/QueryContext, which dispatch to the per-call
// functions configured by the test.
type stubConn struct {
	execFn  func(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error)
	queryFn func(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error)

	execCalls  atomic.Int32
	queryCalls atomic.Int32
}

func (s *stubConn) Close() error              { return nil }
func (s *stubConn) Begin() (driver.Tx, error) { return stubTx{}, nil }
func (s *stubConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return stubTx{}, nil
}
func (s *stubConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (s *stubConn) PrepareContext(context.Context, string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (s *stubConn) Ping(context.Context) error               { return nil }
func (s *stubConn) CheckNamedValue(*driver.NamedValue) error { return nil }
func (s *stubConn) ResetSession(context.Context) error       { return nil }
func (s *stubConn) IsValid() bool                            { return true }

func (s *stubConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	s.execCalls.Add(1)
	if s.execFn == nil {
		return driver.RowsAffected(0), nil
	}
	return s.execFn(ctx, query, args)
}

func (s *stubConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	s.queryCalls.Add(1)
	if s.queryFn == nil {
		return nil, nil
	}
	return s.queryFn(ctx, query, args)
}

type stubTx struct{}

func (stubTx) Commit() error   { return nil }
func (stubTx) Rollback() error { return nil }

// TestTidbConn_RetryOnConflict verifies that ExecContext and QueryContext
// retry on TiDB conflict codes when no transaction is active, and that the
// underlying call observes the retry.
func TestTidbConn_RetryOnConflict(t *testing.T) {
	for _, code := range []uint16{1213, 8022, 9007} {
		t.Run("exec_"+strconv.Itoa(int(code)), func(t *testing.T) {
			stub := &stubConn{}
			stub.execFn = func(context.Context, string, []driver.NamedValue) (driver.Result, error) {
				if stub.execCalls.Load() == 1 {
					return nil, &mysql.MySQLError{Number: code}
				}
				return driver.RowsAffected(1), nil
			}

			conn := &tidbConn{underlying: stub}
			res, err := conn.ExecContext(context.Background(), "DELETE FROM t WHERE id = ?", nil)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Equal(t, int32(2), stub.execCalls.Load())
		})

		t.Run("query_"+strconv.Itoa(int(code)), func(t *testing.T) {
			stub := &stubConn{}
			stub.queryFn = func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
				if stub.queryCalls.Load() == 1 {
					return nil, &mysql.MySQLError{Number: code}
				}
				return nil, nil
			}

			conn := &tidbConn{underlying: stub}
			_, err := conn.QueryContext(context.Background(), "SELECT 1", nil)
			require.NoError(t, err)
			require.Equal(t, int32(2), stub.queryCalls.Load())
		})
	}
}

// TestTidbConn_NoRetryInTransaction verifies that once a transaction is
// active on the conn, conflict errors propagate to the caller untouched —
// retries inside a tx are the caller's job (txutil.WithTx) so business logic
// can be reapplied.
func TestTidbConn_NoRetryInTransaction(t *testing.T) {
	stub := &stubConn{}
	stub.execFn = func(context.Context, string, []driver.NamedValue) (driver.Result, error) {
		return nil, &mysql.MySQLError{Number: 9007}
	}

	conn := &tidbConn{underlying: stub}
	_, err := conn.BeginTx(context.Background(), driver.TxOptions{})
	require.NoError(t, err)

	_, err = conn.ExecContext(context.Background(), "UPDATE t SET v=v+1", nil)
	var mysqlErr *mysql.MySQLError
	require.True(t, errors.As(err, &mysqlErr))
	require.Equal(t, uint16(9007), mysqlErr.Number)
	require.Equal(t, int32(1), stub.execCalls.Load(), "must not retry inside transaction")
}

// TestTidbConn_NoRetryOnNonRetryable confirms that errors outside the
// conflict set propagate immediately.
func TestTidbConn_NoRetryOnNonRetryable(t *testing.T) {
	stub := &stubConn{}
	stub.execFn = func(context.Context, string, []driver.NamedValue) (driver.Result, error) {
		// 1062 (duplicate key) is not in retrydb's conflict set.
		return nil, &mysql.MySQLError{Number: 1062}
	}

	conn := &tidbConn{underlying: stub}
	_, err := conn.ExecContext(context.Background(), "INSERT INTO t VALUES (1)", nil)
	require.Error(t, err)
	require.Equal(t, int32(1), stub.execCalls.Load(), "must not retry non-retryable errors")
}

// TestTidbTx_ClearsInTransaction confirms Commit and Rollback clear the
// inTransaction flag so subsequent single-statement calls regain retry
// semantics.
func TestTidbTx_ClearsInTransaction(t *testing.T) {
	for _, op := range []struct {
		name string
		fn   func(driver.Tx) error
	}{
		{"commit", func(tx driver.Tx) error { return tx.Commit() }},
		{"rollback", func(tx driver.Tx) error { return tx.Rollback() }},
	} {
		t.Run(op.name, func(t *testing.T) {
			stub := &stubConn{}
			conn := &tidbConn{underlying: stub}

			tx, err := conn.BeginTx(context.Background(), driver.TxOptions{})
			require.NoError(t, err)
			require.True(t, conn.inTransaction)

			require.NoError(t, op.fn(tx))
			require.False(t, conn.inTransaction)
		})
	}
}
