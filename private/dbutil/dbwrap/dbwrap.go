// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbwrap

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"
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
	*sql.DB
}

func (s sqlDB) DriverContext(ctx context.Context) driver.Driver {
	return s.DB.Driver()
}

func (s sqlDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := s.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return sqlTx{Tx: tx}, nil
}

func (s sqlDB) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	return s.DB.PrepareContext(ctx, query)
}

func (s sqlDB) Conn(ctx context.Context) (Conn, error) {
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
	*sql.Conn
}

func (s sqlConn) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := s.Conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return sqlTx{Tx: tx}, nil
}

func (s sqlConn) RawContext(ctx context.Context, f func(driverConn interface{}) error) error {
	return s.Conn.Raw(f)
}

// Stmt is an interface for *sql.Stmt-like prepared statements.
type Stmt interface {
	ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row

	Close() error
}

type sqlTx struct {
	*sql.Tx
}

func (s sqlTx) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	return s.Tx.PrepareContext(ctx, query)
}
