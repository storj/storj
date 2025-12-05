// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/reputation"
)

// ErrReputation represents errors from the reputation database.
var ErrReputation = errs.Class("reputation")

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
			audit_success_count,
			audit_total_count,
			audit_reputation_alpha,
			audit_reputation_beta,
			audit_reputation_score,
			audit_unknown_reputation_alpha,
			audit_unknown_reputation_beta,
			audit_unknown_reputation_score,
			online_score,
			audit_history,
			disqualified_at,
			suspended_at,
			offline_suspended_at,
			offline_under_review_at,
			vetted_at,
			updated_at,
			joined_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

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

	var auditHistoryBytes []byte
	if stats.AuditHistory != nil {
		auditHistoryBytes, err = pb.Marshal(stats.AuditHistory)
		if err != nil {
			return ErrReputation.Wrap(err)
		}
	}

	_, err = db.ExecContext(ctx, query,
		stats.SatelliteID,
		stats.Audit.SuccessCount,
		stats.Audit.TotalCount,
		stats.Audit.Alpha,
		stats.Audit.Beta,
		stats.Audit.Score,
		stats.Audit.UnknownAlpha,
		stats.Audit.UnknownBeta,
		stats.Audit.UnknownScore,
		stats.OnlineScore,
		auditHistoryBytes,
		stats.DisqualifiedAt,
		stats.SuspendedAt,
		stats.OfflineSuspendedAt,
		stats.OfflineUnderReviewAt,
		stats.VettedAt,
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
		`SELECT audit_success_count,
			audit_total_count,
			audit_reputation_alpha,
			audit_reputation_beta,
			audit_reputation_score,
			audit_unknown_reputation_alpha,
			audit_unknown_reputation_beta,
			audit_unknown_reputation_score,
			online_score,
			audit_history,
			disqualified_at,
			suspended_at,
			offline_suspended_at,
			offline_under_review_at,
			vetted_at,
			updated_at,
			joined_at
		FROM reputation WHERE satellite_id = ?`,
		satelliteID,
	)

	var auditHistoryBytes []byte
	err = row.Scan(
		&stats.Audit.SuccessCount,
		&stats.Audit.TotalCount,
		&stats.Audit.Alpha,
		&stats.Audit.Beta,
		&stats.Audit.Score,
		&stats.Audit.UnknownAlpha,
		&stats.Audit.UnknownBeta,
		&stats.Audit.UnknownScore,
		&stats.OnlineScore,
		&auditHistoryBytes,
		&stats.DisqualifiedAt,
		&stats.SuspendedAt,
		&stats.OfflineSuspendedAt,
		&stats.OfflineUnderReviewAt,
		&stats.VettedAt,
		&stats.UpdatedAt,
		&stats.JoinedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		err = nil
		return &stats, nil
	}
	if err != nil {
		return &stats, ErrReputation.Wrap(err)
	}

	if auditHistoryBytes != nil {
		stats.AuditHistory = &pb.AuditHistory{}
		err = pb.Unmarshal(auditHistoryBytes, stats.AuditHistory)
	}
	return &stats, ErrReputation.Wrap(err)
}

// All retrieves all stats from DB.
func (db *reputationDB) All(ctx context.Context) (_ []reputation.Stats, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT satellite_id,
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
			vetted_at,
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
			&stats.VettedAt,
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

// Delete removes stats for specific satellite.
func (db *reputationDB) Delete(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, "DELETE FROM reputation WHERE satellite_id = ?", satelliteID)
	return ErrReputation.Wrap(err)
}
