// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/retrydb"
)

// DriverName is the database/sql driver name this package registers. Keep in
// sync with the corresponding tagsql.TiDBName logical name and the metabase
// dbutil.SplitConnStr dispatch.
const DriverName = "tidb"

// Driver is the type for the "tidb" sql/database driver.
// It wraps github.com/go-sql-driver/mysql so that tidb:// URLs can be opened directly
// (the URL is translated to a MySQL DSN), connections can be distinguished from
// plain MySQL at the driver-type level, and single-statement Exec/Query calls
// outside a transaction transparently retry on TiDB conflict errors.
type Driver struct{}

// Open returns a connection. If name has the tidb:// scheme it is converted to
// a MySQL DSN; otherwise it is forwarded to the MySQL driver as-is, allowing
// raw DSNs to also work.
func (d *Driver) Open(name string) (driver.Conn, error) {
	dsn, err := translateName(name)
	if err != nil {
		return nil, err
	}
	conn, err := mysql.MySQLDriver{}.Open(dsn)
	if err != nil {
		return nil, err
	}
	return wrapConn(conn)
}

// OpenConnector returns a Connector that produces wrapped connections for
// later sql.DB use. The DSN translation happens here, once, so re-connects
// during the pool's lifetime do not re-parse the URL.
func (d *Driver) OpenConnector(name string) (driver.Connector, error) {
	dsn, err := translateName(name)
	if err != nil {
		return nil, err
	}
	c, err := mysql.MySQLDriver{}.OpenConnector(dsn)
	if err != nil {
		return nil, err
	}
	return &tidbConnector{driver: d, underlying: c}, nil
}

// translateName converts a tidb:// URL to a MySQL DSN. Inputs that are not
// tidb:// URLs are returned unchanged so callers may also supply raw MySQL
// DSNs.
func translateName(name string) (string, error) {
	if strings.HasPrefix(name, "tidb://") {
		return URLToDSN(name)
	}
	return name, nil
}

// tidbConnector wraps a mysql connector so that the produced Conn is also
// wrapped.
type tidbConnector struct {
	driver     *Driver
	underlying driver.Connector
}

// Driver returns the driver used by this connector.
func (c *tidbConnector) Driver() driver.Driver { return c.driver }

// Connect produces a new wrapped connection.
func (c *tidbConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.underlying.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return wrapConn(conn)
}

// connAll asserts that the underlying mysql connection implements every
// driver-side interface we want to forward. The mysql driver implements all of
// these as of v1.10.
type connAll interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.QueryerContext
	driver.Pinger
	driver.NamedValueChecker
	driver.SessionResetter
	driver.Validator
}

// tidbConn wraps a mysql driver connection and adds retry semantics for
// single-statement operations performed outside a transaction. The
// inTransaction flag is set by BeginTx and cleared by tidbTx.Commit/Rollback;
// any path that opens a transaction outside BeginTx (raw BEGIN via Exec,
// savepoints, autocommit-off DSN flags) would invalidate it.
type tidbConn struct {
	underlying    connAll
	inTransaction bool
}

func wrapConn(conn driver.Conn) (driver.Conn, error) {
	c, ok := conn.(connAll)
	if !ok {
		return nil, errs.New("mysql driver Conn does not implement expected interfaces, got %T", conn)
	}
	return &tidbConn{underlying: c}, nil
}

func (c *tidbConn) Close() error                           { return c.underlying.Close() }
func (c *tidbConn) Ping(ctx context.Context) error         { return c.underlying.Ping(ctx) }
func (c *tidbConn) ResetSession(ctx context.Context) error { return c.underlying.ResetSession(ctx) }
func (c *tidbConn) IsValid() bool                          { return c.underlying.IsValid() }

func (c *tidbConn) CheckNamedValue(nv *driver.NamedValue) error {
	return c.underlying.CheckNamedValue(nv)
}

func (c *tidbConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.underlying.Prepare(query)
	if err != nil {
		return nil, err
	}
	return wrapStmt(stmt, c)
}

func (c *tidbConn) PrepareContext(ctx context.Context, query string) (_ driver.Stmt, err error) {
	defer mon.Task()(&ctx)(&err)
	stmt, err := c.underlying.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return wrapStmt(stmt, c)
}

func (c *tidbConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

// BeginTx starts a transaction with the given options. Once a transaction is
// active on this connection, single-statement retries are disabled until the
// transaction concludes — retries inside a transaction must be driven by the
// caller (txutil.WithTx) so business logic can be reapplied.
func (c *tidbConn) BeginTx(ctx context.Context, opts driver.TxOptions) (_ driver.Tx, err error) {
	defer mon.Task()(&ctx)(&err)
	tx, err := c.underlying.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	c.inTransaction = true
	return &tidbTx{underlying: tx, conn: c}, nil
}

// ExecContext runs an Exec, retrying on retryable errors when no transaction
// is active.
func (c *tidbConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (_ driver.Result, err error) {
	defer mon.Task()(&ctx)(&err)
	return retrydb.RetryConflict(mon, c.txActive, func() (driver.Result, error) {
		return c.underlying.ExecContext(ctx, query, args)
	})
}

// QueryContext runs a Query, retrying on retryable errors when no transaction
// is active.
func (c *tidbConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (_ driver.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	return retrydb.RetryConflict(mon, c.txActive, func() (driver.Rows, error) {
		return c.underlying.QueryContext(ctx, query, args)
	})
}

func (c *tidbConn) txActive() bool { return c.inTransaction }

// tidbTx wraps a mysql transaction so the conn's inTransaction flag is reset
// when the transaction concludes.
type tidbTx struct {
	underlying driver.Tx
	conn       *tidbConn
}

func (tx *tidbTx) Commit() error {
	err := tx.underlying.Commit()
	tx.conn.inTransaction = false
	return err
}

func (tx *tidbTx) Rollback() error {
	err := tx.underlying.Rollback()
	tx.conn.inTransaction = false
	return err
}

// tidbStmt wraps a prepared statement so single-statement retries also apply
// to explicit Stmt usage.
type tidbStmt struct {
	underlying retrydb.StmtAll
	conn       *tidbConn
}

func wrapStmt(stmt driver.Stmt, conn *tidbConn) (driver.Stmt, error) {
	s, ok := stmt.(retrydb.StmtAll)
	if !ok {
		return nil, errs.New("mysql Stmt %T does not provide context interfaces", stmt)
	}
	return &tidbStmt{underlying: s, conn: conn}, nil
}

func (s *tidbStmt) Close() error  { return s.underlying.Close() }
func (s *tidbStmt) NumInput() int { return s.underlying.NumInput() }

// Exec / Query forward to *Context so callers using the deprecated API still
// pick up retry-on-conflict semantics.
func (s *tidbStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), retrydb.ValuesToNamed(args))
}

func (s *tidbStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), retrydb.ValuesToNamed(args))
}

// ExecContext runs an Exec on a prepared statement, retrying on retryable
// errors when no transaction is active.
func (s *tidbStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (_ driver.Result, err error) {
	defer mon.Task()(&ctx)(&err)
	return retrydb.RetryConflict(mon, s.conn.txActive, func() (driver.Result, error) {
		return s.underlying.ExecContext(ctx, args)
	})
}

// QueryContext runs a Query on a prepared statement, retrying on retryable
// errors when no transaction is active.
func (s *tidbStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (_ driver.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	return retrydb.RetryConflict(mon, s.conn.txActive, func() (driver.Rows, error) {
		return s.underlying.QueryContext(ctx, args)
	})
}

var defaultDriver = &Driver{}

func init() {
	sql.Register(DriverName, defaultDriver)
}
