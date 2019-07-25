// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/console"
)

// diskSpaceUsage works disk space usage db
// which caches data from satellite rollups
type diskSpaceUsage struct {
	*InfoDB
}

// Store stores disk space usage stamps to db
func (db *diskSpaceUsage) Store(ctx context.Context, stamps []console.DiskSpaceUsage) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(stamps) == 0 {
		return nil
	}

	stmt := `INSERT OR REPLACE INTO rollup_disk_storage_usages(satellite_id, at_rest_total, timestamp) 
			VALUES(?,?,?)`

	cb := func(tx *sql.Tx) error {
		txStmt, err := tx.PrepareContext(ctx, stmt)
		if err != nil {
			return err
		}

		defer func() {
			err = errs.Combine(err, txStmt.Close())
		}()

		for _, stamp := range stamps {
			_, err = txStmt.Exec(stamp.SatelliteID, stamp.AtRestTotal, stamp.Timestamp.UTC())

			if err != nil {
				return err
			}
		}

		return nil
	}

	return db.withTx(ctx, cb)
}

// GetDaily returns daily disk usage for particular satellite
// for provided time range
func (db *diskSpaceUsage) GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []console.DiskSpaceUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT *
				FROM rollup_disk_storage_usages
				WHERE timestamp IN (
					SELECT MAX(timestamp) 
					FROM rollup_disk_storage_usages
					WHERE satellite_id = ?
					AND ? <= timestamp AND timestamp <= ?
					GROUP BY DATE(timestamp)
				)`

	rows, err := db.db.QueryContext(ctx, query, satelliteID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []console.DiskSpaceUsage
	for rows.Next() {
		var satellite storj.NodeID
		var atRestTotal float64
		var timeStamp time.Time

		err = rows.Scan(&satellite, &atRestTotal, &timeStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, console.DiskSpaceUsage{
			SatelliteID: satellite,
			AtRestTotal: atRestTotal,
			Timestamp:   timeStamp,
		})
	}

	return stamps, nil
}

// GetDailyTotal returns daily disk usage summed across all known satellites
// for provided time range
func (db *diskSpaceUsage) GetDailyTotal(ctx context.Context, from, to time.Time) (_ []console.DiskSpaceUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT SUM(at_rest_total), timestamp 
				FROM rollup_disk_storage_usages
				WHERE timestamp IN (
					SELECT MAX(timestamp)
					FROM rollup_disk_storage_usages
					WHERE ? <= timestamp AND timestamp <= ?
					GROUP BY DATE(timestamp), satellite_id
				) GROUP BY DATE(timestamp)`

	rows, err := db.db.QueryContext(ctx, query, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []console.DiskSpaceUsage
	for rows.Next() {
		var atRestTotal float64
		var timeStamp time.Time

		err = rows.Scan(&atRestTotal, &timeStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, console.DiskSpaceUsage{
			AtRestTotal: atRestTotal,
			Timestamp:   timeStamp,
		})
	}

	return stamps, nil
}
