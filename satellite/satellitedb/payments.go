// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

//database implements DB
type payments struct {
	db *dbx.DB
}

// Query ... TODO
func (db *payments) Query(ctx context.Context, start time.Time, end time.Time) error {
	// tx, err := db.db.Open(ctx)
	// if err != nil {
	// 	return Error.Wrap(err)
	// }

	return nil
}
