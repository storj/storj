// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/console"
)

// nodeStats works with node stats db
type nodeStats struct {
	*InfoDB
}

// Create inserts new stats into the db
func (db *nodeStats) Create(ctx context.Context, stats console.NodeStats) (_ *console.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := `INSERT INTO node_stats (
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
			) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`

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
		stats.UpdatedAt.UTC(),
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// Update updates stored stats
func (db *nodeStats) Update(ctx context.Context, stats console.NodeStats) (err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := `UPDATE node_stats
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
			WHERE satellite_id = ?`

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
		stats.UpdatedAt.UTC(),
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

// Get retrieves stats for specific satellite
func (db *nodeStats) Get(ctx context.Context, satelliteID storj.NodeID) (_ *console.NodeStats, err error) {
	defer mon.Task()(&ctx)(&err)

	stats := console.NodeStats{}

	row := db.db.QueryRowContext(ctx,
		`SELECT * FROM node_stats WHERE satellite_id = ?`,
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
