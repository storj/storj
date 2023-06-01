// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"storj.io/private/tagsql"
)

// withRows ensures that rows get properly closed after the callback finishes.
func withRows(rows tagsql.Rows, err error) func(func(tagsql.Rows) error) error {
	return func(callback func(tagsql.Rows) error) error {
		if err != nil {
			return err
		}
		err := callback(rows)
		return errs.Combine(rows.Err(), rows.Close(), err)
	}
}

// convertSlice converts xs by applying fn to each element.
// If there's an error during conversion, the function
// returns an empty slice and the error.
func convertSlice[In, Out any](xs []In, fn func(In) (Out, error)) ([]Out, error) {
	rs := make([]Out, len(xs))
	for i := range xs {
		var err error
		rs[i], err = fn(xs[i])
		if err != nil {
			return nil, err
		}
	}
	return rs, nil
}
