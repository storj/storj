// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbwrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"storj.io/storj/pkg/traces"
)

// DB implements a wrapper interface for *sql.DB-like databases which
// require contexts.
type DB interface {
	DriverContext(context.Context) driver.Driver

	BeginTx(context.Context, *sql.TxOptions) (Tx, error)

	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	PrepareContext(ctx context.Context, query string) (Stmt, error)

	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	SetConnMaxLifetime(time.Duration)
	Stats() sql.DBStats

	Conn(ctx context.Context) (Conn, error)

	Close() error
}

type sqlDB struct {
	DB *sql.DB
}

func (s sqlDB) DriverContext(ctx context.Context) driver.Driver {
	traces.Tag(ctx, traces.TagDB)
	return s.DB.Driver()
}

func (s sqlDB) Close() error { return s.DB.Close() }

func (s sqlDB) SetMaxIdleConns(n int)              { s.DB.SetMaxIdleConns(n) }
func (s sqlDB) SetMaxOpenConns(n int)              { s.DB.SetMaxOpenConns(n) }
func (s sqlDB) SetConnMaxLifetime(d time.Duration) { s.DB.SetConnMaxLifetime(d) }
func (s sqlDB) Stats() sql.DBStats                 { return s.DB.Stats() }

func (s sqlDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	traces.Tag(ctx, traces.TagDB)
	tx, err := s.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return sqlTx{Tx: tx}, nil
}

func (s sqlDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.DB.ExecContext(ctx, query, args...)
}

func (s sqlDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.DB.QueryContext(ctx, query, args...)
}

func (s sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	return s.DB.QueryRowContext(ctx, query, args...)
}

func (s sqlDB) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.DB.PrepareContext(ctx, query)
}

func (s sqlDB) Conn(ctx context.Context) (Conn, error) {
	traces.Tag(ctx, traces.TagDB)
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	return sqlConn{Conn: conn}, nil
}

// SQLDB turns a *sql.DB into a DB-matching interface
func SQLDB(db *sql.DB) DB {
	return sqlDB{DB: db}
}

// Tx is an interface for *sql.Tx-like transactions
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	PrepareContext(ctx context.Context, query string) (Stmt, error)

	Commit() error
	Rollback() error
}

// Conn is an interface for *sql.Conn-like connections
type Conn interface {
	BeginTx(context.Context, *sql.TxOptions) (Tx, error)

	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	RawContext(ctx context.Context, f func(driverConn interface{}) error) error

	Close() error
}

type sqlConn struct {
	Conn *sql.Conn
}

func (s sqlConn) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	traces.Tag(ctx, traces.TagDB)
	tx, err := s.Conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return sqlTx{Tx: tx}, nil
}

func (s sqlConn) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.Conn.ExecContext(ctx, query, args...)
}

func (s sqlConn) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.Conn.QueryContext(ctx, query, args...)
}

func (s sqlConn) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	return s.Conn.QueryRowContext(ctx, query, args...)
}

func (s sqlConn) RawContext(ctx context.Context, f func(driverConn interface{}) error) error {
	traces.Tag(ctx, traces.TagDB)
	return s.Conn.Raw(f)
}

func (s sqlConn) Close() error {
	return s.Conn.Close()
}

// Stmt is an interface for *sql.Stmt-like prepared statements.
type Stmt interface {
	ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row

	Close() error
}

type sqlTx struct {
	Tx *sql.Tx
}

func (s sqlTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.Tx.ExecContext(ctx, query, args...)
}

func (s sqlTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.Tx.QueryContext(ctx, query, args...)
}

func (s sqlTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	return s.Tx.QueryRowContext(ctx, query, args...)
}

func (s sqlTx) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.Tx.PrepareContext(ctx, query)
}

func (s sqlTx) Commit() error   { return s.Tx.Commit() }
func (s sqlTx) Rollback() error { return s.Tx.Rollback() }
