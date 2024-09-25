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
	"errors"
	"fmt"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/leak"
	"storj.io/common/traces"
)

var mon = monkit.Package()

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
		tracker:      leak.Root(1),
	}
}

// WithoutContext turns a *sql.DB into a DB-matching that redirects context calls to regular calls.
func WithoutContext(db *sql.DB) DB {
	return &sqlDB{
		db:           db,
		useContext:   false,
		useTxContext: false,
		tracker:      leak.Root(1),
	}
}

// AllowContext turns a *sql.DB into a DB which uses context calls.
func AllowContext(db *sql.DB) DB {
	return &sqlDB{
		db:           db,
		useContext:   true,
		useTxContext: true,
		tracker:      leak.Root(1),
	}
}

// DB implements a wrapper for *sql.DB-like database.
//
// The wrapper adds tracing to all calls.
// It also adds context handling compatibility for different databases.
type DB interface {
	Name() string

	// To be deprecated, the following take ctx as argument,
	// however do not pass it forward to the underlying database.
	Begin(ctx context.Context) (Tx, error)
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
}

// sqlDB implements DB, which optionally disables contexts.
type sqlDB struct {
	db           *sql.DB
	useContext   bool
	useTxContext bool
	tracker      leak.Ref
}

const (

	// CockroachName is the name when tagsql wraps a Cockroach DB connection.
	CockroachName string = "cockroach"

	// PostgresName is the name when tagsql wraps a Cockroach DB connection.
	PostgresName string = "postgres"

	// SpannerName is the name when tagsql wraps a Cockroach DB connection.
	SpannerName string = "spanner"

	// SqliteName is the name when tagsql wraps a SQLite3 connection.
	SqliteName string = "sqlite"
)

func (s *sqlDB) Name() string {
	driverType := fmt.Sprintf("%T", s.db.Driver())
	switch {
	case strings.Contains(driverType, "cockroach"):
		return CockroachName
	case strings.Contains(driverType, "postgres"):
		return PostgresName
	case strings.Contains(driverType, "spanner"):
		return SpannerName
	case strings.Contains(driverType, "sqlite3.SQLiteDriver"):
		return SqliteName
	// only used by golang benchmark
	case strings.Contains(driverType, "stdlib.Driver"):
		return PostgresName
	// only used under test; treat as sqlite
	case strings.Contains(driverType, "utccheck.Driver"):
		return SqliteName
	default:
		panic("unknown database driver: " + driverType)
	}

}

func (s *sqlDB) Begin(ctx context.Context) (Tx, error) {
	traces.Tag(ctx, traces.TagDB)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	return &sqlTx{
		tx:         tx,
		useContext: s.useContext && s.useTxContext,
		tracker:    s.tracker.Child("sqlTx", 1),
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
		tracker:    s.tracker.Child("sqlTx", 1),
	}, err
}

func (s *sqlDB) Close() error {
	return errs.Combine(s.tracker.Close(), s.db.Close())
}

func (s *sqlDB) Conn(ctx context.Context) (Conn, error) {
	monConnWaiting.Inc(1)
	defer monConnWaiting.Dec(1)

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

	monConnOpen.Inc(1)
	return &sqlConn{
		conn:         conn,
		useContext:   s.useContext,
		useTxContext: s.useTxContext,
		tracker:      s.tracker.Child("sqlConn", 1),
	}, nil
}

func (s *sqlDB) Exec(ctx context.Context, query string, args ...interface{}) (_ sql.Result, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	return s.db.Exec(query, args...)
}

func (s *sqlDB) ExecContext(ctx context.Context, query string, args ...interface{}) (_ sql.Result, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

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

func (s *sqlDB) Prepare(ctx context.Context, query string) (_ Stmt, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query)(&err)

	stmt, err := s.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &sqlStmt{
		query:      query,
		stmt:       stmt,
		useContext: s.useContext,
		tracker:    s.tracker.Child("sqlStmt", 1),
	}, nil
}

func (s *sqlDB) PrepareContext(ctx context.Context, query string) (_ Stmt, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query)(&err)

	var stmt *sql.Stmt
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
		query:      query,
		stmt:       stmt,
		useContext: s.useContext,
		tracker:    s.tracker.Child("sqlStmt", 1),
	}, nil
}

func (s *sqlDB) Query(ctx context.Context, query string, args ...interface{}) (_ Rows, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	return s.wrapRows(s.db.Query(query, args...))
}

func (s *sqlDB) QueryContext(ctx context.Context, query string, args ...interface{}) (_ Rows, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	if !s.useContext {
		return s.wrapRows(s.db.Query(query, args...))
	}
	return s.wrapRows(s.db.QueryContext(ctx, query, args...))
}

func (s *sqlDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(nil)

	return s.db.QueryRow(query, args...)
}

func (s *sqlDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(nil)

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
