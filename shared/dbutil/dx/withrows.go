// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dx

import (
	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

// WithRows wraps the (rows, err) result of a QueryContext call so a callback
// can process the rows without repeating the close / rows.Err / error
// plumbing. If err is non-nil it is returned immediately and the callback is
// not invoked; otherwise the callback runs and the returned error combines
// the callback error, rows.Err, and rows.Close.
//
// Usage:
//
//	err := dx.WithRows(db.QueryContext(ctx, "SELECT ..."))(func(rows dx.Rows) error {
//	    for rows.Next() {
//	        // ...
//	    }
//	    return nil
//	})
func WithRows(rows tagsql.Rows, err error) func(func(Rows) error) error {
	return func(callback func(Rows) error) error {
		if err != nil {
			return err
		}
		cberr := callback(rows)
		return errs.Combine(cberr, rows.Err(), rows.Close())
	}
}
