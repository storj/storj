// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/storageusage"
)

// StorageUsageDBName represents the database name.
const StorageUsageDBName = "storage_usage"

// storageUsageDB storage usage DB
type storageUsageDB struct {
	dbContainerImpl
}

// Store stores storage usage stamps to db replacing conflicting entries
func (db *storageUsageDB) Store(ctx context.Context, stamps []storageusage.Stamp) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(stamps) == 0 {
		return nil
	}

	query := `INSERT OR REPLACE INTO storage_usage(satellite_id, at_rest_total, interval_start)
			VALUES(?,?,?)`

	return withTx(ctx, db.GetDB(), func(tx *sql.Tx) error {
		for _, stamp := range stamps {
			_, err = tx.ExecContext(ctx, query, stamp.SatelliteID, stamp.AtRestTotal, stamp.IntervalStart.UTC())

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// GetDaily returns daily storage usage stamps for particular satellite
// for provided time range
func (db *storageUsageDB) GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT satellite_id,
					SUM(at_rest_total),
					interval_start
				FROM storage_usage
				WHERE satellite_id = ?
				AND ? <= interval_start AND interval_start <= ?
				GROUP BY DATE(interval_start)
				ORDER BY interval_start`

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
		var intervalStart time.Time

		err = rows.Scan(&satellite, &atRestTotal, &intervalStart)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			SatelliteID:   satellite,
			AtRestTotal:   atRestTotal,
			IntervalStart: intervalStart,
		})
	}

	return stamps, nil
}

// GetDailyTotal returns daily storage usage stamps summed across all known satellites
// for provided time range
func (db *storageUsageDB) GetDailyTotal(ctx context.Context, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT SUM(at_rest_total), interval_start
				FROM storage_usage
				WHERE ? <= interval_start AND interval_start <= ?
				GROUP BY DATE(interval_start)
				ORDER BY interval_start`

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
		var intervalStart time.Time

		err = rows.Scan(&atRestTotal, &intervalStart)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			AtRestTotal:   atRestTotal,
			IntervalStart: intervalStart,
		})
	}

	return stamps, nil
}

// Summary returns aggregated storage usage across all satellites.
func (db *storageUsageDB) Summary(ctx context.Context, from, to time.Time) (_ float64, err error) {
	defer mon.Task()(&ctx, from, to)(&err)
	var summary sql.NullFloat64

	query := `SELECT SUM(at_rest_total)
				FROM storage_usage
				WHERE ? <= interval_start AND interval_start <= ?`

	err = db.QueryRowContext(ctx, query, from.UTC(), to.UTC()).Scan(&summary)
	return summary.Float64, err
}

// SatelliteSummary returns aggregated storage usage for a particular satellite.
func (db *storageUsageDB) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ float64, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)
	var summary sql.NullFloat64

	query := `SELECT SUM(at_rest_total)
				FROM storage_usage
				WHERE satellite_id = ?
				AND ? <= interval_start AND interval_start <= ?`

	err = db.QueryRowContext(ctx, query, satelliteID, from.UTC(), to.UTC()).Scan(&summary)
	return summary.Float64, err
}
