// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/dbutil/retrydb"
)

// Driver is the type for the "cockroach" sql/database driver. It uses
// github.com/jackc/pgx/v5/stdlib under the covers because of Cockroach's
// PostgreSQL compatibility, but allows differentiation between pg and crdb
// connections.
type Driver struct {
	pgxDriver stdlib.Driver
}

// Open opens a new cockroachDB connection.
func (cd *Driver) Open(name string) (driver.Conn, error) {
	name = translateName(name)
	conn, err := cd.pgxDriver.Open(name)
	if err != nil {
		return nil, err
	}
	pgxStdlibConn, ok := conn.(*stdlib.Conn)
	if !ok {
		return nil, errs.New("Conn from pgx is not a *stdlib.Conn??? T: %T", conn)
	}
	return &cockroachConn{pgxStdlibConn}, nil
}

// OpenConnector obtains a new db Connector, which sql.DB can use to
// obtain each needed connection at the appropriate time.
func (cd *Driver) OpenConnector(name string) (driver.Connector, error) {
	name = translateName(name)
	pgxConnector, err := cd.pgxDriver.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return &cockroachConnector{driver: cd, pgxConnector: pgxConnector}, nil
}

// cockroachConnector is a thin wrapper around a pq-based connector. This allows
// Driver to supply our custom cockroachConn type for connections.
type cockroachConnector struct {
	driver       *Driver
	pgxConnector driver.Connector
}

// Driver returns the driver being used for this connector.
func (c *cockroachConnector) Driver() driver.Driver {
	return c.driver
}

// Connect creates a new connection using the connector.
func (c *cockroachConnector) Connect(ctx context.Context) (driver.Conn, error) {
	pgxConn, err := c.pgxConnector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	pgxStdlibConn, ok := pgxConn.(*stdlib.Conn)
	if !ok {
		return nil, errs.New("Conn from pgx is not a *stdlib.Conn??? T: %T", pgxConn)
	}
	return &cockroachConn{pgxStdlibConn}, nil
}

type connAll interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ExecerContext
	driver.QueryerContext
	driver.Pinger
}

// cockroachConn is a connection to a database. It is not used concurrently by multiple goroutines.
type cockroachConn struct {
	underlying *stdlib.Conn
}

// Assert that cockroachConn fulfills connAll.
var _ connAll = (*cockroachConn)(nil)

// StdlibConn returns the underlying pgx std connection.
func (c *cockroachConn) StdlibConn() *stdlib.Conn { return c.underlying }

// Close closes the cockroachConn.
func (c *cockroachConn) Close() error {
	return c.underlying.Close()
}

// Ping checks if the cockroachConn is reachable.
func (c *cockroachConn) Ping(ctx context.Context) error {
	return c.underlying.Ping(ctx)
}

// ExecContext (when implemented by a driver.Conn) provides ExecContext
// functionality to a sql.DB instance. This implementation provides
// retry semantics for single statements.
func (c *cockroachConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.underlying.ExecContext(ctx, query, args)
	for err != nil && !c.isInTransaction() && retrydb.ShouldRetry(err) {
		mon.Event("needed_retry")
		result, err = c.underlying.ExecContext(ctx, query, args)
	}
	return result, err
}

type cockroachRows struct {
	rows         driver.Rows
	firstResults []driver.Value
	eof          bool
}

// Columns returns the names of the columns.
func (rows *cockroachRows) Columns() []string {
	return rows.rows.Columns()
}

// Close closes the rows iterator.
func (rows *cockroachRows) Close() error {
	return rows.rows.Close()
}

// Next implements the Next method on driver.Rows.
func (rows *cockroachRows) Next(dest []driver.Value) error {
	if rows.eof {
		return io.EOF
	}
	if rows.firstResults == nil {
		return rows.rows.Next(dest)
	}
	copy(dest, rows.firstResults)
	rows.firstResults = nil
	return nil
}

func wrapRows(rows driver.Rows) (crdbRows *cockroachRows, err error) {
	columns := rows.Columns()
	dest := make([]driver.Value, len(columns))
	err = rows.Next(dest)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return &cockroachRows{rows: rows, firstResults: nil, eof: true}, nil
		}
		return nil, err
	}
	return &cockroachRows{rows: rows, firstResults: dest}, nil
}

// QueryContext (when implemented by a driver.Conn) provides QueryContext
// functionality to a sql.DB instance. This implementation provides
// retry semantics for single statements.
func (c *cockroachConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (_ driver.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	for {
		result, err := c.underlying.QueryContext(ctx, query, args)
		if err != nil {
			if retrydb.ShouldRetry(err) {
				if c.isInTransaction() {
					return nil, err
				}
				mon.Event("needed_retry")
				continue
			}
			return nil, err
		}
		wrappedResult, err := wrapRows(result)
		if err != nil {
			// If this returns an error it's probably the same error
			// we got from calling Next inside wrapRows.
			_ = result.Close()
			if retrydb.ShouldRetry(err) {
				if c.isInTransaction() {
					return nil, err
				}
				mon.Event("needed_retry")
				continue
			}
			return nil, err
		}
		return wrappedResult, nil
	}
}

// Begin starts a new transaction.
func (c *cockroachConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

// BeginTx begins a new transaction using the specified context and with the specified options.
func (c *cockroachConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.underlying.BeginTx(ctx, opts)
}

// Prepare prepares a statement for future execution.
func (c *cockroachConn) Prepare(query string) (driver.Stmt, error) {
	pqStmt, err := c.underlying.Prepare(query)
	if err != nil {
		return nil, err
	}
	adapted, ok := pqStmt.(stmtAll)
	if !ok {
		return nil, errs.New("Stmt type %T does not provide stmtAll?!", adapted)
	}
	return &cockroachStmt{underlyingStmt: adapted, conn: c}, nil
}

type transactionStatus byte

const (
	txnStatusIdle                transactionStatus = 'I'
	txnStatusIdleInTransaction   transactionStatus = 'T'
	txnStatusInFailedTransaction transactionStatus = 'E'
)

func (c *cockroachConn) txnStatus() transactionStatus {
	pgConn := c.underlying.Conn().PgConn()
	return transactionStatus(pgConn.TxStatus())
}

func (c *cockroachConn) isInTransaction() bool {
	txnStatus := c.txnStatus()
	return txnStatus == txnStatusIdleInTransaction || txnStatus == txnStatusInFailedTransaction
}

type stmtAll interface {
	driver.Stmt
	driver.StmtExecContext
	driver.StmtQueryContext
}

type cockroachStmt struct {
	underlyingStmt stmtAll
	conn           *cockroachConn
}

// Assert that cockroachStmt satisfies StmtExecContext and StmtQueryContext.
var _ stmtAll = (*cockroachStmt)(nil)

// Close closes a prepared statement.
func (stmt *cockroachStmt) Close() error {
	return stmt.underlyingStmt.Close()
}

// NumInput returns the number of placeholder parameters.
func (stmt *cockroachStmt) NumInput() int {
	return stmt.underlyingStmt.NumInput()
}

// Exec executes a SQL statement in the background context.
func (stmt *cockroachStmt) Exec(args []driver.Value) (driver.Result, error) {
	// since (driver.Stmt).Exec() is deprecated, we translate our Value args to NamedValue args
	// and pass in background context to ExecContext instead.
	namedArgs := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	result, err := stmt.underlyingStmt.ExecContext(context.Background(), namedArgs)
	for err != nil && !stmt.conn.isInTransaction() && retrydb.ShouldRetry(err) {
		mon.Event("needed_retry")
		result, err = stmt.underlyingStmt.ExecContext(context.Background(), namedArgs)
	}
	return result, err
}

// Query executes a query in the background context.
func (stmt *cockroachStmt) Query(args []driver.Value) (driver.Rows, error) {
	// since (driver.Stmt).Query() is deprecated, we translate our Value args to NamedValue args
	// and pass in background context to QueryContext instead.
	namedArgs := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return stmt.QueryContext(context.Background(), namedArgs)
}

// ExecContext executes SQL statements in the specified context.
func (stmt *cockroachStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	result, err := stmt.underlyingStmt.ExecContext(ctx, args)
	for err != nil && !stmt.conn.isInTransaction() && retrydb.ShouldRetry(err) {
		mon.Event("needed_retry")
		result, err = stmt.underlyingStmt.ExecContext(ctx, args)
	}
	return result, err
}

// QueryContext executes a query in the specified context.
func (stmt *cockroachStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (_ driver.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	for {
		result, err := stmt.underlyingStmt.QueryContext(ctx, args)
		if err != nil {
			if retrydb.ShouldRetry(err) {
				if stmt.conn.isInTransaction() {
					return nil, err
				}
				mon.Event("needed_retry")
				continue
			}
			return nil, err
		}
		wrappedResult, err := wrapRows(result)
		if err != nil {
			// If this returns an error it's probably the same error
			// we got from calling Next inside wrapRows.
			_ = result.Close()
			if retrydb.ShouldRetry(err) {
				if stmt.conn.isInTransaction() {
					return nil, err
				}
				mon.Event("needed_retry")
				continue
			}
			return nil, err
		}
		return wrappedResult, nil
	}
}

// translateName changes the scheme name in a `cockroach://` URL to
// `postgres://`, as that is what jackc/pgx will expect.
func translateName(name string) string {
	if strings.HasPrefix(name, "cockroach://") {
		name = "postgres://" + name[12:]
	}
	return name
}

var defaultDriver = &Driver{}

func init() {
	sql.Register("cockroach", defaultDriver)
}
