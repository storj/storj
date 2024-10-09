// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"errors"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"
	"google.golang.org/api/iterator"
)

var (
	// Error is the default error class for this package.
	Error = errs.Class("spannerutil")
	// ErrMultipleRows is returned when multiple rows are returned from
	// a query that expects no more than one.
	ErrMultipleRows = errs.Class("more than 1 row returned")
)

// CollectRows scans each row into a slice.
func CollectRows[T any](iter *spanner.RowIterator, scan func(row *spanner.Row, item *T) error) (rs []T, _ error) {
	defer iter.Stop()

	for {
		row, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			return rs, nil
		}
		if err != nil {
			return nil, Error.Wrap(err)
		}

		rs = slices.Grow(rs, 1)
		rs = rs[:len(rs)+1]

		err = scan(row, &rs[len(rs)-1])
		if err != nil {
			return nil, Error.New("scan failed: %w", err)
		}
	}
}

// CollectRow scans a single row query. It returns errors if the iterator doesn't have exactly one
// row.
func CollectRow[T any](iter *spanner.RowIterator, scan func(row *spanner.Row, item *T) error) (r T, _ error) {
	defer iter.Stop()

	row, err := iter.Next()
	if errors.Is(err, iterator.Done) {
		var zero T
		return zero, Error.New("no rows: %w", iterator.Done)
	}
	if err != nil {
		var zero T
		return zero, Error.Wrap(err)
	}

	err = scan(row, &r)
	if err != nil {
		var zero T
		return zero, Error.New("scan failed: %w", err)
	}

	_, errCheck := iter.Next()
	if errCheck == nil {
		var zero T
		return zero, ErrMultipleRows.New("")
	}
	if !errors.Is(errCheck, iterator.Done) {
		var zero T
		return zero, Error.New("failed checking for remaining rows")
	}

	return r, nil
}
