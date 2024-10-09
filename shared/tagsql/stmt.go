// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/common/leak"
	"storj.io/common/traces"
)

// Stmt is an interface for *sql.Stmt.
type Stmt interface {
	// Exec and other methods take a context for tracing
	// purposes, but do not pass the context to the underlying database query.
	Exec(ctx context.Context, args ...interface{}) (sql.Result, error)
	Query(ctx context.Context, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, args ...interface{}) *sql.Row

	// ExecContext and other Context methods take a context for tracing and also
	// pass the context to the underlying database, if this tagsql instance is
	// configured to do so. (By default, lib/pq does not ever, and
	// mattn/go-sqlite3 does not for transactions).
	ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, args ...interface{}) (Rows, error)
	QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row

	Close() error
}

// sqlStmt implements Stmt, which optionally disables contexts.
type sqlStmt struct {
	query      string
	stmt       *sql.Stmt
	useContext bool
	tracker    leak.Ref
}

func (s *sqlStmt) Close() error {
	return errs.Combine(s.tracker.Close(), s.stmt.Close())
}

func (s *sqlStmt) Exec(ctx context.Context, args ...interface{}) (_ sql.Result, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(&err)

	return s.stmt.Exec(args...)
}

func (s *sqlStmt) ExecContext(ctx context.Context, args ...interface{}) (_ sql.Result, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(&err)

	if !s.useContext {
		return s.stmt.Exec(args...)
	}
	return s.stmt.ExecContext(ctx, args...)
}

func (s *sqlStmt) Query(ctx context.Context, args ...interface{}) (_ Rows, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(&err)

	return s.wrapRows(s.stmt.Query(args...))
}

func (s *sqlStmt) QueryContext(ctx context.Context, args ...interface{}) (_ Rows, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(&err)

	if !s.useContext {
		return s.wrapRows(s.stmt.Query(args...))
	}
	return s.wrapRows(s.stmt.QueryContext(ctx, args...))
}

func (s *sqlStmt) QueryRow(ctx context.Context, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(nil)

	return s.stmt.QueryRow(args...)
}

func (s *sqlStmt) QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(nil)

	if !s.useContext {
		return s.stmt.QueryRow(args...)
	}
	return s.stmt.QueryRowContext(ctx, args...)
}
