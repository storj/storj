// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/reputation"
)

// ErrReputation represents errors from the reputation database.
var ErrReputation = errs.Class("reputation error")

// ReputationDBName represents the database name.
const ReputationDBName = "reputation"

// reputation works with node reputation DB.
type reputationDB struct {
	dbContainerImpl
}

// Store inserts or updates reputation stats into the db.
func (db *reputationDB) Store(ctx context.Context, stats reputation.Stats) (err error) {
	defer mon.Task()(&ctx)(&err)

	query := `INSERT OR REPLACE INTO reputation (
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
			audit_unknown_reputation_alpha,
			audit_unknown_reputation_beta,
			audit_unknown_reputation_score,
			online_score,
			disqualified_at,
			suspended_at,
			offline_suspended_at,
			offline_under_review_at,
			updated_at,
			joined_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	// ensure we insert utc
	if stats.DisqualifiedAt != nil {
		utc := stats.DisqualifiedAt.UTC()
		stats.DisqualifiedAt = &utc
	}
	if stats.SuspendedAt != nil {
		utc := stats.SuspendedAt.UTC()
		stats.SuspendedAt = &utc
	}
	if stats.OfflineSuspendedAt != nil {
		utc := stats.OfflineSuspendedAt.UTC()
		stats.OfflineSuspendedAt = &utc
	}
	if stats.OfflineUnderReviewAt != nil {
		utc := stats.OfflineUnderReviewAt.UTC()
		stats.OfflineUnderReviewAt = &utc
	}

	_, err = db.ExecContext(ctx, query,
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
		stats.Audit.UnknownAlpha,
		stats.Audit.UnknownBeta,
		stats.Audit.UnknownScore,
		stats.OnlineScore,
		stats.DisqualifiedAt,
		stats.SuspendedAt,
		stats.OfflineSuspendedAt,
		stats.OfflineUnderReviewAt,
		stats.UpdatedAt.UTC(),
		stats.JoinedAt.UTC(),
	)

	return ErrReputation.Wrap(err)
}

// Get retrieves stats for specific satellite.
func (db *reputationDB) Get(ctx context.Context, satelliteID storj.NodeID) (_ *reputation.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	stats := reputation.Stats{
		SatelliteID: satelliteID,
	}

	row := db.QueryRowContext(ctx,
		`SELECT uptime_success_count,
			uptime_total_count,
			uptime_reputation_alpha,
			uptime_reputation_beta,
			uptime_reputation_score,
			audit_success_count,
			audit_total_count,
			audit_reputation_alpha,
			audit_reputation_beta,
			audit_reputation_score,
			audit_unknown_reputation_alpha,
			audit_unknown_reputation_beta,
			audit_unknown_reputation_score,
			online_score,
			disqualified_at,
			suspended_at,
			offline_suspended_at,
			offline_under_review_at,
			updated_at,
			joined_at
		FROM reputation WHERE satellite_id = ?`,
		satelliteID,
	)

	err = row.Scan(
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
		&stats.Audit.UnknownAlpha,
		&stats.Audit.UnknownBeta,
		&stats.Audit.UnknownScore,
		&stats.OnlineScore,
		&stats.DisqualifiedAt,
		&stats.SuspendedAt,
		&stats.OfflineSuspendedAt,
		&stats.OfflineUnderReviewAt,
		&stats.UpdatedAt,
		&stats.JoinedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}

	return &stats, ErrReputation.Wrap(err)
}

// All retrieves all stats from DB.
func (db *reputationDB) All(ctx context.Context) (_ []reputation.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT satellite_id,
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
			audit_unknown_reputation_alpha,
			audit_unknown_reputation_beta,
			audit_unknown_reputation_score,
			online_score,
			disqualified_at,
			suspended_at,
			offline_suspended_at,
			offline_under_review_at,
			updated_at,
			joined_at
		FROM reputation`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var statsList []reputation.Stats
	for rows.Next() {
		var stats reputation.Stats

		err := rows.Scan(&stats.SatelliteID,
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
			&stats.Audit.UnknownAlpha,
			&stats.Audit.UnknownBeta,
			&stats.Audit.UnknownScore,
			&stats.OnlineScore,
			&stats.DisqualifiedAt,
			&stats.SuspendedAt,
			&stats.OfflineSuspendedAt,
			&stats.OfflineUnderReviewAt,
			&stats.UpdatedAt,
			&stats.JoinedAt,
		)

		if err != nil {
			return nil, ErrReputation.Wrap(err)
		}

		statsList = append(statsList, stats)
	}

	return statsList, rows.Err()
}
