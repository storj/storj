// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/piecestore"
)

// ErrUsedSerials represents errors from the used serials database.
var ErrUsedSerials = errs.Class("usedserialsdb error")

// UsedSerialsDBName represents the database name.
const UsedSerialsDBName = "used_serial"

type usedSerialsDB struct {
	dbContainerImpl
}

// Add adds a serial to the database.
func (db *usedSerialsDB) Add(ctx context.Context, satelliteID storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.Exec(`
		INSERT INTO
			used_serial_(satellite_id, serial_number, expiration)
		VALUES(?, ?, ?)`, satelliteID, serialNumber, expiration.UTC())

	return ErrUsedSerials.Wrap(err)
}

// DeleteExpired deletes expired serial numbers
func (db *usedSerialsDB) DeleteExpired(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.Exec(`DELETE FROM used_serial_ WHERE expiration < ?`, now.UTC())
	return ErrUsedSerials.Wrap(err)
}

// IterateAll iterates all serials.
// Note, this will lock the database and should only be used during startup.
func (db *usedSerialsDB) IterateAll(ctx context.Context, fn piecestore.SerialNumberFn) (err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.Query(`SELECT satellite_id, serial_number, expiration FROM used_serial_`)
	if err != nil {
		return ErrUsedSerials.Wrap(err)
	}
	defer func() { err = errs.Combine(err, ErrUsedSerials.Wrap(rows.Close())) }()

	for rows.Next() {
		var satelliteID storj.NodeID
		var serialNumber storj.SerialNumber
		var expiration time.Time

		err := rows.Scan(&satelliteID, &serialNumber, &expiration)
		if err != nil {
			return ErrUsedSerials.Wrap(err)
		}

		fn(satelliteID, serialNumber, expiration)
	}

	return ErrUsedSerials.Wrap(rows.Err())
}
