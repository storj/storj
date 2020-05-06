// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"strings"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
)

// Driver is the type for the "cockroach" sql/database driver.
// It uses github.com/lib/pq under the covers because of Cockroach's
// PostgreSQL compatibility, but allows differentiation between pg and
// crdb connections.
type Driver struct {
	pq.Driver
}

// Open opens a new cockroachDB connection.
func (cd *Driver) Open(name string) (driver.Conn, error) {
	name = translateName(name)
	return pq.Open(name)
}

// OpenConnector obtains a new db Connector, which sql.DB can use to
// obtain each needed connection at the appropriate time.
func (cd *Driver) OpenConnector(name string) (driver.Connector, error) {
	name = translateName(name)
	pgConnector, err := pq.NewConnector(name)
	if err != nil {
		return nil, err
	}
	return &cockroachConnector{pgConnector}, nil
}

// cockroachConnector is a thin wrapper around a pq-based connector. This allows
// Driver to supply our custom cockroachConn type for connections.
type cockroachConnector struct {
	pgConnector driver.Connector
}

// Driver returns the driver being used for this connector.
func (c *cockroachConnector) Driver() driver.Driver {
	return &Driver{}
}

// Connect creates a new connection using the connector.
func (c *cockroachConnector) Connect(ctx context.Context) (driver.Conn, error) {
	pgConn, err := c.pgConnector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	if pgConnAll, ok := pgConn.(connAll); ok {
		return &cockroachConn{pgConnAll}, nil
	}
	return nil, errs.New("Underlying connector type %T does not implement connAll?!", pgConn)
}

type connAll interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ExecerContext
	driver.QueryerContext
}

// cockroachConn is a connection to a database. It is not used concurrently by multiple goroutines.
type cockroachConn struct {
	underlying connAll
}

// Assert that cockroachConn fulfills connAll.
var _ connAll = (*cockroachConn)(nil)

// Close closes the cockroachConn.
func (c *cockroachConn) Close() error {
	return c.underlying.Close()
}

// ExecContext (when implemented by a driver.Conn) provides ExecContext
// functionality to a sql.DB instance. This implementation provides
// retry semantics for single statements.
func (c *cockroachConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	result, err := c.underlying.ExecContext(ctx, query, args)
	for err != nil && !c.isInTransaction() && needsRetry(err) {
		result, err = c.underlying.ExecContext(ctx, query, args)
	}
	return result, err
}

// QueryContext (when implemented by a driver.Conn) provides QueryContext
// functionality to a sql.DB instance. This implementation provides
// retry semantics for single statements.
func (c *cockroachConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	result, err := c.underlying.QueryContext(ctx, query, args)
	for err != nil && !c.isInTransaction() && needsRetry(err) {
		result, err = c.underlying.QueryContext(ctx, query, args)
	}
	return result, err
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
	// access c.underlying -> c.underlying.(*pq.conn) -> (*c.underlying.(*pq.conn)).txnStatus
	//
	// this is of course brittle if lib/pq internals change, so a test is necessary to make
	// sure we stay on the same page.
	return transactionStatus(reflect.ValueOf(c.underlying).Elem().Field(4).Uint())
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
	for err != nil && !stmt.conn.isInTransaction() && needsRetry(err) {
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
	result, err := stmt.underlyingStmt.QueryContext(context.Background(), namedArgs)
	for err != nil && !stmt.conn.isInTransaction() && needsRetry(err) {
		result, err = stmt.underlyingStmt.QueryContext(context.Background(), namedArgs)
	}
	return result, err
}

// ExecContext executes SQL statements in the specified context.
func (stmt *cockroachStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	result, err := stmt.underlyingStmt.ExecContext(ctx, args)
	for err != nil && !stmt.conn.isInTransaction() && needsRetry(err) {
		result, err = stmt.underlyingStmt.ExecContext(ctx, args)
	}
	return result, err
}

// QueryContext executes a query in the specified context.
func (stmt *cockroachStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := stmt.underlyingStmt.QueryContext(ctx, args)
	for err != nil && !stmt.conn.isInTransaction() && needsRetry(err) {
		rows, err = stmt.underlyingStmt.QueryContext(ctx, args)
	}
	return rows, err
}

// translateName changes the scheme name in a `cockroach://` URL to
// `postgres://`, as that is what lib/pq will expect.
func translateName(name string) string {
	if strings.HasPrefix(name, "cockroach://") {
		name = "postgres://" + name[12:]
	}
	return name
}

// borrowed from code in crdb
func needsRetry(err error) bool {
	code := errCode(err)
	return code == "40001" || code == "CR000"
}

// borrowed from crdb
func errCode(err error) string {
	switch t := errorCause(err).(type) {
	case *pq.Error:
		return string(t.Code)
	default:
		return ""
	}
}

func errorCause(err error) error {
	for err != nil {
		cause := errors.Unwrap(err)
		if cause == nil {
			break
		}
		err = cause
	}
	return err
}

// Assert that Driver satisfies DriverContext.
var _ driver.DriverContext = &Driver{}

func init() {
	sql.Register("cockroach", &Driver{})
}
