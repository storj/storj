// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/console"
)

type consoledb struct{ *InfoDB }

// Console returns console.DB
func (db *InfoDB) Console() console.DB { return &consoledb{db} }

// Console returns console.DB
func (db *DB) Console() console.DB { return db.info.Console() }

// GetSatelliteIDs returns list of satelliteIDs that storagenode has interacted with
// at least once
func (db *consoledb) GetSatelliteIDs(ctx context.Context, from, to time.Time) (_ storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	var satellites storj.NodeIDList

	rows, err := db.db.QueryContext(ctx, db.Rebind(`
		SELECT DISTINCT satellite_id
		FROM bandwidth_usage
		WHERE ? <= created_at AND created_at <= ?`), from, to)

	if err != nil {
		if err == sql.ErrNoRows {
			return satellites, nil
		}
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var satelliteID storj.NodeID
		if err = rows.Scan(&satelliteID); err != nil {
			return nil, err
		}

		satellites = append(satellites, satelliteID)
	}

	return satellites, nil
}

// CreateStats inserts new stats into the db
func (db *consoledb) CreateStats(ctx context.Context, stats console.Stats) (_ *console.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := db.Rebind(`INSERT INTO node_stats (
				satellite_id, 
				uptime_success_count,
				uptime_total_count,
				uptime_reputation_alpha,
				uptime_reputation_beta,
				uptime_reputation_score,
				audit_success_count,
				audit_total_count,
				audit_reputation_alpha,
				audit_reputation_beta,
				audit_reputation_score,
				updated_at
			) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)

	_, err = db.db.ExecContext(ctx, stmt,
		stats.SatelliteID,
		stats.UptimeCheck.SuccessCount,
		stats.UptimeCheck.TotalCount,
		stats.UptimeCheck.ReputationAlpha,
		stats.UptimeCheck.ReputationBeta,
		stats.UptimeCheck.ReputationScore,
		stats.AuditCheck.SuccessCount,
		stats.AuditCheck.TotalCount,
		stats.AuditCheck.ReputationAlpha,
		stats.AuditCheck.ReputationBeta,
		stats.AuditCheck.ReputationScore,
		stats.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// UpdateStats updates stored stats
func (db *consoledb) UpdateStats(ctx context.Context, stats console.Stats) (err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := db.Rebind(`UPDATE node_stats
			SET uptime_success_count = ?,
				uptime_total_count = ?,
				uptime_reputation_alpha = ?,
				uptime_reputation_beta = ?,
				uptime_reputation_score = ?,
				audit_success_count = ?,
				audit_total_count = ?,
				audit_reputation_alpha = ?,
				audit_reputation_beta = ?,
				audit_reputation_score = ?,
				updated_at = ?
			WHERE satellite_id = ?`)

	res, err := db.db.ExecContext(ctx, stmt,
		stats.UptimeCheck.SuccessCount,
		stats.UptimeCheck.TotalCount,
		stats.UptimeCheck.ReputationAlpha,
		stats.UptimeCheck.ReputationBeta,
		stats.UptimeCheck.ReputationScore,
		stats.AuditCheck.SuccessCount,
		stats.AuditCheck.TotalCount,
		stats.AuditCheck.ReputationAlpha,
		stats.AuditCheck.ReputationBeta,
		stats.AuditCheck.ReputationScore,
		stats.UpdatedAt,
		stats.SatelliteID,
	)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetStatsSatellite retrieves stats for specific satellite
func (db *consoledb) GetStatsSatellite(ctx context.Context, satelliteID storj.NodeID) (_ *console.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	stats := console.Stats{}

	row := db.db.QueryRowContext(ctx,
		db.Rebind(`SELECT * FROM node_stats WHERE satellite_id = ?`),
		satelliteID,
	)

	err = row.Scan(&stats.SatelliteID,
		&stats.UptimeCheck.SuccessCount,
		&stats.UptimeCheck.TotalCount,
		&stats.UptimeCheck.ReputationAlpha,
		&stats.UptimeCheck.ReputationBeta,
		&stats.UptimeCheck.ReputationScore,
		&stats.AuditCheck.SuccessCount,
		&stats.AuditCheck.TotalCount,
		&stats.AuditCheck.ReputationAlpha,
		&stats.AuditCheck.ReputationBeta,
		&stats.AuditCheck.ReputationScore,
		&stats.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// StoreSpaceUsageStamps stores disk space usage stamps to db
func (db *consoledb) StoreSpaceUsageStamps(ctx context.Context, stamps []console.SpaceUsageStamp) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(stamps) == 0 {
		return nil
	}

	stmt := db.Rebind(`INSERT OR REPLACE INTO rollup_space_usages(rollup_id, satellite_id, at_rest_total, timestamp) 
			VALUES(?,?,?,?)`)

	cb := func(tx *sql.Tx) error {
		txStmt, err := tx.PrepareContext(ctx, stmt)
		if err != nil {
			return err
		}

		for _, stamp := range stamps {
			_, err = txStmt.Exec(stamp.RollupID, stamp.SatelliteID, stamp.AtRestTotal, stamp.Timestamp)

			if err != nil {
				return err
			}
		}

		return nil
	}

	return db.withTx(ctx, cb)
}

// GetDailyBandwidthUsed returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order
func (db *consoledb) GetDailyTotalBandwidthUsed(ctx context.Context, from, to time.Time) (_ []console.BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	since, _ := getDateEdges(from.UTC())
	_, before := getDateEdges(to.UTC())

	return db.getDailyBandwidthUsed(ctx,
		"WHERE ? <= created_at AND created_at <= ?",
		since, before)
}

// GetDailyBandwidthUsed returns slice of daily bandwidth usage for provided time range,
// sorted in ascending order for particular satellite
func (db *consoledb) GetDailyBandwidthUsed(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []console.BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	since, _ := getDateEdges(from.UTC())
	_, before := getDateEdges(to.UTC())

	return db.getDailyBandwidthUsed(ctx,
		"WHERE satellite_id = ? AND ? <= created_at AND created_at <= ?",
		satelliteID, since, before)
}

// getDailyBandwidthUsed returns slice of grouped by date bandwidth usage
// sorted in ascending order and applied condition if any
func (db *consoledb) getDailyBandwidthUsed(ctx context.Context, cond string, args ...interface{}) (_ []console.BandwidthUsed, err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.Rebind(`
		SELECT action, SUM(amount), created_at
		FROM bandwidth_usage
		` + cond + `
		GROUP BY DATE(created_at), action
		ORDER BY created_at ASC
	`)

	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var dates []time.Time
	dailyBandwidth := make(map[time.Time]*console.BandwidthUsed, 0)

	for rows.Next() {
		var action int32
		var amount int64
		var createdAt time.Time

		err = rows.Scan(&action, &amount, &createdAt)
		if err != nil {
			return nil, err
		}

		from, to := getDateEdges(createdAt)

		bandwidthUsed, ok := dailyBandwidth[from]
		if !ok {
			bandwidthUsed = &console.BandwidthUsed{
				From: from,
				To:   to,
			}

			dates = append(dates, from)
			dailyBandwidth[from] = bandwidthUsed
		}

		switch pb.PieceAction(action) {
		case pb.PieceAction_GET:
			bandwidthUsed.Egress.Usage = amount
		case pb.PieceAction_GET_AUDIT:
			bandwidthUsed.Egress.Audit = amount
		case pb.PieceAction_GET_REPAIR:
			bandwidthUsed.Egress.Repair = amount
		case pb.PieceAction_PUT:
			bandwidthUsed.Ingress.Usage = amount
		case pb.PieceAction_PUT_REPAIR:
			bandwidthUsed.Ingress.Repair = amount
		}
	}

	var bandwidthUsedList []console.BandwidthUsed
	for _, date := range dates {
		bandwidthUsedList = append(bandwidthUsedList, *dailyBandwidth[date])
	}

	return bandwidthUsedList, nil
}

// GetDailyDiskSpaceUsageTotal returns daily disk usage summed across all known satellites
// for provided time range
func (db *consoledb) GetDailyDiskSpaceUsageTotal(ctx context.Context, from, to time.Time) (_ []console.SpaceUsageStamp, err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.Rebind(`SELECT SUM(at_rest_total), timestamp 
							FROM rollup_space_usages
							WHERE rollup_id IN (
								SELECT MAX(rollup_id)
								FROM rollup_space_usages
								WHERE ? <= timestamp AND timestamp <= ?
								GROUP BY DATE(timestamp), satellite_id
							) GROUP BY DATE(timestamp)`)

	rows, err := db.db.QueryContext(ctx, query, from, to)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []console.SpaceUsageStamp
	for rows.Next() {
		var atRestTotal float64
		var timeStamp time.Time

		err = rows.Scan(&atRestTotal, &timeStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, console.SpaceUsageStamp{
			AtRestTotal: atRestTotal,
			Timestamp:   timeStamp,
		})
	}

	return stamps, nil
}

// GetDailyDiskSpaceUsageSatellite returns daily disk usage for particular satellite
// for provided time range
func (db *consoledb) GetDailyDiskSpaceUsageSatellite(ctx context.Context, satelliteID storj.NodeID, from, to time.Time) (_ []console.SpaceUsageStamp, err error) {
	defer mon.Task()(&ctx)(&err)

	query := db.Rebind(`SELECT *
							FROM rollup_space_usages
							WHERE rollup_id IN (
								SELECT MAX(rollup_id) 
								FROM rollup_space_usages
								WHERE satellite_id = ?
								AND ? <= timestamp AND timestamp <= ?
								GROUP BY DATE(timestamp)
							)`)

	rows, err := db.db.QueryContext(ctx, query, satelliteID, from, to)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var stamps []console.SpaceUsageStamp
	for rows.Next() {
		var rollupID int64
		var satellite storj.NodeID
		var atRestTotal float64
		var timeStamp time.Time

		err = rows.Scan(&rollupID, &satellite, &atRestTotal, &timeStamp)
		if err != nil {
			return nil, err
		}

		stamps = append(stamps, console.SpaceUsageStamp{
			RollupID:    rollupID,
			SatelliteID: satellite,
			AtRestTotal: atRestTotal,
			Timestamp:   timeStamp,
		})
	}

	return stamps, nil
}

// withTx is a helper method which executes callback in transaction scope
func (db *consoledb) withTx(ctx context.Context, cb func(tx *sql.Tx) error) error {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
		}

		err = tx.Commit()
	}()

	return cb(tx)
}

// getDateEdges returns start and end of the provided day
func getDateEdges(t time.Time) (time.Time, time.Time) {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC),
		time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, -1, time.UTC)
}
