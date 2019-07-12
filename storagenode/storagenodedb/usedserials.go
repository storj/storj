// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/piecestore"
)

type usedSerials struct {
	*InfoDB
}

// UsedSerials returns used serials database.
func (db *DB) UsedSerials() piecestore.UsedSerials { return db.info.UsedSerials() }

// UsedSerials returns used serials database.
func (db *InfoDB) UsedSerials() piecestore.UsedSerials { return &usedSerials{db} }

// Add adds a serial to the database.
func (db *usedSerials) Add(ctx context.Context, satelliteID storj.NodeID, serialNumber storj.SerialNumber, expiration time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Exec(`
		INSERT INTO
			used_serial(satellite_id, serial_number, expiration)
		VALUES(?, ?, ?)`, satelliteID, serialNumber, expiration)

	return ErrInfo.Wrap(err)
}

// DeleteExpired deletes expired serial numbers
func (db *usedSerials) DeleteExpired(ctx context.Context, now time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Exec(`DELETE FROM used_serial WHERE datetime(expiration) < datetime(?)`, now)
	return ErrInfo.Wrap(err)
}

// IterateAll iterates all serials.
// Note, this will lock the database and should only be used during startup.
func (db *usedSerials) IterateAll(ctx context.Context, fn piecestore.SerialNumberFn) (err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.Query(`SELECT satellite_id, serial_number, expiration FROM used_serial`)
	if err != nil {
		return ErrInfo.Wrap(err)
	}
	defer func() { err = errs.Combine(err, ErrInfo.Wrap(rows.Close())) }()

	for rows.Next() {
		var satelliteID storj.NodeID
		var serialNumber storj.SerialNumber
		var expiration time.Time

		err := rows.Scan(&satelliteID, &serialNumber, &expiration)
		if err != nil {
			return ErrInfo.Wrap(err)
		}

		fn(satelliteID, serialNumber, expiration)
	}

	return ErrInfo.Wrap(rows.Err())
}
