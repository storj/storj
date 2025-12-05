// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/common/leak"
	"storj.io/common/traces"
	"storj.io/storj/shared/flightrecorder"
)

// Stmt is an interface for *sql.Stmt.
type Stmt interface {
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
	box        *flightrecorder.Box
}

func (s *sqlStmt) Close() error {
	s.record()
	return errs.Combine(s.tracker.Close(), s.stmt.Close())
}

func (s *sqlStmt) ExecContext(ctx context.Context, args ...interface{}) (_ sql.Result, err error) {
	s.record()
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(&err)

	if !s.useContext {
		return s.stmt.Exec(args...) //nolint: noctx, fallback for non-context behaviour
	}
	return s.stmt.ExecContext(ctx, args...)
}

func (s *sqlStmt) QueryContext(ctx context.Context, args ...interface{}) (_ Rows, err error) {
	s.record()
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(&err)

	if !s.useContext {
		return s.wrapRows(s.stmt.Query(args...)) //nolint: noctx, fallback for non-context behaviour
	}
	return s.wrapRows(s.stmt.QueryContext(ctx, args...))
}

func (s *sqlStmt) QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row {
	s.record()
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, s.query, args)(nil)

	if !s.useContext {
		return s.stmt.QueryRow(args...) //nolint: noctx, fallback for non-context behaviour
	}
	return s.stmt.QueryRowContext(ctx, args...)
}

func (s *sqlStmt) record() {
	if s.box == nil {
		return
	}

	s.box.Enqueue(flightrecorder.EventTypeDB, 1) // 1 to skip record call.
}
