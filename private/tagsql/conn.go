// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/private/traces"
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

// ConnWithoutTxContext wraps *sql.Conn.
func ConnWithoutTxContext(conn *sql.Conn) Conn {
	return &sqlConn{conn: conn, useContext: true, useTxContext: false}
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
	tracker      *tracker
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
		tracker:    s.tracker.child(1),
	}, nil
}

func (s *sqlConn) Close() error {
	return errs.Combine(s.tracker.close(), s.conn.Close())
}

func (s *sqlConn) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	traces.Tag(ctx, traces.TagDB)
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

func (s *sqlConn) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	traces.Tag(ctx, traces.TagDB)
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
		tracker:    s.tracker.child(1),
	}, nil
}

func (s *sqlConn) QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	return s.tracker.wrapRows(s.conn.QueryContext(ctx, query, args...))
}

func (s *sqlConn) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	traces.Tag(ctx, traces.TagDB)
	if !s.useContext {
		ctx = context2.WithoutCancellation(ctx)
	}
	return s.conn.QueryRowContext(ctx, query, args...)
}

func (s *sqlConn) Raw(ctx context.Context, f func(driverConn interface{}) error) (err error) {
	traces.Tag(ctx, traces.TagDB)
	return s.conn.Raw(f)
}
