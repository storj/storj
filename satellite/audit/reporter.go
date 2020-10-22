// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/overlay"
)

// Reporter records audit reports in overlay and implements the reporter interface
//
// architecture: Service
type Reporter struct {
	log              *zap.Logger
	overlay          *overlay.Service
	containment      Containment
	maxRetries       int
	maxReverifyCount int32
}

// Report contains audit result lists for nodes that succeeded, failed, were offline, have pending audits, or failed for unknown reasons.
type Report struct {
	Successes     storj.NodeIDList
	Fails         storj.NodeIDList
	Offlines      storj.NodeIDList
	PendingAudits []*PendingAudit
	Unknown       storj.NodeIDList
}

// NewReporter instantiates a reporter.
func NewReporter(log *zap.Logger, overlay *overlay.Service, containment Containment, maxRetries int, maxReverifyCount int32) *Reporter {
	return &Reporter{
		log:              log,
		overlay:          overlay,
		containment:      containment,
		maxRetries:       maxRetries,
		maxReverifyCount: maxReverifyCount}
}

// RecordAudits saves audit results to overlay. When no error, it returns
// nil for both return values, otherwise it returns the report with the fields
// set to the values which have been saved and the error.
func (reporter *Reporter) RecordAudits(ctx context.Context, req Report, path storj.Path) (_ Report, err error) {
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

	tries := 0
	for tries <= reporter.maxRetries {
		if len(successes) == 0 && len(fails) == 0 && len(unknowns) == 0 && len(offlines) == 0 && len(pendingAudits) == 0 {
			return Report{}, nil
		}

		errlist = errs.Group{}

		if len(successes) > 0 {
			successes, err = reporter.recordAuditSuccessStatus(ctx, successes)
			if err != nil {
				errlist.Add(err)
			}
		}
		if len(fails) > 0 {
			fails, err = reporter.recordAuditFailStatus(ctx, fails)
			if err != nil {
				errlist.Add(err)
			}
		}
		if len(unknowns) > 0 {
			unknowns, err = reporter.recordAuditUnknownStatus(ctx, unknowns)
			if err != nil {
				errlist.Add(err)
			}
		}
		if len(offlines) > 0 {
			offlines, err = reporter.recordOfflineStatus(ctx, offlines)
			if err != nil {
				errlist.Add(err)
			}
		}
		if len(pendingAudits) > 0 {
			pendingAudits, err = reporter.recordPendingAudits(ctx, pendingAudits)
			if err != nil {
				errlist.Add(err)
			}
		}

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

// recordAuditFailStatus updates nodeIDs in overlay with isup=true, auditoutcome=fail.
func (reporter *Reporter) recordAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRequests := make([]*overlay.UpdateRequest, len(failedAuditNodeIDs))
	for i, nodeID := range failedAuditNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditOutcome: overlay.AuditFailure,
		}
	}
	failed, err = reporter.overlay.BatchUpdateStats(ctx, updateRequests)
	if err != nil || len(failed) > 0 {
		reporter.log.Debug("failed to record Failed Nodes ", zap.Strings("NodeIDs", failed.Strings()))
		return failed, errs.Combine(Error.New("failed to record some audit fail statuses in overlay"), err)
	}
	return nil, nil
}

// recordAuditUnknownStatus updates nodeIDs in overlay with isup=true, auditoutcome=unknown.
func (reporter *Reporter) recordAuditUnknownStatus(ctx context.Context, unknownAuditNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRequests := make([]*overlay.UpdateRequest, len(unknownAuditNodeIDs))
	for i, nodeID := range unknownAuditNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditOutcome: overlay.AuditUnknown,
		}
	}

	failed, err = reporter.overlay.BatchUpdateStats(ctx, updateRequests)
	if err != nil || len(failed) > 0 {
		reporter.log.Debug("failed to record Unknown Nodes ", zap.Strings("NodeIDs", failed.Strings()))
		return failed, errs.Combine(Error.New("failed to record some audit unknown statuses in overlay"), err)
	}
	return nil, nil
}

// recordOfflineStatus updates nodeIDs in overlay with isup=false, auditoutcome=offline.
func (reporter *Reporter) recordOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRequests := make([]*overlay.UpdateRequest, len(offlineNodeIDs))
	for i, nodeID := range offlineNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         false,
			AuditOutcome: overlay.AuditOffline,
		}
	}

	failed, err = reporter.overlay.BatchUpdateStats(ctx, updateRequests)
	if err != nil || len(failed) > 0 {
		reporter.log.Debug("failed to record Offline Nodes ", zap.Strings("NodeIDs", failed.Strings()))
		return failed, errs.Combine(Error.New("failed to record some audit offline statuses in overlay"), err)
	}

	return nil, nil
}

// recordAuditSuccessStatus updates nodeIDs in overlay with isup=true, auditoutcome=success.
func (reporter *Reporter) recordAuditSuccessStatus(ctx context.Context, successNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRequests := make([]*overlay.UpdateRequest, len(successNodeIDs))
	for i, nodeID := range successNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditOutcome: overlay.AuditSuccess,
		}
	}

	failed, err = reporter.overlay.BatchUpdateStats(ctx, updateRequests)
	if err != nil || len(failed) > 0 {
		reporter.log.Debug("failed to record Success Nodes ", zap.Strings("NodeIDs", failed.Strings()))
		return failed, errs.Combine(Error.New("failed to record some audit success statuses in overlay"), err)
	}
	return nil, nil
}

// recordPendingAudits updates the containment status of nodes with pending audits.
func (reporter *Reporter) recordPendingAudits(ctx context.Context, pendingAudits []*PendingAudit) (failed []*PendingAudit, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group

	var updateRequests []*overlay.UpdateRequest
	for _, pendingAudit := range pendingAudits {
		if pendingAudit.ReverifyCount < reporter.maxReverifyCount {
			err := reporter.containment.IncrementPending(ctx, pendingAudit)
			if err != nil {
				failed = append(failed, pendingAudit)
				errlist.Add(err)
			}
		} else {
			// record failure -- max reverify count reached
			updateRequests = append(updateRequests, &overlay.UpdateRequest{
				NodeID:       pendingAudit.NodeID,
				IsUp:         true,
				AuditOutcome: overlay.AuditFailure,
			})
		}
	}

	failedBatch, err := reporter.overlay.BatchUpdateStats(ctx, updateRequests)
	if err != nil {
		errlist.Add(err)
	}
	if len(failedBatch) > 0 {
		pendingMap := make(map[storj.NodeID]*PendingAudit)
		for _, pendingAudit := range pendingAudits {
			pendingMap[pendingAudit.NodeID] = pendingAudit
		}
		for _, nodeID := range failedBatch {
			pending, ok := pendingMap[nodeID]
			if ok {
				failed = append(failed, pending)
			}
		}
	}

	if len(failed) > 0 {
		for _, v := range failed {
			reporter.log.Debug("failed to record Pending Nodes ", zap.Stringer("NodeID", v.NodeID), zap.String("Path", v.Path))
		}
		return failed, errs.Combine(Error.New("failed to record some pending audits"), errlist.Err())
	}
	return nil, nil
}
