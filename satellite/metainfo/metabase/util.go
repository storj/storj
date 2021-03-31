// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"github.com/zeebo/errs"

	"storj.io/storj/private/tagsql"
)

func withRows(rows tagsql.Rows, err error) func(func(tagsql.Rows) error) error {
	return func(callback func(tagsql.Rows) error) error {
		if err != nil {
			return err
		}
		err := callback(rows)
		return errs.Combine(rows.Err(), rows.Close(), err)
	}
}
