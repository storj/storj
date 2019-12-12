// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package utccheck

import (
	"context"
	"database/sql/driver"
	"time"

	"github.com/zeebo/errs"
)

// Connector wraps a driver.Connector with utc checks.
type Connector struct {
	connector driver.Connector
}

// WrapConnector wraps a driver.Connector with utc checks.
func WrapConnector(connector driver.Connector) *Connector {
	return &Connector{connector: connector}
}

// Unwrap returns the underlying driver.Connector.
func (c *Connector) Unwrap() driver.Connector { return c.connector }

// Connect returns a wrapped driver.Conn with utc checks.
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.connector.Connect(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return WrapConn(conn), nil
}

// Driver returns a wrapped driver.Driver with utc checks.
func (c *Connector) Driver() driver.Driver {
	return WrapDriver(c.connector.Driver())
}

//
// driver
//

// Driver wraps a driver.Driver with utc checks.
type Driver struct {
	driver driver.Driver
}

// WrapDriver wraps a driver.Driver with utc checks.
func WrapDriver(driver driver.Driver) *Driver {
	return &Driver{driver: driver}
}

// Unwrap returns the underlying driver.Driver.
func (d *Driver) Unwrap() driver.Driver { return d.driver }

// Open returns a wrapped driver.Conn with utc checks.
func (d *Driver) Open(name string) (driver.Conn, error) {
	conn, err := d.driver.Open(name)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return WrapConn(conn), nil
}

//
// conn
//

// Conn wraps a driver.Conn with utc checks.
type Conn struct {
	conn driver.Conn
}

// WrapConn wraps a driver.Conn with utc checks.
func WrapConn(conn driver.Conn) *Conn {
	return &Conn{conn: conn}
}

// Unwrap returns the underlying driver.Conn.
func (c *Conn) Unwrap() driver.Conn { return c.conn }

// Close closes the conn.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Ping implements driver.Pinger.
func (c *Conn) Ping(ctx context.Context) error {
	// sqlite3 implements this
	return c.conn.(driver.Pinger).Ping(ctx)
}

// Begin returns a wrapped driver.Tx with utc checks.
func (c *Conn) Begin() (driver.Tx, error) {
	//lint:ignore SA1019 deprecated is fine. this is a wrapper.
	//nolint
	tx, err := c.conn.Begin()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return WrapTx(tx), nil
}

// BeginTx returns a wrapped driver.Tx with utc checks.
func (c *Conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	// sqlite3 implements this
	tx, err := c.conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return WrapTx(tx), nil
}

// Query checks the arguments for non-utc timestamps and returns the result.
func (c *Conn) Query(query string, args []driver.Value) (driver.Rows, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}

	// sqlite3 implements this
	//
	//lint:ignore SA1019 deprecated is fine. this is a wrapper.
	//nolint
	return c.conn.(driver.Queryer).Query(query, args)
}

// QueryContext checks the arguments for non-utc timestamps and returns the result.
func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := utcCheckNamedArgs(args); err != nil {
		return nil, err
	}

	// sqlite3 implements this
	return c.conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

// Exec checks the arguments for non-utc timestamps and returns the result.
func (c *Conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, err
	}

	// sqlite3 implements this
	//
	//lint:ignore SA1019 deprecated is fine. this is a wrapper.
	//nolint
	return c.conn.(driver.Execer).Exec(query, args)
}

// ExecContext checks the arguments for non-utc timestamps and returns the result.
func (c *Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := utcCheckNamedArgs(args); err != nil {
		return nil, err
	}

	// sqlite3 implements this
	return c.conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

// Prepare returns a wrapped driver.Stmt with utc checks.
func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return WrapStmt(stmt), nil
}

// PrepareContext checks the arguments for non-utc timestamps and returns the result.
func (c *Conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	// sqlite3 implements this
	stmt, err := c.conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return WrapStmt(stmt), nil
}

//
// stmt
//

// Stmt wraps a driver.Stmt with utc checks.
type Stmt struct {
	stmt driver.Stmt
}

// WrapStmt wraps a driver.Stmt with utc checks.
func WrapStmt(stmt driver.Stmt) *Stmt {
	return &Stmt{stmt: stmt}
}

// Unwrap returns the underlying driver.Stmt.
func (s *Stmt) Unwrap() driver.Stmt { return s.stmt }

// Close closes the stmt.
func (s *Stmt) Close() error {
	return s.stmt.Close()
}

// NumInput returns the number of inputs to the stmt.
func (s *Stmt) NumInput() int {
	return s.stmt.NumInput()
}

// Exec checks the arguments for non-utc timestamps and returns the result.
func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, errs.Wrap(err)
	}

	//lint:ignore SA1019 deprecated is fine. this is a wrapper.
	//nolint
	return s.stmt.Exec(args)
}

// Query checks the arguments for non-utc timestamps and returns the result.
func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	if err := utcCheckArgs(args); err != nil {
		return nil, errs.Wrap(err)
	}

	//lint:ignore SA1019 deprecated is fine. this is a wrapper.
	//nolint
	return s.stmt.Query(args)
}

//
// tx
//

// Tx wraps a driver.Tx with utc checks.
type Tx struct {
	tx driver.Tx
}

// WrapTx wraps a driver.Tx with utc checks.
func WrapTx(tx driver.Tx) *Tx {
	return &Tx{tx: tx}
}

// Unwrap returns the underlying driver.Tx.
func (t *Tx) Unwrap() driver.Tx { return t.tx }

// Commit commits the tx.
func (t *Tx) Commit() error {
	return t.tx.Commit()
}

// Rollback rolls the tx back.
func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

//
// helpers
//

func utcCheckArg(n int, arg interface{}) error {
	var t time.Time
	var ok bool

	switch a := arg.(type) {
	case time.Time:
		t, ok = a, true
	case *time.Time:
		if a != nil {
			t, ok = *a, true
		}
	}

	if !ok {
		return nil
	} else if loc := t.Location(); loc != time.UTC {
		return errs.New("invalid timezone on argument %d: %v", n, loc)
	} else {
		return nil
	}
}

func utcCheckNamedArgs(args []driver.NamedValue) error {
	for n, arg := range args {
		if err := utcCheckArg(n, arg.Value); err != nil {
			return err
		}
	}
	return nil
}

func utcCheckArgs(args []driver.Value) error {
	for n, arg := range args {
		if err := utcCheckArg(n, arg); err != nil {
			return err
		}
	}
	return nil
}
