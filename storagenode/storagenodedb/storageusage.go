// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/shared/tagsql"
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

	return sqliteutil.WithTx(ctx, db.GetDB(), func(ctx context.Context, tx tagsql.Tx) error {
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
	query := `SELECT su1.satellite_id,
					su1.at_rest_total,
					COALESCE(
						(
							CAST(strftime('%s', su1.interval_end_time) AS NUMERIC)
							-
							CAST(strftime('%s', (
								SELECT interval_end_time
								FROM storage_usage
								WHERE satellite_id = su1.satellite_id
								AND timestamp < su1.timestamp
								ORDER BY timestamp DESC
								LIMIT 1
							)) AS NUMERIC)
						) / 3600,
						24
					) AS hour_interval,
					su1.timestamp
				FROM storage_usage su1
				WHERE su1.satellite_id = ?
				AND ? <= su1.timestamp AND su1.timestamp <= ?
				ORDER BY su1.timestamp ASC`

	rows, err := db.QueryContext(ctx, query, satelliteID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var stamps []storageusage.Stamp
	for rows.Next() {
		var satellite storj.NodeID
		var atRestTotal, intervalInHours sql.NullFloat64
		var timestamp time.Time

		err = rows.Scan(&satellite, &atRestTotal, &intervalInHours, &timestamp)
		if err != nil {
			return nil, err
		}

		atRestTotalBytes := float64(0)
		if intervalInHours.Float64 > 0 {
			atRestTotalBytes = atRestTotal.Float64 / intervalInHours.Float64
		}

		stamps = append(stamps, storageusage.Stamp{
			SatelliteID:      satellite,
			AtRestTotal:      atRestTotal.Float64,
			AtRestTotalBytes: atRestTotalBytes,
			IntervalInHours:  intervalInHours.Float64,
			IntervalStart:    timestamp,
		})
	}

	return stamps, rows.Err()
}

// GetDailyTotal returns daily storage usage stamps summed across all known satellites
// for provided time range.
func (db *storageUsageDB) GetDailyTotal(ctx context.Context, from, to time.Time) (_ []storageusage.StampGroup, err error) {
	defer mon.Task()(&ctx)(&err)

	// hour_interval = current row interval_end_time - previous row interval_end_time
	// Rows with 0-hour difference are assumed to be 24 hours.
	query := `SELECT SUM(su3.at_rest_total), SUM(su3.at_rest_total_bytes), su3.timestamp
				FROM (
					SELECT
						su1.at_rest_total,
						su1.at_rest_total / COALESCE(
							(
								CAST(strftime('%s', su1.interval_end_time) AS NUMERIC)
								-
								CAST(strftime('%s', (
									SELECT interval_end_time
									FROM storage_usage su2
									WHERE su2.satellite_id = su1.satellite_id
									AND su2.timestamp < su1.timestamp
									ORDER BY su2.timestamp DESC
									LIMIT 1
								)) AS NUMERIC)
							) / 3600,
							24
						) AS at_rest_total_bytes,
						su1.timestamp
					FROM storage_usage su1
					WHERE ? <= su1.timestamp AND su1.timestamp <= ?
				) as su3
				GROUP BY su3.timestamp
				ORDER BY su3.timestamp ASC`

	rows, err := db.QueryContext(ctx, query, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []storageusage.StampGroup
	for rows.Next() {
		var atRestTotal, atRestTotalBytes sql.NullFloat64
		var timestamp time.Time

		err = rows.Scan(&atRestTotal, &atRestTotalBytes, &timestamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, storageusage.StampGroup{
			AtRestTotal:      atRestTotal.Float64,
			AtRestTotalBytes: atRestTotalBytes.Float64,
			IntervalStart:    timestamp,
		})
	}

	return stamps, rows.Err()
}

// Summary returns aggregated storage usage in Bytes*hour and average usage in bytes across all satellites.
func (db *storageUsageDB) Summary(ctx context.Context, from, to time.Time) (_, _ float64, err error) {
	defer mon.Task()(&ctx, from, to)(&err)
	var summary, averageUsageInBytes sql.NullFloat64

	query := `SELECT SUM(at_rest_total), AVG(at_rest_total_bytes)
				FROM (
					SELECT
						SUM(su1.at_rest_total) AS at_rest_total,
						SUM(
							su1.at_rest_total / (
								COALESCE(
									(
										CAST(strftime('%s', su1.interval_end_time) AS NUMERIC)
										-
										CAST(strftime('%s', (
											SELECT interval_end_time
											FROM storage_usage su2
											WHERE su2.satellite_id = su1.satellite_id
											AND su2.timestamp < su1.timestamp
											ORDER BY su2.timestamp DESC
											LIMIT 1
										)) AS NUMERIC)
									) / 3600,
									24
								)
							)
						) AS at_rest_total_bytes,
						su1.timestamp
					FROM storage_usage su1
					WHERE ? <= su1.timestamp AND su1.timestamp <= ?
					GROUP BY timestamp
				) as su3`

	err = db.QueryRowContext(ctx, query, from.UTC(), to.UTC()).Scan(&summary, &averageUsageInBytes)
	return summary.Float64, averageUsageInBytes.Float64, err
}

// SatelliteSummary returns aggregated storage usage in Bytes*hour and average usage in bytes for a particular satellite.
func (db *storageUsageDB) SatelliteSummary(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_, _ float64, err error) {
	defer mon.Task()(&ctx, satelliteID, from, to)(&err)
	var summary, averageUsageInBytes sql.NullFloat64

	query := `SELECT SUM(su3.at_rest_total), AVG(su3.at_rest_total_bytes)
				FROM (
					SELECT
						su1.at_rest_total,
						(
							su1.at_rest_total / (
								COALESCE(
									(
										CAST(strftime('%s', su1.interval_end_time) AS NUMERIC)
										-
										CAST(strftime('%s', (
											SELECT interval_end_time
											FROM storage_usage su2
											WHERE su2.satellite_id = su1.satellite_id
											AND su2.timestamp < su1.timestamp
											ORDER BY su2.timestamp DESC
											LIMIT 1
										)) AS NUMERIC)
									) / 3600,
									24
								)
							)
						) AS at_rest_total_bytes
					FROM storage_usage su1
					WHERE su1.satellite_id = ?
					AND ? <= su1.timestamp AND su1.timestamp <= ?
				) as su3`

	err = db.QueryRowContext(ctx, query, satelliteID, from.UTC(), to.UTC()).Scan(&summary, &averageUsageInBytes)
	return summary.Float64, averageUsageInBytes.Float64, err
}
