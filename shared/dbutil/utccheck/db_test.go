// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utccheck_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/dbutil/utccheck"
)

func TestUTCDB(t *testing.T) {
	ctx := t.Context()

	notUTC := time.FixedZone("not utc", -1)
	db := sql.OpenDB(utccheck.WrapConnector(emptyConnector{}))

	{ // time.Time not in UTC
		_, err := db.ExecContext(ctx, "", time.Now().In(notUTC))
		require.Error(t, err)
	}

	{ // *time.Time not in UTC
		now := time.Now().In(notUTC)
		_, err := db.ExecContext(ctx, "", &now)
		require.Error(t, err)
	}

	{ // time.Time in UTC
		_, err := db.ExecContext(ctx, "", time.Now().UTC())
		require.NoError(t, err)
	}

	{ // *time.Time in UTC
		now := time.Now().UTC()
		_, err := db.ExecContext(ctx, "", &now)
		require.NoError(t, err)
	}

	{ // nil *time.Time
		_, err := db.ExecContext(ctx, "", (*time.Time)(nil))
		require.NoError(t, err)
	}
}

//
// empty driver
//

type emptyConnector struct{}

func (emptyConnector) Connect(context.Context) (driver.Conn, error) { return emptyConn{}, nil }
func (emptyConnector) Driver() driver.Driver                        { return nil }

type emptyConn struct{}

func (emptyConn) Close() error { return nil }

func (emptyConn) Prepare(query string) (driver.Stmt, error) { return emptyStmt{}, nil }
func (emptyConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return emptyStmt{}, nil
}

func (emptyConn) Begin() (driver.Tx, error) { return emptyTx{}, nil }
func (emptyConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return emptyTx{}, nil
}

func (emptyConn) Query(query string, args []driver.Value) (driver.Rows, error) { return nil, nil }
func (emptyConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return nil, nil
}

func (emptyConn) Exec(query string, args []driver.Value) (driver.Result, error) { return nil, nil }
func (emptyConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return nil, nil
}

type emptyTx struct{}

func (emptyTx) Commit() error   { return nil }
func (emptyTx) Rollback() error { return nil }

type emptyStmt struct{}

func (emptyStmt) Close() error                                    { return nil }
func (emptyStmt) Exec(args []driver.Value) (driver.Result, error) { return nil, nil }
func (emptyStmt) Query(args []driver.Value) (driver.Rows, error)  { return nil, nil }

// must be 1 so that we can pass 1 argument.
func (emptyStmt) NumInput() int { return 1 }
