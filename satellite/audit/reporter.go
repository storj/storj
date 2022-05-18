// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// reporter records audit reports in overlay and implements the Reporter interface.
//
// architecture: Service
type reporter struct {
	log              *zap.Logger
	reputations      *reputation.Service
	containment      Containment
	maxRetries       int
	maxReverifyCount int32
}

// Reporter records audit reports in the overlay and database.
type Reporter interface {
	RecordAudits(ctx context.Context, req Report) (_ Report, err error)
}

// Report contains audit result.
// It records whether an audit is able to be completed, the total number of
// pieces a given audit has conducted for, lists for nodes that
// succeeded, failed, were offline, have pending audits, or failed for unknown
// reasons and their current reputation status.
type Report struct {
	Successes       storj.NodeIDList
	Fails           storj.NodeIDList
	Offlines        storj.NodeIDList
	PendingAudits   []*PendingAudit
	Unknown         storj.NodeIDList
	NodesReputation map[storj.NodeID]overlay.ReputationStatus
}

// NewReporter instantiates a reporter.
func NewReporter(log *zap.Logger, reputations *reputation.Service, containment Containment, maxRetries int, maxReverifyCount int32) Reporter {
	return &reporter{
		log:              log,
		reputations:      reputations,
		containment:      containment,
		maxRetries:       maxRetries,
		maxReverifyCount: maxReverifyCount,
	}
}

// RecordAudits saves audit results to overlay. When no error, it returns
// nil for both return values, otherwise it returns the report with the fields
// set to the values which have been saved and the error.
func (reporter *reporter) RecordAudits(ctx context.Context, req Report) (_ Report, err error) {
	defer mon.Task()(&ctx)(&err)

	successes := req.Successes
	fails := req.Fails
	unknowns := req.Unknown
	offlines := req.Offlines
	pendingAudits := req.PendingAudits

	reporter.log.Debug("Reporting audits",
		zap.Int("successes", len(successes)),
		zap.Int("failures", len(fails)),
		zap.Int("unknowns", len(unknowns)),
		zap.Int("offlines", len(offlines)),
		zap.Int("pending", len(pendingAudits)),
	)

	var errlist errs.Group
	nodesReputation := req.NodesReputation

	tries := 0
	for tries <= reporter.maxRetries {
		if len(successes) == 0 && len(fails) == 0 && len(unknowns) == 0 && len(offlines) == 0 && len(pendingAudits) == 0 {
			return Report{}, nil
		}

		errlist = errs.Group{}

		successes, err = reporter.recordAuditStatus(ctx, successes, nodesReputation, reputation.AuditSuccess)
		errlist.Add(err)
		fails, err = reporter.recordAuditStatus(ctx, fails, nodesReputation, reputation.AuditFailure)
		errlist.Add(err)
		unknowns, err = reporter.recordAuditStatus(ctx, unknowns, nodesReputation, reputation.AuditUnknown)
		errlist.Add(err)
		offlines, err = reporter.recordAuditStatus(ctx, offlines, nodesReputation, reputation.AuditOffline)
		errlist.Add(err)
		pendingAudits, err = reporter.recordPendingAudits(ctx, pendingAudits, nodesReputation)
		errlist.Add(err)

		tries++
	}

	err = errlist.Err()
	if tries >= reporter.maxRetries && err != nil {
		return Report{
			Successes:     successes,
			Fails:         fails,
			Offlines:      offlines,
			Unknown:       unknowns,
			PendingAudits: pendingAudits,
		}, errs.Combine(Error.New("some nodes failed to be updated in overlay"), err)
	}
	return Report{}, nil
}

func (reporter *reporter) recordAuditStatus(ctx context.Context, nodeIDs storj.NodeIDList, nodesReputation map[storj.NodeID]overlay.ReputationStatus, auditOutcome reputation.AuditType) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIDs) == 0 {
		return nil, nil
	}
	var errors errs.Group
	for _, nodeID := range nodeIDs {
		err = reporter.reputations.ApplyAudit(ctx, nodeID, nodesReputation[nodeID], auditOutcome)
		if err != nil {
			failed = append(failed, nodeID)
			errors.Add(Error.New("failed to record audit status %s in overlay for node %s: %w", auditOutcome.String(), nodeID.String(), err))
		}
	}
	return failed, errors.Err()
}

// recordPendingAudits updates the containment status of nodes with pending audits.
func (reporter *reporter) recordPendingAudits(ctx context.Context, pendingAudits []*PendingAudit, nodesReputation map[storj.NodeID]overlay.ReputationStatus) (failed []*PendingAudit, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group

	for _, pendingAudit := range pendingAudits {
		if pendingAudit.ReverifyCount < reporter.maxReverifyCount {
			err := reporter.containment.IncrementPending(ctx, pendingAudit)
			if err != nil {
				failed = append(failed, pendingAudit)
				errlist.Add(err)
			}
			reporter.log.Info("Audit pending",
				zap.Stringer("Piece ID", pendingAudit.PieceID),
				zap.Stringer("Node ID", pendingAudit.NodeID))
		} else {
			// record failure -- max reverify count reached
			reporter.log.Info("max reverify count reached (audit failed)", zap.Stringer("Node ID", pendingAudit.NodeID))
			err = reporter.reputations.ApplyAudit(ctx, pendingAudit.NodeID, nodesReputation[pendingAudit.NodeID], reputation.AuditFailure)
			if err != nil {
				errlist.Add(err)
				failed = append(failed, pendingAudit)
			} else {
				_, err = reporter.containment.Delete(ctx, pendingAudit.NodeID)
				if err != nil && !ErrContainedNotFound.Has(err) {
					errlist.Add(err)
				}
			}
		}
	}

	if len(failed) > 0 {
		for _, v := range failed {
			reporter.log.Debug("failed to record Pending Nodes ",
				zap.Stringer("NodeID", v.NodeID),
				zap.String("Segment StreamID", v.StreamID.String()),
				zap.Uint64("Segment Position", v.Position.Encode()))
		}
		return failed, errs.Combine(Error.New("failed to record some pending audits"), errlist.Err())
	}
	return nil, nil
}
