// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/storageusage"
)

// storageusageDB storage usage DB
type storageusageDB struct {
	location string
	SQLDB
}

// newStorageusageDB returns a new instance of storageusageDB initialized with the specified database.
func newStorageusageDB(db SQLDB, location string) *storageusageDB {
	return &storageusageDB{
		location: location,
		SQLDB:    db,
	}
}

// Store stores storage usage stamps to db replacing conflicting entries
func (db *storageusageDB) Store(ctx context.Context, stamps []storageusage.Stamp) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(stamps) == 0 {
		return nil
	}

	query := `INSERT OR REPLACE INTO storage_usage(satellite_id, at_rest_total, timestamp) 
			VALUES(?,?,?)`

	return db.withTx(ctx, func(tx *sql.Tx) error {
		for _, stamp := range stamps {
			_, err = db.ExecContext(ctx, query, stamp.SatelliteID, stamp.AtRestTotal, stamp.Timestamp.UTC())

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// GetDaily returns daily storage usage stamps for particular satellite
// for provided time range
func (db *storageusageDB) GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT *
				FROM storage_usage
				WHERE timestamp IN (
					SELECT MAX(timestamp) 
					FROM storage_usage
					WHERE satellite_id = ?
					AND ? <= timestamp AND timestamp <= ?
					GROUP BY DATE(timestamp)
				)`

	rows, err := db.QueryContext(ctx, query, satelliteID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []storageusage.Stamp
	for rows.Next() {
		var satellite storj.NodeID
		var atRestTotal float64
		var timeStamp time.Time

		err = rows.Scan(&satellite, &atRestTotal, &timeStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			SatelliteID: satellite,
			AtRestTotal: atRestTotal,
			Timestamp:   timeStamp,
		})
	}

	return stamps, nil
}

// GetDailyTotal returns daily storage usage stamps summed across all known satellites
// for provided time range
func (db *storageusageDB) GetDailyTotal(ctx context.Context, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT SUM(at_rest_total), timestamp 
				FROM storage_usage
				WHERE timestamp IN (
					SELECT MAX(timestamp)
					FROM storage_usage
					WHERE ? <= timestamp AND timestamp <= ?
					GROUP BY DATE(timestamp), satellite_id
				) GROUP BY DATE(timestamp)`

	rows, err := db.QueryContext(ctx, query, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []storageusage.Stamp
	for rows.Next() {
		var atRestTotal float64
		var timeStamp time.Time

		err = rows.Scan(&atRestTotal, &timeStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			AtRestTotal: atRestTotal,
			Timestamp:   timeStamp,
		})
	}

	return stamps, nil
}

// withTx is a helper method which executes callback in transaction scope
func (db *storageusageDB) withTx(ctx context.Context, cb func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()

	return cb(tx)
}
