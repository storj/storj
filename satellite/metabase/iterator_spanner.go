// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO: should this go in tagsql before we have a full tagsql-for-spanner impl?

package metabase

import (
	"database/sql"
	"errors"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/iterator"

	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

type spannerRows struct {
	rowIterator *spanner.RowIterator
	row         *spanner.Row
	err         error
}

var _ = newSpannerRows // TODO(spanner): ignore staticcheck warning about unused func, remove afterwards

func newSpannerRows(rowIterator *spanner.RowIterator) *spannerRows {
	return &spannerRows{rowIterator: rowIterator}
}

func (sr *spannerRows) Close() error {
	sr.rowIterator.Stop()
	return nil
}

func (sr *spannerRows) Err() error {
	return sr.err
}

func (sr *spannerRows) Next() bool {
	sr.row, sr.err = sr.rowIterator.Next()
	if errors.Is(sr.err, iterator.Done) {
		sr.row = nil
		sr.err = nil
		return false
	}
	return true
}

func (sr *spannerRows) ColumnTypes() ([]*sql.ColumnType, error) {
	return nil, Error.New("ColumnTypes doesn't work here")
}

func (sr *spannerRows) Columns() ([]string, error) {
	if sr.row == nil {
		return nil, Error.New("no row found")
	}
	return sr.row.ColumnNames(), nil
}

func (sr *spannerRows) Scan(dest ...interface{}) error {
	if sr.err != nil {
		return sr.err
	}
	fields := make([]any, len(dest))
	for i, col := range dest {
		switch col := col.(type) {
		case *int32: // could do other int types, but this is the only one we currently run into
			fields[i] = spannerutil.Int(col)
		default:
			fields[i] = col
		}
	}
	return sr.row.Columns(fields...)
}

func (sr *spannerRows) NextResultSet() bool {
	return false
}

var _ tagsql.Rows = &spannerRows{}
