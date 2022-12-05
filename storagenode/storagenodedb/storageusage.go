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

	// hour_interval = current row interval_end_time - previous row interval_end_time
	// Rows with 0-hour difference are assumed to be 24 hours.
	query := `SELECT satellite_id,
					at_rest_total,
					COALESCE(
						(
							CAST(strftime('%s', interval_end_time) AS NUMERIC)
							-
							CAST(strftime('%s', LAG(interval_end_time) OVER (PARTITION BY satellite_id ORDER BY interval_end_time)) AS NUMERIC)
						) / 3600,
						24
					) AS hour_interval,
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
		var atRestTotal, intervalInHours float64
		var timestamp time.Time

		err = rows.Scan(&satellite, &atRestTotal, &intervalInHours, &timestamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			SatelliteID:      satellite,
			AtRestTotal:      atRestTotal,
			AtRestTotalBytes: atRestTotal / intervalInHours,
			IntervalInHours:  intervalInHours,
			IntervalStart:    timestamp,
		})
	}

	return stamps, rows.Err()
}

// GetDailyTotal returns daily storage usage stamps summed across all known satellites
// for provided time range.
func (db *storageUsageDB) GetDailyTotal(ctx context.Context, from, to time.Time) (_ []storageusage.Stamp, err error) {
	defer mon.Task()(&ctx)(&err)

	// hour_interval = current row interval_end_time - previous row interval_end_time
	// Rows with 0-hour difference are assumed to be 24 hours.
	query := `SELECT SUM(usages.at_rest_total), SUM(usages.hour_interval), usages.timestamp
				FROM (
					SELECT at_rest_total, timestamp,
							COALESCE(
								(
									CAST(strftime('%s', interval_end_time) AS NUMERIC)
									-
									CAST(strftime('%s', LAG(interval_end_time) OVER (PARTITION BY satellite_id ORDER BY interval_end_time)) AS NUMERIC)
								) / 3600,
								24
							) AS hour_interval
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
		var atRestTotal, intervalInHours float64
		var timestamp time.Time

		err = rows.Scan(&atRestTotal, &intervalInHours, &timestamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.Stamp{
			AtRestTotal:      atRestTotal,
			AtRestTotalBytes: atRestTotal / intervalInHours,
			IntervalInHours:  intervalInHours,
			IntervalStart:    timestamp,
		})
	}

	return stamps, rows.Err()
}

// Summary returns aggregated storage usage in Bytes*hour and average usage in bytes across all satellites.
func (db *storageUsageDB) Summary(ctx context.Context, from, to time.Time) (_, _ float64, err error) {
	defer mon.Task()(&ctx, from, to)(&err)
	var summary, averageUsageInBytes sql.NullFloat64

	query := `SELECT SUM(usages.at_rest_total), AVG(usages.at_rest_total_bytes)
				FROM (
					SELECT
						at_rest_total,
						at_rest_total / (
							COALESCE(
								(
									CAST(strftime('%s', interval_end_time) AS NUMERIC)
									-
									CAST(strftime('%s', LAG(interval_end_time) OVER (PARTITION BY satellite_id ORDER BY interval_end_time)) AS NUMERIC)
								) / 3600,
								24
							) 
						) AS at_rest_total_bytes
					FROM storage_usage
					WHERE ? <= timestamp AND timestamp <= ?
				) as usages`

	err = db.QueryRowContext(ctx, query, from.UTC(), to.UTC()).Scan(&summary, &averageUsageInBytes)
	return summary.Float64, averageUsageInBytes.Float64, err
}

// SatelliteSummary returns aggregated storage usage in Bytes*hour and average usage in bytes for a particular satellite.
func (db *storageUsageDB) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_, _ float64, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)
	var summary, averageUsageInBytes sql.NullFloat64

	query := `SELECT SUM(usages.at_rest_total), AVG(usages.at_rest_total_bytes)
				FROM (
					SELECT
						at_rest_total,
						at_rest_total / (
							COALESCE(
								(
									CAST(strftime('%s', interval_end_time) AS NUMERIC)
									-
									CAST(strftime('%s', LAG(interval_end_time) OVER (PARTITION BY satellite_id ORDER BY interval_end_time)) AS NUMERIC)
								) / 3600,
								24
							) 
						) AS at_rest_total_bytes
					FROM storage_usage
					WHERE satellite_id = ?
					AND ? <= timestamp AND timestamp <= ?
				) as usages`

	err = db.QueryRowContext(ctx, query, satelliteID, from.UTC(), to.UTC()).Scan(&summary, &averageUsageInBytes)
	return summary.Float64, averageUsageInBytes.Float64, err
}
