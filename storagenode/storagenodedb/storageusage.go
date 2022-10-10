// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/private/tagsql"
	"storj.io/storj/storagenode/storageusage"
)

// StorageUsageDBName represents the database name.
const StorageUsageDBName = "storage_usage"

// storageUsageDB storage usage DB.
type storageUsageDB struct {
	dbContainerImpl
}

// Store stores storage usage stamps to db replacing conflicting entries.
func (db *storageUsageDB) Store(ctx context.Context, stamps []storageusage.Stamp) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(stamps) == 0 {
		return nil
	}

	query := `INSERT OR REPLACE INTO storage_usage(satellite_id, at_rest_total, interval_end_time, timestamp)
			VALUES(?,?,?,?)`

	return withTx(ctx, db.GetDB(), func(tx tagsql.Tx) error {
		for _, stamp := range stamps {
			_, err = tx.ExecContext(ctx, query, stamp.SatelliteID, stamp.AtRestTotal, stamp.IntervalEndTime.UTC(), stamp.IntervalStart.UTC())

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// GetDaily returns daily storage usage stamps for particular satellite
// for provided time range.
func (db *storageUsageDB) GetDaily(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	// the at_rest_total is in bytes*hours, so to find the total number
	// of hours used to get the at_rest_total, we find the hour difference,
	// between the interval_end_time of a row and that of the previous row
	// and divide the at_rest_total by the hour interval and multiply by 24 hours
	// 24 hours to estimate the value for a 24hour time window.
	// i.e. 24 * (at_rest_total/hour_difference), where the
	// hour_difference = current row interval_end_time - previous row interval_end_time
	// Rows with 0-hour difference are assumed to be 24 hours.
	query := `SELECT satellite_id,
					 COALESCE(24 * (at_rest_total / COALESCE((CAST(strftime('%s', interval_end_time) AS NUMERIC) - CAST(strftime('%s', LAG(interval_end_time) OVER (PARTITION BY satellite_id ORDER BY interval_end_time)) AS NUMERIC)) / 3600, 24)), at_rest_total),
					 timestamp
				FROM storage_usage
				WHERE satellite_id = ?
				AND ? <= timestamp AND timestamp <= ?
				ORDER BY timestamp`

	rows, err := db.QueryContext(ctx, query, satelliteID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var stamps []storageusage.Stamp
	for rows.Next() {
		var satellite storj.NodeID
		var atRestTotal float64
		var timestamp time.Time

		err = rows.Scan(&satellite, &atRestTotal, &timestamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			SatelliteID:   satellite,
			AtRestTotal:   atRestTotal,
			IntervalStart: timestamp,
		})
	}

	return stamps, rows.Err()
}

// GetDailyTotal returns daily storage usage stamps summed across all known satellites
// for provided time range.
func (db *storageUsageDB) GetDailyTotal(ctx context.Context, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	// the at_rest_total is in bytes*hours, so to find the total number
	// of hours used to get the at_rest_total, we find the hour difference,
	// between the interval_end_time of a row and that of the previous row
	// and divide the at_rest_total by the hour interval and multiply by 24 hours
	// 24 hours to estimate the value for a 24hour time window.
	// i.e. 24 * (at_rest_total/hour_difference), where the
	// hour_difference = current row interval_end_time - previous row interval_end_time
	// Rows with 0-hour difference are assumed to be 24 hours.
	query := `SELECT SUM(usages.at_rest_total), usages.timestamp
				FROM (
					SELECT timestamp,
						   COALESCE(24 * (at_rest_total / COALESCE((CAST(strftime('%s', interval_end_time) AS NUMERIC) - CAST(strftime('%s', LAG(interval_end_time) OVER (PARTITION BY satellite_id ORDER BY interval_end_time)) AS NUMERIC)) / 3600, 24)), at_rest_total) AS at_rest_total
					FROM storage_usage
					WHERE ? <= timestamp AND timestamp <= ?
				) as usages
				GROUP BY usages.timestamp
				ORDER BY usages.timestamp`

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
		var timestamp time.Time

		err = rows.Scan(&atRestTotal, &timestamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			AtRestTotal:   atRestTotal,
			IntervalStart: timestamp,
		})
	}

	return stamps, rows.Err()
}

// Summary returns aggregated storage usage across all satellites.
func (db *storageUsageDB) Summary(ctx context.Context, from, to time.Time) (_ float64, err error) {
	defer mon.Task()(&ctx, from, to)(&err)
	var summary sql.NullFloat64

	query := `SELECT SUM(at_rest_total)
				FROM storage_usage
				WHERE ? <= timestamp AND timestamp <= ?`

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
				AND ? <= timestamp AND timestamp <= ?`

	err = db.QueryRowContext(ctx, query, satelliteID, from.UTC(), to.UTC()).Scan(&summary)
	return summary.Float64, err
}
