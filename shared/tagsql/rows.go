// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package tagsql

import (
	"database/sql"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/leak"
)

// Rows implements a wrapper for *sql.Rows.
type Rows interface {
	Close() error
	ColumnTypes() ([]*sql.ColumnType, error)
	Columns() ([]string, error)
	Err() error
	Next() bool
	NextResultSet() bool
	Scan(dest ...interface{}) error
}

func wrapRows(tracker leak.Ref, rows *sql.Rows, err error) (Rows, error) {
	if rows == nil || err != nil {
		return nil, err
	}
	return &sqlRows{
		rows:    rows,
		tracker: tracker.Child("sqlRows", 3),
	}, err
}

func (x *sqlDB) wrapRows(rows *sql.Rows, err error) (Rows, error) {
	return wrapRows(x.tracker, rows, err)
}
func (x *sqlConn) wrapRows(rows *sql.Rows, err error) (Rows, error) {
	return wrapRows(x.tracker, rows, err)
}
func (x *sqlTx) wrapRows(rows *sql.Rows, err error) (Rows, error) {
	return wrapRows(x.tracker, rows, err)
}
func (x *sqlStmt) wrapRows(rows *sql.Rows, err error) (Rows, error) {
	return wrapRows(x.tracker, rows, err)
}

type sqlRows struct {
	rows      *sql.Rows
	tracker   leak.Ref
	errcalled bool
}

func (s *sqlRows) Close() error {
	var errCalling error
	if !s.errcalled {
		var x strings.Builder
		fmt.Fprintf(&x, "--- rows.Err() was not called, for rows started at ---\n")
		fmt.Fprintf(&x, "%s", s.tracker.StartStack())
		fmt.Fprintf(&x, "--- Closing the rows at ---\n")
		fmt.Fprintf(&x, "%s", string(debug.Stack()))
		errCalling = errors.New(x.String())
	}
	return errs.Combine(errCalling, s.tracker.Close(), s.rows.Close())
}

func (s *sqlRows) ColumnTypes() ([]*sql.ColumnType, error) {
	return s.rows.ColumnTypes()
}

func (s *sqlRows) Columns() ([]string, error) {
	return s.rows.Columns()
}

func (s *sqlRows) Err() error {
	s.errcalled = true
	return s.rows.Err()
}

func (s *sqlRows) Next() bool {
	s.errcalled = false
	return s.rows.Next()
}

func (s *sqlRows) NextResultSet() bool {
	return s.rows.NextResultSet()
}

func (s *sqlRows) Scan(dest ...interface{}) error {
	err := s.rows.Scan(dest...)
	if err != nil {
		s.errcalled = true
	}
	return err
}
