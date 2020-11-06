// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package tagsql implements a tagged wrapper for databases.
//
// This package also handles hides context cancellation from database drivers
// that don't support it.
package tagsql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"runtime/pprof"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/private/traces"
)

// Open opens *sql.DB and wraps the implementation with tagging.
func Open(ctx context.Context, driverName, dataSourceName string) (DB, error) {
	var sdb *sql.DB
	var err error
	pprof.Do(ctx, pprof.Labels("db", driverName), func(ctx context.Context) {
		sdb, err = sql.Open(driverName, dataSourceName)
	})
	if err != nil {
		return nil, err
	}

	err = sdb.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return Wrap(sdb), nil
}

// Wrap turns a *sql.DB into a DB-matching interface.
func Wrap(db *sql.DB) DB {
	support, err := DetectContextSupport(db)
	if err != nil {
		// When we reach here it is definitely a programmer error.
		// Add any new database drivers into DetectContextSupport
		panic(err)
	}

	return &sqlDB{
		db:           db,
		useContext:   support.Basic(),
		useTxContext: support.Transactions(),
		tracker:      rootTracker(1),
	}
}

// WithoutContext turns a *sql.DB into a DB-matching that redirects context calls to regular calls.
func WithoutContext(db *sql.DB) DB {
	return &sqlDB{
		db:           db,
		useContext:   false,
		useTxContext: false,
		tracker:      rootTracker(1),
	}
}

// AllowContext turns a *sql.DB into a DB which uses context calls.
func AllowContext(db *sql.DB) DB {
	return &sqlDB{
		db:           db,
		useContext:   true,
		useTxContext: true,
		tracker:      rootTracker(1),
	}
}

// DB implements a wrapper for *sql.DB-like database.
//
// The wrapper adds tracing to all calls.
// It also adds context handling compatibility for different databases.
type DB interface {
	// To be deprecated, the following take ctx as argument,
	// however do not pass it forward to the underlying database.
	Begin(ctx context.Context) (Tx, error)
	Driver() driver.Driver
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Ping(ctx context.Context) error
	Prepare(ctx context.Context, query string) (Stmt, error)
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row

	BeginTx(ctx context.Context, txOptions *sql.TxOptions) (Tx, error)
	Conn(ctx context.Context) (Conn, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PingContext(ctx context.Context) error
	PrepareContext(ctx context.Context, query string) (Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	Close() error

	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats

	Internal() *sql.DB
}

// sqlDB implements DB, which optionally disables contexts.
type sqlDB struct {
	db           *sql.DB
	useContext   bool
	useTxContext bool
	tracker      *tracker
}

func (s *sqlDB) Internal() *sql.DB { return s.db }

func (s *sqlDB) Begin(ctx context.Context) (Tx, error) {
	traces.Tag(ctx, traces.TagDB)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	return &sqlTx{
		tx:         tx,
		useContext: s.useContext && s.useTxContext,
		tracker:    s.tracker.child(1),
	}, err
}

func (s *sqlDB) BeginTx(ctx context.Context, txOptions *sql.TxOptions) (Tx, error) {
	if txOptions != nil {
		return nil, errors.New("txOptions not supported")
	}
	traces.Tag(ctx, traces.TagDB)

	var tx *sql.Tx
	var err error
	if !s.useContext {
		tx, err = s.db.Begin()
	} else {
		tx, err = s.db.BeginTx(ctx, nil)
	}

	if err != nil {
		return nil, err
	}

	return &sqlTx{
		tx:         tx,
		useContext: s.useContext && s.useTxContext,
		tracker:    s.tracker.child(1),
	}, err
}

func (s *sqlDB) Close() error {
	return errs.Combine(s.tracker.close(), s.db.Close())
}

func (s *sqlDB) Conn(ctx context.Context) (Conn, error) {
	traces.Tag(ctx, traces.TagDB)
	var conn *sql.Conn
	var err error
	if !s.useContext {
		// Uses WithoutCancellation, because there isn't an underlying call that doesn't take a context.
		conn, err = s.db.Conn(context2.WithoutCancellation(ctx))
	} else {
		conn, err = s.db.Conn(ctx)
	}
	if err != nil {
		return nil, err
	}
	return &sqlConn{
		conn:         conn,
		useContext:   s.useContext,
		useTxContext: s.useTxContext,
		tracker:      s.tracker.child(1),
	}, nil
}

func (s *sqlDB) Driver() driver.Driver {
	return s.db.Driver()
}

func (s *sqlDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.db.Exec(query, args...)
}

func (s *sqlDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		return s.db.Exec(query, args...)
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *sqlDB) Ping(ctx context.Context) error {
	traces.Tag(ctx, traces.TagDB)
	return s.db.Ping()
}

func (s *sqlDB) PingContext(ctx context.Context) error {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		return s.db.Ping()
	}
	return s.db.PingContext(ctx)
}

func (s *sqlDB) Prepare(ctx context.Context, query string) (Stmt, error) {
	traces.Tag(ctx, traces.TagDB)
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &sqlStmt{stmt: stmt, useContext: s.useContext}, nil
}

func (s *sqlDB) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	traces.Tag(ctx, traces.TagDB)
	var stmt *sql.Stmt
	var err error
	if !s.useContext {
		stmt, err = s.db.Prepare(query)
		if err != nil {
			return nil, err
		}
	} else {
		stmt, err = s.db.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
	}
	return &sqlStmt{
		stmt:       stmt,
		useContext: s.useContext,
		tracker:    s.tracker.child(1),
	}, nil
}

func (s *sqlDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	traces.Tag(ctx, traces.TagDB)
	return s.tracker.wrapRows(s.db.Query(query, args...))
}

func (s *sqlDB) QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		return s.tracker.wrapRows(s.db.Query(query, args...))
	}
	return s.tracker.wrapRows(s.db.QueryContext(ctx, query, args...))
}

func (s *sqlDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	return s.db.QueryRow(query, args...)
}

func (s *sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		return s.db.QueryRow(query, args...)
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *sqlDB) SetConnMaxLifetime(d time.Duration) {
	s.db.SetConnMaxLifetime(d)
}

func (s *sqlDB) SetMaxIdleConns(n int) {
	s.db.SetMaxIdleConns(n)
}

func (s *sqlDB) SetMaxOpenConns(n int) {
	s.db.SetMaxOpenConns(n)
}

func (s *sqlDB) Stats() sql.DBStats {
	return s.db.Stats()
}
