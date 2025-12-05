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

// ExecQueryer contains methods for executing queries.
type ExecQueryer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Tx is an interface for *sql.Tx-like transactions.
type Tx interface {
	// ExecContext and other Context methods take a context for tracing and also
	// pass the context to the underlying database, if this tagsql instance is
	// configured to do so. (By default, lib/pq does not ever, and
	// mattn/go-sqlite3 does not for transactions).
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	Commit() error
	Rollback() error
}

// sqlTx implements Tx, which optionally disables contexts.
type sqlTx struct {
	tx         *sql.Tx
	useContext bool
	tracker    leak.Ref
	box        *flightrecorder.Box
}

func (s *sqlTx) ExecContext(ctx context.Context, query string, args ...interface{}) (_ sql.Result, err error) {
	s.record()
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	if !s.useContext {
		return s.tx.Exec(query, args...) //nolint: noctx, fallback for non-context behaviour
	}
	return s.tx.ExecContext(ctx, query, args...)
}

func (s *sqlTx) PrepareContext(ctx context.Context, query string) (_ Stmt, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query)(&err)

	var stmt *sql.Stmt
	if !s.useContext {
		stmt, err = s.tx.Prepare(query) //nolint: noctx, fallback for non-context behaviour
		if err != nil {
			return nil, err
		}
	} else {
		stmt, err = s.tx.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
	}
	return &sqlStmt{
		query:      query,
		stmt:       stmt,
		useContext: s.useContext,
		tracker:    s.tracker.Child("sqlStmt", 1),
		box:        s.box,
	}, err
}

func (s *sqlTx) QueryContext(ctx context.Context, query string, args ...interface{}) (_ Rows, err error) {
	s.record()
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	if !s.useContext {
		return s.wrapRows(s.tx.Query(query, args...)) //nolint: noctx, fallback for non-context behaviour
	}
	return s.wrapRows(s.tx.QueryContext(ctx, query, args...))
}

func (s *sqlTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	s.record()
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(nil)

	if !s.useContext {
		return s.tx.QueryRow(query, args...) //nolint: noctx, fallback for non-context behaviour
	}
	return s.tx.QueryRowContext(ctx, query, args...)
}

func (s *sqlTx) Commit() error {
	return errs.Combine(s.tracker.Close(), s.tx.Commit())
}

func (s *sqlTx) Rollback() error {
	s.record()
	return errs.Combine(s.tracker.Close(), s.tx.Rollback())
}

func (s *sqlTx) record() {
	if s.box == nil {
		return
	}

	s.box.Enqueue(flightrecorder.EventTypeDB, 1) // 1 to skip record call.
}
