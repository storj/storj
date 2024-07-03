// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"github.com/jackc/pgtype"
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

// pgtype_ObjectKeyArray returns an object usable by pg drivers for passing a []ObjectKey slice
// into a database as type BYTEA[].
func pgtype_ObjectKeyArray(objectKeysArray []ObjectKey) *pgtype.ByteaArray {
	pgtypeByteaArray := make([]pgtype.Bytea, len(objectKeysArray))
	for i, key := range objectKeysArray {
		pgtypeByteaArray[i].Bytes = []byte(key)
		pgtypeByteaArray[i].Status = pgtype.Present
	}
	return &pgtype.ByteaArray{
		Elements:   pgtypeByteaArray,
		Dimensions: []pgtype.ArrayDimension{{Length: int32(len(objectKeysArray)), LowerBound: 1}},
		Status:     pgtype.Present,
	}
}

// spanner_ObjectKeyArray returns an object usable by spanner drivers for passing a []ObjectKey slice
// into a database as type BYTES[].
func spanner_ObjectKeyArray(objectKeysArray []ObjectKey) [][]byte {
	if objectKeysArray == nil {
		return nil
	}

	r := make([][]byte, len(objectKeysArray))
	for i, key := range objectKeysArray {
		r[i] = []byte(key)
	}
	return r
}
