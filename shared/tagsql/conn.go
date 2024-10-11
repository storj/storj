// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/leak"
	"storj.io/common/traces"
)

var (
	monConnWaiting = mon.Counter("sql_conn_waiting")
	monConnOpen    = mon.Counter("sql_conn_open")
)

// Conn is an interface for *sql.Conn-like connections.
type Conn interface {
	BeginTx(ctx context.Context, txOptions *sql.TxOptions) (Tx, error)
	Close() error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PingContext(ctx context.Context) error
	PrepareContext(ctx context.Context, query string) (Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Raw(ctx context.Context, f func(driverConn interface{}) error) (err error)
}

// TODO:
// Is there a way to call non-context versions on *sql.Conn?
// The pessimistic and safer assumption is that using any context may break
// lib/pq internally. It might be fine, however it's unclear, how fine it is.

// sqlConn implements Conn, which optionally disables contexts.
type sqlConn struct {
	conn         *sql.Conn
	useContext   bool
	useTxContext bool
	tracker      leak.Ref
	monReleased  bool
}

func (s *sqlConn) BeginTx(ctx context.Context, txOptions *sql.TxOptions) (Tx, error) {
	if txOptions != nil {
		return nil, errors.New("txOptions not supported")
	}
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlTx{
		tx:         tx,
		useContext: s.useContext && s.useTxContext,
		tracker:    s.tracker.Child("sqlTx", 1),
	}, nil
}

func (s *sqlConn) Close() error {
	if !s.monReleased {
		monConnOpen.Dec(1)
		s.monReleased = true
	}
	return errs.Combine(s.tracker.Close(), s.conn.Close())
}

func (s *sqlConn) ExecContext(ctx context.Context, query string, args ...interface{}) (_ sql.Result, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	return s.conn.ExecContext(ctx, query, args...)
}

func (s *sqlConn) PingContext(ctx context.Context) error {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	return s.conn.PingContext(ctx)
}

func (s *sqlConn) PrepareContext(ctx context.Context, query string) (_ Stmt, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query)(&err)

	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	stmt, err := s.conn.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &sqlStmt{
		stmt:       stmt,
		useContext: s.useContext,
		tracker:    s.tracker.Child("sqlStmt", 1),
	}, nil
}

func (s *sqlConn) QueryContext(ctx context.Context, query string, args ...interface{}) (_ Rows, err error) {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(&err)

	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	return s.wrapRows(s.conn.QueryContext(ctx, query, args...))
}

func (s *sqlConn) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	defer mon.Task()(&ctx, query, args)(nil)

	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	return s.conn.QueryRowContext(ctx, query, args...)
}

func (s *sqlConn) Raw(ctx context.Context, f func(driverConn interface{}) error) (err error) {
	traces.Tag(ctx, traces.TagDB)
	return s.conn.Raw(f)
}
