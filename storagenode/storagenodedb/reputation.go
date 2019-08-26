// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/reputation"
)

// ErrReputation represents errors from the reputation database.
var ErrReputation = errs.Class("reputation error")

// reputation works with node reputation DB
type reputationDB struct {
	location string
	SQLDB
}

// newReputationDB returns a new instance of reputationDB initialized with the specified database.
func newReputationDB(db SQLDB, location string) *reputationDB {
	return &reputationDB{
		location: location,
		SQLDB:    db,
	}
}

// Store inserts or updates reputation stats into the db
func (db *reputationDB) Store(ctx context.Context, stats reputation.Stats) (err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := `INSERT OR REPLACE INTO reputation (
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

	_, err = db.ExecContext(ctx, stmt,
		stats.SatelliteID,
		stats.Uptime.SuccessCount,
		stats.Uptime.TotalCount,
		stats.Uptime.Alpha,
		stats.Uptime.Beta,
		stats.Uptime.Score,
		stats.Audit.SuccessCount,
		stats.Audit.TotalCount,
		stats.Audit.Alpha,
		stats.Audit.Beta,
		stats.Audit.Score,
		stats.UpdatedAt.UTC(),
	)

	return nil
}

// Get retrieves stats for specific satellite
func (db *reputationDB) Get(ctx context.Context, satelliteID storj.NodeID) (_ *reputation.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	stats := reputation.Stats{}

	row := db.QueryRowContext(ctx,
		`SELECT * FROM reputation WHERE satellite_id = ?`,
		satelliteID,
	)

	err = row.Scan(&stats.SatelliteID,
		&stats.Uptime.SuccessCount,
		&stats.Uptime.TotalCount,
		&stats.Uptime.Alpha,
		&stats.Uptime.Beta,
		&stats.Uptime.Score,
		&stats.Audit.SuccessCount,
		&stats.Audit.TotalCount,
		&stats.Audit.Alpha,
		&stats.Audit.Beta,
		&stats.Audit.Score,
		&stats.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
