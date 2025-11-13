// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
)

var _ reputation.DB = (*reputations)(nil)

type reputations struct {
	db *satelliteDB
}

// Update updates a node's reputation stats as the result of a single audit
// event. See ApplyUpdates for further information.
//
// If the node (as represented in the returned info) becomes newly vetted,
// disqualified, or suspended as a result of this update, the caller is
// responsible for updating the records in the overlay to match.
func (reputations *reputations) Update(ctx context.Context, updateReq reputation.UpdateRequest, now time.Time) (_ *reputation.Info, err error) {
	mutations, err := reputation.UpdateRequestToMutations(updateReq, now)
	if err != nil {
		return nil, err
	}
	return reputations.ApplyUpdates(ctx, updateReq.NodeID, mutations, updateReq.Config, now)
}

// ApplyUpdates updates a node's reputation stats.
// The update is done in a loop to handle concurrent update calls and to avoid
// the need for an explicit transaction.
// There are three main steps go into the update process:
//  1. Get existing row for the node
//     (if no row found, insert a new row).
//  2. Evaluate what the new values for the row fields should be.
//  3. Update row using compare-and-swap.
//
// If the node (as represented in the returned info) becomes newly vetted,
// disqualified, or suspended as a result of these updates, the caller is
// responsible for updating the records in the overlay to match.
func (reputations *reputations) ApplyUpdates(ctx context.Context, nodeID storj.NodeID, updates reputation.Mutations, reputationConfig reputation.Config, now time.Time) (_ *reputation.Info, err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		// get existing reputation stats
		dbNode, err := reputations.db.Get_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}

		// if this is a new node, we will insert a new entry into the table
		if dbNode == nil {
			historyBytes, err := pb.Marshal(&pb.AuditHistory{})
			if err != nil {
				return nil, Error.Wrap(err)
			}

			// set default reputation stats for new node
			newNode := dbx.Reputation{
				Id:                          nodeID.Bytes(),
				UnknownAuditReputationAlpha: 1,
				AuditReputationAlpha:        reputationConfig.InitialAlpha,
				AuditReputationBeta:         reputationConfig.InitialBeta,
				OnlineScore:                 1,
				AuditHistory:                historyBytes,
			}

			var windows []*pb.AuditWindow
			if updates.OnlineHistory != nil {
				windows = updates.OnlineHistory.Windows
			}
			auditHistoryResponse, err := mergeAuditHistory(ctx, historyBytes, windows, reputationConfig.AuditHistory)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			update := reputations.populateUpdateNodeStats(&newNode, updates, reputationConfig, auditHistoryResponse, now)

			createFields := reputations.populateCreateFields(update)
			stats, err := reputations.db.Create_Reputation(ctx, dbx.Reputation_Id(nodeID.Bytes()), dbx.Reputation_AuditHistory(auditHistoryResponse.History), createFields)
			if err != nil {
				// if node has been added into the table during a concurrent
				// Update call happened between Get and Insert, we will try again so the audit is recorded
				// correctly
				if dbx.IsConstraintError(err) {
					mon.Event("reputations_update_query_retry_create")
					continue
				}

				return nil, Error.Wrap(err)
			}

			status, err := dbxToReputationInfo(stats)
			if err != nil {
				return nil, Error.Wrap(err)
			}
			return &status, nil
		}

		if updates.PositiveResults != 0 ||
			updates.UnknownResults != 0 ||
			updates.FailureResults != 0 ||
			updates.OfflineResults != 0 ||
			(updates.OnlineHistory != nil && len(updates.OnlineHistory.Windows) != 0) {
			// there is something to change

			auditHistoryResponse, err := mergeAuditHistory(ctx, dbNode.AuditHistory, updates.OnlineHistory.Windows, reputationConfig.AuditHistory)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			update := reputations.populateUpdateNodeStats(dbNode, updates, reputationConfig, auditHistoryResponse, now)

			updateFields := reputations.populateUpdateFields(update, auditHistoryResponse.History)
			oldAuditHistory := dbx.Reputation_AuditHistory(dbNode.AuditHistory)
			dbNode, err = reputations.db.Update_Reputation_By_Id_And_AuditHistory(ctx, dbx.Reputation_Id(nodeID.Bytes()), oldAuditHistory, updateFields)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, Error.Wrap(err)
			}

			// if update failed due to concurrent audit_history updates, we will try
			// again to get the latest data and update it
			if dbNode == nil {
				mon.Event("reputations_update_query_retry_update")
				continue
			}
		}

		status, err := dbxToReputationInfo(dbNode)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		return &status, nil
	}
}

func (reputations *reputations) Get(ctx context.Context, nodeID storj.NodeID) (*reputation.Info, error) {
	res, err := reputations.db.Get_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()))
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return nil, reputation.ErrNodeNotFound.New("no reputation entry for node")
		}
		return nil, Error.Wrap(err)
	}

	history := &pb.AuditHistory{}
	if err := pb.Unmarshal(res.AuditHistory, history); err != nil {
		return nil, Error.Wrap(err)
	}
	var dqReason overlay.DisqualificationReason
	if res.DisqualificationReason != nil {
		dqReason = overlay.DisqualificationReason(*res.DisqualificationReason)
	}

	return &reputation.Info{
		AuditSuccessCount:           res.AuditSuccessCount,
		TotalAuditCount:             res.TotalAuditCount,
		CreatedAt:                   &res.CreatedAt,
		VettedAt:                    res.VettedAt,
		UnknownAuditSuspended:       res.UnknownAuditSuspended,
		OfflineSuspended:            res.OfflineSuspended,
		UnderReview:                 res.UnderReview,
		Disqualified:                res.Disqualified,
		DisqualificationReason:      dqReason,
		OnlineScore:                 res.OnlineScore,
		AuditHistory:                history,
		AuditReputationAlpha:        res.AuditReputationAlpha,
		AuditReputationBeta:         res.AuditReputationBeta,
		UnknownAuditReputationAlpha: res.UnknownAuditReputationAlpha,
		UnknownAuditReputationBeta:  res.UnknownAuditReputationBeta,
	}, nil
}

// DisqualifyNode disqualifies a storage node.
func (reputations *reputations) DisqualifyNode(ctx context.Context, nodeID storj.NodeID, disqualifiedAt time.Time, disqualificationReason overlay.DisqualificationReason) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = reputations.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		if reputations.db.impl == dbutil.Cockroach || reputations.db.impl == dbutil.Postgres {
			_, err = tx.Tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
			if err != nil {
				return err
			}
		}

		_, err = tx.Get_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()))
		if errors.Is(err, sql.ErrNoRows) {
			historyBytes, err := pb.Marshal(&pb.AuditHistory{})
			if err != nil {
				return err
			}

			switch reputations.db.impl {
			case dbutil.Cockroach, dbutil.Postgres:
				_, err = tx.Tx.ExecContext(ctx, `
					INSERT INTO reputations (id, audit_history)
					VALUES ($1, $2)
					ON CONFLICT (id) DO NOTHING`,
					nodeID, historyBytes)
			case dbutil.Spanner:
				_, err = tx.Tx.ExecContext(ctx, `
					INSERT OR IGNORE INTO reputations (id, audit_history)
					VALUES (?, ?)`,
					nodeID, historyBytes)
			}
			if err != nil {
				return err
			}

		} else if err != nil {
			return err
		}

		updateFields := dbx.Reputation_Update_Fields{}
		updateFields.Disqualified = dbx.Reputation_Disqualified(disqualifiedAt.UTC())
		updateFields.DisqualificationReason = dbx.Reputation_DisqualificationReason(int(disqualificationReason))

		_, err = tx.Update_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()), updateFields)
		return err
	})
	return Error.Wrap(err)
}

// SuspendNodeUnknownAudit suspends a storage node for unknown audits.
func (reputations *reputations) SuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID, suspendedAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = reputations.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		if reputations.db.impl == dbutil.Cockroach || reputations.db.impl == dbutil.Postgres {
			_, err = tx.Tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
			if err != nil {
				return err
			}
		}

		_, err = tx.Get_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()))
		if errors.Is(err, sql.ErrNoRows) {
			historyBytes, err := pb.Marshal(&pb.AuditHistory{})
			if err != nil {
				return err
			}

			switch reputations.db.impl {
			case dbutil.Cockroach, dbutil.Postgres:
				_, err = tx.Tx.ExecContext(ctx, `
					INSERT INTO reputations (id, audit_history)
					VALUES ($1, $2)
					ON CONFLICT (id) DO NOTHING
				`, nodeID, historyBytes)
			case dbutil.Spanner:
				_, err = tx.Tx.ExecContext(ctx, `
					INSERT OR IGNORE INTO reputations (id, audit_history)
					VALUES (?, ?);
				`, nodeID, historyBytes)
			}
			if err != nil {
				return err
			}

		} else if err != nil {
			return err
		}

		updateFields := dbx.Reputation_Update_Fields{}
		updateFields.UnknownAuditSuspended = dbx.Reputation_UnknownAuditSuspended(suspendedAt.UTC())

		_, err = tx.Update_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()), updateFields)
		return err
	})
	return Error.Wrap(err)
}

// UnsuspendNodeUnknownAudit unsuspends a storage node for unknown audits.
func (reputations *reputations) UnsuspendNodeUnknownAudit(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = reputations.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		if reputations.db.impl == dbutil.Cockroach || reputations.db.impl == dbutil.Postgres {
			_, err = tx.Tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
			if err != nil {
				return err
			}
		}

		_, err = tx.Get_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()))
		if errors.Is(err, sql.ErrNoRows) {
			historyBytes, err := pb.Marshal(&pb.AuditHistory{})
			if err != nil {
				return err
			}

			switch reputations.db.impl {
			case dbutil.Cockroach, dbutil.Postgres:
				_, err = tx.Tx.ExecContext(ctx, `
					INSERT INTO reputations (id, audit_history)
					VALUES ($1, $2)
					ON CONFLICT (id) DO NOTHING
				`, nodeID, historyBytes)
			case dbutil.Spanner:
				_, err = tx.Tx.ExecContext(ctx, `
					INSERT OR IGNORE INTO reputations (id, audit_history)
					VALUES (?, ?);
				`, nodeID, historyBytes)
			}

			if err != nil {
				return err
			}

		} else if err != nil {
			return err
		}

		updateFields := dbx.Reputation_Update_Fields{}
		updateFields.UnknownAuditSuspended = dbx.Reputation_UnknownAuditSuspended_Null()

		_, err = tx.Update_Reputation_By_Id(ctx, dbx.Reputation_Id(nodeID.Bytes()), updateFields)
		return err

	})
	return Error.Wrap(err)
}

func (reputations *reputations) populateCreateFields(update updateNodeStats) dbx.Reputation_Create_Fields {
	createFields := dbx.Reputation_Create_Fields{}

	if update.VettedAt.set {
		createFields.VettedAt = dbx.Reputation_VettedAt(update.VettedAt.value)
	}
	if update.TotalAuditCount.set {
		createFields.TotalAuditCount = dbx.Reputation_TotalAuditCount(update.TotalAuditCount.value)
	}
	if update.AuditReputationAlpha.set {
		createFields.AuditReputationAlpha = dbx.Reputation_AuditReputationAlpha(update.AuditReputationAlpha.value)
	}
	if update.AuditReputationBeta.set {
		createFields.AuditReputationBeta = dbx.Reputation_AuditReputationBeta(update.AuditReputationBeta.value)
	}
	if update.Disqualified.set {
		createFields.Disqualified = dbx.Reputation_Disqualified(update.Disqualified.value)
	}
	if update.DisqualificationReason.set {
		createFields.DisqualificationReason = dbx.Reputation_DisqualificationReason(update.DisqualificationReason.value)
	}
	if update.UnknownAuditReputationAlpha.set {
		createFields.UnknownAuditReputationAlpha = dbx.Reputation_UnknownAuditReputationAlpha(update.UnknownAuditReputationAlpha.value)
	}
	if update.UnknownAuditReputationBeta.set {
		createFields.UnknownAuditReputationBeta = dbx.Reputation_UnknownAuditReputationBeta(update.UnknownAuditReputationBeta.value)
	}
	if update.UnknownAuditSuspended.set {
		if update.UnknownAuditSuspended.isNil {
			createFields.UnknownAuditSuspended = dbx.Reputation_UnknownAuditSuspended_Null()
		} else {
			createFields.UnknownAuditSuspended = dbx.Reputation_UnknownAuditSuspended(update.UnknownAuditSuspended.value)
		}
	}
	if update.AuditSuccessCount.set {
		createFields.AuditSuccessCount = dbx.Reputation_AuditSuccessCount(update.AuditSuccessCount.value)
	}

	if update.OnlineScore.set {
		createFields.OnlineScore = dbx.Reputation_OnlineScore(update.OnlineScore.value)
	}
	if update.OfflineSuspended.set {
		if update.OfflineSuspended.isNil {
			createFields.OfflineSuspended = dbx.Reputation_OfflineSuspended_Null()
		} else {
			createFields.OfflineSuspended = dbx.Reputation_OfflineSuspended(update.OfflineSuspended.value)
		}
	}
	if update.OfflineUnderReview.set {
		if update.OfflineUnderReview.isNil {
			createFields.UnderReview = dbx.Reputation_UnderReview_Null()
		} else {
			createFields.UnderReview = dbx.Reputation_UnderReview(update.OfflineUnderReview.value)
		}
	}

	return createFields
}

func (reputations *reputations) populateUpdateFields(update updateNodeStats, history []byte) dbx.Reputation_Update_Fields {
	updateFields := dbx.Reputation_Update_Fields{
		AuditHistory: dbx.Reputation_AuditHistory(history),
	}
	if update.VettedAt.set {
		updateFields.VettedAt = dbx.Reputation_VettedAt(update.VettedAt.value)
	}
	if update.TotalAuditCount.set {
		updateFields.TotalAuditCount = dbx.Reputation_TotalAuditCount(update.TotalAuditCount.value)
	}
	if update.AuditReputationAlpha.set {
		updateFields.AuditReputationAlpha = dbx.Reputation_AuditReputationAlpha(update.AuditReputationAlpha.value)
	}
	if update.AuditReputationBeta.set {
		updateFields.AuditReputationBeta = dbx.Reputation_AuditReputationBeta(update.AuditReputationBeta.value)
	}
	if update.Disqualified.set {
		updateFields.Disqualified = dbx.Reputation_Disqualified(update.Disqualified.value)
	}
	if update.DisqualificationReason.set {
		updateFields.DisqualificationReason = dbx.Reputation_DisqualificationReason(update.DisqualificationReason.value)
	}
	if update.UnknownAuditReputationAlpha.set {
		updateFields.UnknownAuditReputationAlpha = dbx.Reputation_UnknownAuditReputationAlpha(update.UnknownAuditReputationAlpha.value)
	}
	if update.UnknownAuditReputationBeta.set {
		updateFields.UnknownAuditReputationBeta = dbx.Reputation_UnknownAuditReputationBeta(update.UnknownAuditReputationBeta.value)
	}
	if update.UnknownAuditSuspended.set {
		if update.UnknownAuditSuspended.isNil {
			updateFields.UnknownAuditSuspended = dbx.Reputation_UnknownAuditSuspended_Null()
		} else {
			updateFields.UnknownAuditSuspended = dbx.Reputation_UnknownAuditSuspended(update.UnknownAuditSuspended.value)
		}
	}
	if update.AuditSuccessCount.set {
		updateFields.AuditSuccessCount = dbx.Reputation_AuditSuccessCount(update.AuditSuccessCount.value)
	}

	if update.OnlineScore.set {
		updateFields.OnlineScore = dbx.Reputation_OnlineScore(update.OnlineScore.value)
	}
	if update.OfflineSuspended.set {
		if update.OfflineSuspended.isNil {
			updateFields.OfflineSuspended = dbx.Reputation_OfflineSuspended_Null()
		} else {
			updateFields.OfflineSuspended = dbx.Reputation_OfflineSuspended(update.OfflineSuspended.value)
		}
	}
	if update.OfflineUnderReview.set {
		if update.OfflineUnderReview.isNil {
			updateFields.UnderReview = dbx.Reputation_UnderReview_Null()
		} else {
			updateFields.UnderReview = dbx.Reputation_UnderReview(update.OfflineUnderReview.value)
		}
	}

	return updateFields
}

func (reputations *reputations) populateUpdateNodeStats(dbNode *dbx.Reputation, updates reputation.Mutations, config reputation.Config, historyResponse *reputation.UpdateAuditHistoryResponse, now time.Time) updateNodeStats {
	// there are four audit outcomes: success, failure, offline, and unknown
	// if a node fails enough audits, it gets disqualified
	// if a node gets enough "unknown" audits, it gets put into suspension
	// if a node gets enough successful audits, and is in suspension, it gets removed from suspension
	auditAlpha := dbNode.AuditReputationAlpha
	auditBeta := dbNode.AuditReputationBeta
	unknownAuditAlpha := dbNode.UnknownAuditReputationAlpha
	unknownAuditBeta := dbNode.UnknownAuditReputationBeta
	totalAuditCount := dbNode.TotalAuditCount
	vettedAt := dbNode.VettedAt

	logger := reputations.db.log.With(zap.Stringer("Node ID", zapNodeIDBytes(dbNode.Id)))

	// Here we rely on the observation that, conceptually, if we have
	// collected some list of successes failures while auditing node N
	// during some short time period, it might reasonably have happened that
	// the events occurred in a different order.
	//
	// That is, if a node passed audit 1, then failed audit 2, then passed
	// audit 3, it is fair to treat it as if it passed two audits and then
	// failed one. This is because we expect that the order in which the
	// events occurred is not very relevant. If a node failed an audit for
	// piece P at time T, then it likely would also have failed an audit
	// for the same piece at time T±ε, so we can grade it as though that
	// had happened.
	//
	// There are conditions under which the order of events makes the
	// difference in whether a node is disqualified or not. To be as fair
	// as possible, we will not disqualify in those conditions. If a node
	// remains un-disqualified under any ordering of events, we should not
	// disqualify it. To that end, we will always apply failures _before_
	// applying successes. That ordering will always yield the highest
	// possible result alpha and the lowest possible result beta, assuming
	// weight > 0 and 0 < λ < 1 (the proof is left as an exercise for the
	// reader).

	// for audit failure, only update normal alpha/beta
	auditBeta, auditAlpha = reputation.UpdateReputationMultiple(
		updates.FailureResults,
		auditBeta,
		auditAlpha,
		config.AuditLambda,
		config.AuditWeight,
	)
	// for audit unknown, only update unknown alpha/beta
	unknownAuditBeta, unknownAuditAlpha = reputation.UpdateReputationMultiple(
		updates.UnknownResults,
		unknownAuditBeta,
		unknownAuditAlpha,
		config.UnknownAuditLambda,
		config.AuditWeight,
	)

	// for a successful audit, increase reputation for normal *and* unknown audits
	auditAlpha, auditBeta = reputation.UpdateReputationMultiple(
		updates.PositiveResults,
		auditAlpha,
		auditBeta,
		config.AuditLambda,
		config.AuditWeight,
	)
	unknownAuditAlpha, unknownAuditBeta = reputation.UpdateReputationMultiple(
		updates.PositiveResults,
		unknownAuditAlpha,
		unknownAuditBeta,
		config.UnknownAuditLambda,
		config.AuditWeight,
	)

	// offline results affect only the total count.
	updatedTotalAuditCount := totalAuditCount + int64(updates.OfflineResults+updates.UnknownResults+updates.FailureResults+updates.PositiveResults)

	mon.FloatVal("audit_reputation_alpha").Observe(auditAlpha)
	mon.FloatVal("audit_reputation_beta").Observe(auditBeta)
	mon.FloatVal("unknown_audit_reputation_alpha").Observe(unknownAuditAlpha)
	mon.FloatVal("unknown_audit_reputation_beta").Observe(unknownAuditBeta)
	mon.FloatVal("audit_online_score").Observe(historyResponse.NewScore)

	updateFields := updateNodeStats{
		NodeID:                      dbNode.Id,
		TotalAuditCount:             int64Field{set: true, value: updatedTotalAuditCount},
		AuditReputationAlpha:        float64Field{set: true, value: auditAlpha},
		AuditReputationBeta:         float64Field{set: true, value: auditBeta},
		UnknownAuditReputationAlpha: float64Field{set: true, value: unknownAuditAlpha},
		UnknownAuditReputationBeta:  float64Field{set: true, value: unknownAuditBeta},
		// Updating node stats always exits it from containment mode
		Contained: boolField{set: true, value: false},
		// always update online score
		OnlineScore: float64Field{set: true, value: historyResponse.NewScore},
	}

	timeSinceCreation := now.Sub(dbNode.CreatedAt)
	if vettedAt == nil && timeSinceCreation >= config.MinimumNodeAge && updatedTotalAuditCount >= config.AuditCount {
		updateFields.VettedAt = timeField{set: true, value: now}
	}

	// disqualification case a
	//   a) Success/fail audit reputation falls below audit DQ threshold
	auditRep := auditAlpha / (auditAlpha + auditBeta)
	if auditRep <= config.AuditDQ {
		logger.Info("Disqualified", zap.String("DQ type", "audit failure"))
		mon.Meter("bad_audit_dqs").Mark(1)
		updateFields.Disqualified = timeField{set: true, value: now}
		updateFields.DisqualificationReason = intField{set: true, value: int(overlay.DisqualificationReasonAuditFailure)}
	}

	// if unknown audit rep goes below threshold, suspend node. Otherwise unsuspend node.
	unknownAuditRep := unknownAuditAlpha / (unknownAuditAlpha + unknownAuditBeta)
	if unknownAuditRep <= config.UnknownAuditDQ {
		if dbNode.UnknownAuditSuspended == nil {
			logger.Info("Suspended", zap.String("Category", "Unknown Audits"))
			updateFields.UnknownAuditSuspended = timeField{set: true, value: now}
		}

		// disqualification case b
		//   b) Node is suspended (success/unknown reputation below audit DQ threshold)
		//        AND the suspended grace period has elapsed
		//        AND audit outcome is unknown or failed

		// if suspended grace period has elapsed and unknown audit rep is still
		// too low, disqualify node. Set suspended to nil if node is disqualified
		// NOTE: if updateFields.UnknownAuditSuspended is set, we just suspended
		// the node a few lines above, so it will not be disqualified.
		if dbNode.UnknownAuditSuspended != nil && !updateFields.UnknownAuditSuspended.set &&
			time.Since(*dbNode.UnknownAuditSuspended) > config.SuspensionGracePeriod &&
			config.SuspensionDQEnabled {
			logger.Info("Disqualified", zap.String("DQ type", "suspension grace period expired for unknown audits"))
			mon.Meter("unknown_suspension_dqs").Mark(1)
			updateFields.Disqualified = timeField{set: true, value: now}
			updateFields.DisqualificationReason = intField{set: true, value: int(overlay.DisqualificationReasonSuspension)}
			updateFields.UnknownAuditSuspended = timeField{set: true, isNil: true}
		}
	} else if dbNode.UnknownAuditSuspended != nil {
		logger.Info("Suspension lifted", zap.String("Category", "Unknown Audits"))
		updateFields.UnknownAuditSuspended = timeField{set: true, isNil: true}
	}

	updateFields.AuditSuccessCount = int64Field{set: true, value: dbNode.AuditSuccessCount + int64(updates.PositiveResults)}

	// if suspension not enabled, skip penalization and unsuspend node if applicable
	if !config.AuditHistory.OfflineSuspensionEnabled {
		if dbNode.OfflineSuspended != nil {
			updateFields.OfflineSuspended = timeField{set: true, isNil: true}
		}
		if dbNode.UnderReview != nil {
			updateFields.OfflineUnderReview = timeField{set: true, isNil: true}
		}
		return updateFields
	}

	// only penalize node if online score is below threshold and
	// if it has enough completed windows to fill a tracking period
	penalizeOfflineNode := false
	if historyResponse.NewScore < config.AuditHistory.OfflineThreshold && historyResponse.TrackingPeriodFull {
		penalizeOfflineNode = true
	}

	// Suspension and disqualification for offline nodes
	if dbNode.UnderReview != nil {
		// move node in and out of suspension as needed during review period
		if !penalizeOfflineNode && dbNode.OfflineSuspended != nil {
			updateFields.OfflineSuspended = timeField{set: true, isNil: true}
		} else if penalizeOfflineNode && dbNode.OfflineSuspended == nil {
			updateFields.OfflineSuspended = timeField{set: true, value: now}
		}

		gracePeriodEnd := dbNode.UnderReview.Add(config.AuditHistory.GracePeriod)
		trackingPeriodEnd := gracePeriodEnd.Add(config.AuditHistory.TrackingPeriod)
		trackingPeriodPassed := now.After(trackingPeriodEnd)

		// after tracking period has elapsed, if score is good, clear under review
		// otherwise, disqualify node (if OfflineDQEnabled feature flag is true)
		if trackingPeriodPassed {
			if penalizeOfflineNode {
				if config.AuditHistory.OfflineDQEnabled {
					logger.Info("Disqualified", zap.String("DQ type", "node offline"))
					mon.Meter("offline_dqs").Mark(1)
					updateFields.Disqualified = timeField{set: true, value: now}
					updateFields.DisqualificationReason = intField{set: true, value: int(overlay.DisqualificationReasonNodeOffline)}
				}
			} else {
				updateFields.OfflineUnderReview = timeField{set: true, isNil: true}
				updateFields.OfflineSuspended = timeField{set: true, isNil: true}
			}
		}
	} else if penalizeOfflineNode {
		// suspend node for being offline and begin review period
		updateFields.OfflineUnderReview = timeField{set: true, value: now}
		updateFields.OfflineSuspended = timeField{set: true, value: now}
	}

	return updateFields
}

type intField struct {
	set   bool
	value int
}

type int64Field struct {
	set   bool
	value int64
}

type float64Field struct {
	set   bool
	value float64
}

type boolField struct {
	set   bool
	value bool
}

type timeField struct {
	set   bool
	isNil bool
	value time.Time
}

type updateNodeStats struct {
	NodeID                      []byte
	VettedAt                    timeField
	TotalAuditCount             int64Field
	AuditReputationAlpha        float64Field
	AuditReputationBeta         float64Field
	Disqualified                timeField
	DisqualificationReason      intField
	UnknownAuditReputationAlpha float64Field
	UnknownAuditReputationBeta  float64Field
	UnknownAuditSuspended       timeField
	AuditSuccessCount           int64Field
	Contained                   boolField
	OfflineUnderReview          timeField
	OfflineSuspended            timeField
	OnlineScore                 float64Field
}

func dbxToReputationInfo(dbNode *dbx.Reputation) (reputation.Info, error) {
	info := reputation.Info{
		AuditSuccessCount:           dbNode.AuditSuccessCount,
		TotalAuditCount:             dbNode.TotalAuditCount,
		CreatedAt:                   &dbNode.CreatedAt,
		VettedAt:                    dbNode.VettedAt,
		UnknownAuditSuspended:       dbNode.UnknownAuditSuspended,
		OfflineSuspended:            dbNode.OfflineSuspended,
		UnderReview:                 dbNode.UnderReview,
		Disqualified:                dbNode.Disqualified,
		OnlineScore:                 dbNode.OnlineScore,
		AuditReputationAlpha:        dbNode.AuditReputationAlpha,
		AuditReputationBeta:         dbNode.AuditReputationBeta,
		UnknownAuditReputationAlpha: dbNode.UnknownAuditReputationAlpha,
		UnknownAuditReputationBeta:  dbNode.UnknownAuditReputationBeta,
	}
	if dbNode.DisqualificationReason != nil {
		info.DisqualificationReason = overlay.DisqualificationReason(*dbNode.DisqualificationReason)
	}
	if dbNode.AuditHistory != nil {
		info.AuditHistory = &pb.AuditHistory{}
		if err := pb.Unmarshal(dbNode.AuditHistory, info.AuditHistory); err != nil {
			return info, err
		}
	}
	return info, nil
}

type zapNodeIDBytes []byte

func (z zapNodeIDBytes) String() string {
	nodeID, err := storj.NodeIDFromBytes([]byte(z))
	if err != nil {
		return fmt.Sprintf("invalid node-id 0x%x", []byte(z))
	}
	return nodeID.String()
}
