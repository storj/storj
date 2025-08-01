// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/reputation"
)

// DBReporter records audit reports in overlay and implements the Reporter interface.
//
// architecture: Service
type DBReporter struct {
	log              *zap.Logger
	reputations      *reputation.Service
	overlay          *overlay.Service
	metabase         *metabase.DB
	containment      Containment
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
	Segment *metabase.SegmentForAudit

	Successes       storj.NodeIDList
	Fails           metabase.Pieces
	Offlines        storj.NodeIDList
	PendingAudits   []*ReverificationJob
	Unknown         storj.NodeIDList
	NodesReputation map[storj.NodeID]overlay.ReputationStatus
}

// NewReporter instantiates a DBReporter.
func NewReporter(log *zap.Logger, reputations *reputation.Service, overlay *overlay.Service, metabase *metabase.DB, containment Containment, cfg Config) *DBReporter {
	return &DBReporter{
		log:              log,
		reputations:      reputations,
		overlay:          overlay,
		metabase:         metabase,
		containment:      containment,
		maxRetries:       cfg.MaxRetriesStatDB,
		maxReverifyCount: int32(cfg.MaxReverifyCount),
	}
}

// RecordAudits saves audit results, applying reputation changes as appropriate.
// If some records can not be updated after a number of attempts, the failures
// are logged at level ERROR, but are otherwise thrown away.
func (reporter *DBReporter) RecordAudits(ctx context.Context, req Report) {
	defer mon.Task()(&ctx)(nil)

	successes := req.Successes
	fails := req.Fails
	unknowns := req.Unknown
	offlines := req.Offlines
	pendingAudits := req.PendingAudits

	logger := reporter.log
	if req.Segment != nil {
		logger = logger.With(zap.Stringer("stream ID", req.Segment.StreamID), zap.Uint64("position", req.Segment.Position.Encode()))
	}
	logger.Debug("Reporting audits",
		zap.Int("successes", len(successes)),
		zap.Int("failures", len(fails)),
		zap.Int("unknowns", len(unknowns)),
		zap.Int("offlines", len(offlines)),
		zap.Int("pending", len(pendingAudits)),
	)

	nodesReputation := req.NodesReputation

	reportFailures := func(tries int, resultType string, err error, nodes storj.NodeIDList, pending []*ReverificationJob) {
		if err == nil || tries < reporter.maxRetries {
			// don't need to report anything until the last time through
			return
		}
		reporter.log.Error("failed to update reputation information with audit results",
			zap.String("result_type", resultType),
			zap.Error(err),
			zap.Stringers("node_ids", nodes),
			zap.Int("nodes_num", len(nodes)),
			zap.Any("pending_segment_audits", pending))
	}

	var err error
	for tries := 0; tries <= reporter.maxRetries; tries++ {
		if len(successes) == 0 && len(fails) == 0 && len(unknowns) == 0 && len(offlines) == 0 && len(pendingAudits) == 0 {
			return
		}

		successes, err = reporter.recordAuditStatus(ctx, successes, nodesReputation, reputation.AuditSuccess)
		reportFailures(tries, "successful", err, successes, nil)
		fails, err = reporter.recordFailedAudits(ctx, req.Segment, fails, nodesReputation)
		reportFailures(tries, "failed", err, nil, nil)
		unknowns, err = reporter.recordAuditStatus(ctx, unknowns, nodesReputation, reputation.AuditUnknown)
		reportFailures(tries, "unknown", err, unknowns, nil)
		offlines, err = reporter.recordAuditStatus(ctx, offlines, nodesReputation, reputation.AuditOffline)
		reportFailures(tries, "offline", err, offlines, nil)
		pendingAudits, err = reporter.recordPendingAudits(ctx, pendingAudits, nodesReputation)
		reportFailures(tries, "pending", err, nil, pendingAudits)
	}
}

func (reporter *DBReporter) recordAuditStatus(ctx context.Context, nodeIDs storj.NodeIDList, nodesReputation map[storj.NodeID]overlay.ReputationStatus, auditOutcome reputation.AuditType) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(nodeIDs) == 0 {
		return nil, nil
	}
	var errors errs.Group
	for _, nodeID := range nodeIDs {
		err = reporter.reputations.ApplyAudit(ctx, nodeID, nodesReputation[nodeID], auditOutcome)
		if err != nil {
			failed = append(failed, nodeID)
			mon.Counter("audit_apply_to_db_failure").Inc(1)
			errors.Add(Error.New("failed to record audit status %s in overlay for node %s: %w", auditOutcome.String(), nodeID, err))
		}
	}
	return failed, errors.Err()
}

// recordPendingAudits updates the containment status of nodes with pending piece audits.
func (reporter *DBReporter) recordPendingAudits(ctx context.Context, pendingAudits []*ReverificationJob, nodesReputation map[storj.NodeID]overlay.ReputationStatus) (failed []*ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group

	for _, pendingAudit := range pendingAudits {
		logger := reporter.log.With(
			zap.Stringer("node_id", pendingAudit.Locator.NodeID),
			zap.Stringer("stream_id", pendingAudit.Locator.StreamID),
			zap.Uint64("position", pendingAudit.Locator.Position.Encode()),
			zap.Int("piece_num", pendingAudit.Locator.PieceNum))

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
		_, stillContained, err := reporter.containment.Delete(ctx, &pendingAudit.Locator)
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

const maxPiecesToRemoveAtOnce = 6

// recordFailedAudits performs reporting and response to hard-failed audits. Failed audits generally
// mean the piece is gone. Remove the pieces from the relevant pointers so that the segment can be
// repaired if appropriate, and so that we don't continually dock reputation for the same missing
// piece(s).
func (reporter *DBReporter) recordFailedAudits(
	ctx context.Context, segment *metabase.SegmentForAudit, failures []metabase.Piece, nodesReputation map[storj.NodeID]overlay.ReputationStatus,
) (failedToRecord []metabase.Piece, err error) {
	defer mon.Task()(&ctx)(&err)

	piecesToRemove := make(metabase.Pieces, 0, len(failures))
	var errors errs.Group
	for _, f := range failures {
		err = reporter.reputations.ApplyAudit(ctx, f.StorageNode, nodesReputation[f.StorageNode], reputation.AuditFailure)
		if err != nil {
			failedToRecord = append(failedToRecord, f)
			errors.Add(Error.New("failed to record audit failure in overlay for node %s: %w", f.StorageNode, err))
		}
		piecesToRemove = append(piecesToRemove, f)
	}
	if segment != nil {
		// Safety check. If, say, 30 pieces all started having audit failures at the same time, the
		// problem is more likely with the audit system itself and not with the pieces.
		if len(piecesToRemove) > maxPiecesToRemoveAtOnce {
			reporter.log.Error("cowardly refusing to remove large number of pieces for failed audit",
				zap.Int("piecesToRemove", len(piecesToRemove)),
				zap.Int("threshold", maxPiecesToRemoveAtOnce))
			return failedToRecord, errors.Err()
		}
		pieces, err := segment.Pieces.Remove(piecesToRemove)
		if err != nil {
			errors.Add(err)
			return failedToRecord, errors.Err()
		}
		errors.Add(reporter.metabase.UpdateSegmentPieces(ctx, metabase.UpdateSegmentPieces{
			StreamID:      segment.StreamID,
			Position:      segment.Position,
			OldPieces:     segment.Pieces,
			NewRedundancy: segment.Redundancy,
			NewPieces:     pieces,
		}))
	}
	return failedToRecord, errors.Err()
}

// ReportReverificationNeeded implements the Reporter interface.
func (reporter *DBReporter) ReportReverificationNeeded(ctx context.Context, piece *PieceLocator) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = reporter.containment.Insert(ctx, piece)
	if err != nil {
		return Error.New("failed to queue reverification audit for node: %w", err)
	}

	err = reporter.overlay.SetNodeContained(ctx, piece.NodeID, true)
	if err != nil {
		return Error.New("failed to update contained status: %w", err)
	}
	return nil
}

// RecordReverificationResult implements the Reporter interface.
func (reporter *DBReporter) RecordReverificationResult(ctx context.Context, pendingJob *ReverificationJob, outcome Outcome, reputation overlay.ReputationStatus) (err error) {
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
		// We have to look up the segment metainfo and pass it on to RecordAudits so that
		// the segment can be modified (removing this piece). We don't persist this
		// information through the reverification queue.
		segmentInfo, err := reporter.metabase.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
			StreamID: pendingJob.Locator.StreamID,
			Position: pendingJob.Locator.Position,
		})
		if err != nil {
			reporter.log.Error("could not look up segment after audit reverification",
				zap.Stringer("stream ID", pendingJob.Locator.StreamID),
				zap.Uint64("position", pendingJob.Locator.Position.Encode()),
				zap.Error(err),
			)
		} else {
			report.Segment = &segmentInfo
		}
		report.Fails = append(report.Fails, metabase.Piece{
			StorageNode: pendingJob.Locator.NodeID,
			Number:      uint16(pendingJob.Locator.PieceNum),
		})
		keepInQueue = false
	case OutcomeTimedOut:
		// This will get re-added to the reverification queue, but that is idempotent
		// and fine. We do need to add it to PendingAudits in order to get the
		// maxReverifyCount check.
		report.PendingAudits = append(report.PendingAudits, pendingJob)
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
		_, stillContained, err := reporter.containment.Delete(ctx, &pendingJob.Locator)
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

// NoReport disables reporting of audits.
type NoReport struct{}

// RecordAudits implements the Reporter interface.
func (n NoReport) RecordAudits(ctx context.Context, req Report) {

}

// ReportReverificationNeeded implements the Reporter interface.
func (n NoReport) ReportReverificationNeeded(ctx context.Context, piece *PieceLocator) (err error) {
	return nil
}

// RecordReverificationResult implements the Reporter interface.
func (n NoReport) RecordReverificationResult(ctx context.Context, pendingJob *ReverificationJob, outcome Outcome, reputation overlay.ReputationStatus) (err error) {
	return nil
}

var _ Reporter = &NoReport{}
