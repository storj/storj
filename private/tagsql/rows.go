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

func (t *tracker) wrapRows(rows *sql.Rows, err error) (Rows, error) {
	if rows == nil || err != nil {
		return nil, err
	}
	return &sqlRows{
		rows:    rows,
		tracker: t.child(2),
	}, err
}

type sqlRows struct {
	rows      *sql.Rows
	tracker   *tracker
	errcalled bool
}

func (s *sqlRows) Close() error {
	var errCalling error
	if !s.errcalled {
		var x strings.Builder
		fmt.Fprintf(&x, "--- rows.Err() was not called, for rows started at ---\n")
		fmt.Fprintf(&x, "%s", s.tracker.formatStack())
		fmt.Fprintf(&x, "--- Closing the rows at ---\n")
		fmt.Fprintf(&x, "%s", string(debug.Stack()))
		errCalling = errors.New(x.String())
	}
	return errs.Combine(errCalling, s.tracker.close(), s.rows.Close())
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
