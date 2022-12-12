// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"strings"

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
	log         *zap.Logger
	reputations *reputation.Service
	overlay     *overlay.Service
	containment Containment
	// newContainment is temporary, and will replace containment
	newContainment   NewContainment
	maxRetries       int
	maxReverifyCount int32
}

// Reporter records audit reports in the overlay and database.
type Reporter interface {
	RecordAudits(ctx context.Context, req Report)
	ReportReverificationNeeded(ctx context.Context, piece *PieceLocator) (err error)
	RecordReverificationResult(ctx context.Context, pendingJob *ReverificationJob, outcome Outcome, reputation overlay.ReputationStatus) (err error)
}

// Report contains audit result.
// It records whether an audit is able to be completed, the total number of
// pieces a given audit has conducted for, lists for nodes that
// succeeded, failed, were offline, have pending audits, or failed for unknown
// reasons and their current reputation status.
type Report struct {
	Successes     storj.NodeIDList
	Fails         storj.NodeIDList
	Offlines      storj.NodeIDList
	PendingAudits []*PendingAudit
	// PieceAudits is temporary and will replace PendingAudits.
	PieceAudits     []*ReverificationJob
	Unknown         storj.NodeIDList
	NodesReputation map[storj.NodeID]overlay.ReputationStatus
}

// NewReporter instantiates a reporter.
func NewReporter(log *zap.Logger, reputations *reputation.Service, overlay *overlay.Service, containment Containment, newContainment NewContainment, maxRetries int, maxReverifyCount int32) Reporter {
	return &reporter{
		log:              log,
		reputations:      reputations,
		overlay:          overlay,
		containment:      containment,
		newContainment:   newContainment,
		maxRetries:       maxRetries,
		maxReverifyCount: maxReverifyCount,
	}
}

// RecordAudits saves audit results, applying reputation changes as appropriate.
// If some records can not be updated after a number of attempts, the failures
// are logged at level ERROR, but are otherwise thrown away.
func (reporter *reporter) RecordAudits(ctx context.Context, req Report) {
	defer mon.Task()(&ctx)(nil)

	successes := req.Successes
	fails := req.Fails
	unknowns := req.Unknown
	offlines := req.Offlines
	pendingAudits := req.PendingAudits
	pieceAudits := req.PieceAudits

	reporter.log.Debug("Reporting audits",
		zap.Int("successes", len(successes)),
		zap.Int("failures", len(fails)),
		zap.Int("unknowns", len(unknowns)),
		zap.Int("offlines", len(offlines)),
		zap.Int("pending", len(pendingAudits)),
		zap.Int("piece-pending", len(pieceAudits)),
	)

	nodesReputation := req.NodesReputation

	reportFailures := func(tries int, resultType string, err error, nodes storj.NodeIDList, pending []*PendingAudit, pieces []*ReverificationJob) {
		if err == nil || tries < reporter.maxRetries {
			// don't need to report anything until the last time through
			return
		}
		reporter.log.Error("failed to update reputation information with audit results",
			zap.String("result type", resultType),
			zap.Error(err),
			zap.String("node IDs", strings.Join(nodes.Strings(), ", ")),
			zap.Any("pending segment audits", pending),
			zap.Any("pending piece audits", pieces))
	}

	var err error
	for tries := 0; tries <= reporter.maxRetries; tries++ {
		if len(successes) == 0 && len(fails) == 0 && len(unknowns) == 0 && len(offlines) == 0 && len(pendingAudits) == 0 && len(pieceAudits) == 0 {
			return
		}

		successes, err = reporter.recordAuditStatus(ctx, successes, nodesReputation, reputation.AuditSuccess)
		reportFailures(tries, "successful", err, successes, nil, nil)
		fails, err = reporter.recordAuditStatus(ctx, fails, nodesReputation, reputation.AuditFailure)
		reportFailures(tries, "failed", err, fails, nil, nil)
		unknowns, err = reporter.recordAuditStatus(ctx, unknowns, nodesReputation, reputation.AuditUnknown)
		reportFailures(tries, "unknown", err, unknowns, nil, nil)
		offlines, err = reporter.recordAuditStatus(ctx, offlines, nodesReputation, reputation.AuditOffline)
		reportFailures(tries, "offline", err, offlines, nil, nil)
		pendingAudits, err = reporter.recordPendingAudits(ctx, pendingAudits, nodesReputation)
		reportFailures(tries, "pending", err, nil, pendingAudits, nil)
		pieceAudits, err = reporter.recordPendingPieceAudits(ctx, pieceAudits, nodesReputation)
		reportFailures(tries, "pending", err, nil, nil, pieceAudits)
	}
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

// recordPendingPieceAudits updates the containment status of nodes with pending piece audits.
// This function is temporary and will replace recordPendingAudits later in this commit chain.
func (reporter *reporter) recordPendingPieceAudits(ctx context.Context, pendingAudits []*ReverificationJob, nodesReputation map[storj.NodeID]overlay.ReputationStatus) (failed []*ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group

	for _, pendingAudit := range pendingAudits {
		logger := reporter.log.With(
			zap.Stringer("Node ID", pendingAudit.Locator.NodeID),
			zap.Stringer("Stream ID", pendingAudit.Locator.StreamID),
			zap.Uint64("Position", pendingAudit.Locator.Position.Encode()),
			zap.Int("Piece Num", pendingAudit.Locator.PieceNum))

		if pendingAudit.ReverifyCount < int(reporter.maxReverifyCount) {
			err := reporter.ReportReverificationNeeded(ctx, &pendingAudit.Locator)
			if err != nil {
				failed = append(failed, pendingAudit)
				errlist.Add(err)
				continue
			}
			logger.Info("reverification queued")
			continue
		}
		// record failure -- max reverify count reached
		logger.Info("max reverify count reached (audit failed)")
		err = reporter.reputations.ApplyAudit(ctx, pendingAudit.Locator.NodeID, nodesReputation[pendingAudit.Locator.NodeID], reputation.AuditFailure)
		if err != nil {
			logger.Info("failed to update reputation information", zap.Error(err))
			errlist.Add(err)
			failed = append(failed, pendingAudit)
			continue
		}
		_, stillContained, err := reporter.newContainment.Delete(ctx, &pendingAudit.Locator)
		if err != nil {
			if !ErrContainedNotFound.Has(err) {
				errlist.Add(err)
			}
			continue
		}
		if !stillContained {
			err = reporter.overlay.SetNodeContained(ctx, pendingAudit.Locator.NodeID, false)
			if err != nil {
				logger.Error("failed to mark node as not contained", zap.Error(err))
			}
		}
	}

	if len(failed) > 0 {
		return failed, errs.Combine(Error.New("failed to record some pending audits"), errlist.Err())
	}
	return nil, nil
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

func (reporter *reporter) ReportReverificationNeeded(ctx context.Context, piece *PieceLocator) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = reporter.newContainment.Insert(ctx, piece)
	if err != nil {
		return Error.New("failed to queue reverification audit for node: %w", err)
	}

	err = reporter.overlay.SetNodeContained(ctx, piece.NodeID, true)
	if err != nil {
		return Error.New("failed to update contained status: %w", err)
	}
	return nil
}

func (reporter *reporter) RecordReverificationResult(ctx context.Context, pendingJob *ReverificationJob, outcome Outcome, reputation overlay.ReputationStatus) (err error) {
	defer mon.Task()(&ctx)(&err)

	keepInQueue := true
	report := Report{
		NodesReputation: map[storj.NodeID]overlay.ReputationStatus{
			pendingJob.Locator.NodeID: reputation,
		},
	}
	switch outcome {
	case OutcomeNotPerformed:
	case OutcomeNotNecessary:
		keepInQueue = false
	case OutcomeSuccess:
		report.Successes = append(report.Successes, pendingJob.Locator.NodeID)
		keepInQueue = false
	case OutcomeFailure:
		report.Fails = append(report.Fails, pendingJob.Locator.NodeID)
		keepInQueue = false
	case OutcomeTimedOut:
		// This will get re-added to the reverification queue, but that is idempotent
		// and fine. We do need to add it to PendingAudits in order to get the
		// maxReverifyCount check.
		report.PieceAudits = append(report.PieceAudits, pendingJob)
	case OutcomeUnknownError:
		report.Unknown = append(report.Unknown, pendingJob.Locator.NodeID)
		keepInQueue = false
	case OutcomeNodeOffline:
		report.Offlines = append(report.Offlines, pendingJob.Locator.NodeID)
	}
	var errList errs.Group

	// apply any necessary reputation changes
	reporter.RecordAudits(ctx, report)

	// remove from reverifications queue if appropriate
	if !keepInQueue {
		_, stillContained, err := reporter.newContainment.Delete(ctx, &pendingJob.Locator)
		if err != nil {
			if !ErrContainedNotFound.Has(err) {
				errList.Add(err)
			}
		} else if !stillContained {
			err = reporter.overlay.SetNodeContained(ctx, pendingJob.Locator.NodeID, false)
			errList.Add(err)
		}
	}
	return errList.Err()
}
