// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/shared/tagsql"
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

// uuidsToBytesArray converts []uuid.UUID into [][]byte.
func uuidsToBytesArray(uuidArr []uuid.UUID) (bytesArr [][]byte) {
	for _, v := range uuidArr {
		bytesArr = append(bytesArr, v.Bytes())
	}
	return
}
