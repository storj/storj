// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
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

// intLimitRange defines a valid range (1,limit].
type intLimitRange int

// Ensure clamps v to a value between [1,limit].
func (limit intLimitRange) Ensure(v *int) {
	if *v <= 0 || *v > int(limit) {
		*v = int(limit)
	}
}

// Max returns maximum value for the given range.
func (limit intLimitRange) Max() int { return int(limit) }

// ensureRange ensures v is between min and max. It's sets to def, when the value is 0.
func ensureRange(v *int, def, min, max int) {
	switch {
	case *v == 0:
		*v = def
	case *v < min:
		*v = min
	case *v > max:
		*v = max
	}
}
