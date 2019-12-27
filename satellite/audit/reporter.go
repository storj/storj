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

// We do not report offline nodes to the overlay at this time; see V3-3025.
const reportOfflineDuringAudit = false

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

// Report contains audit result lists for nodes that succeeded, failed, were offline, have pending audits, or failed for unknown reasons
type Report struct {
	Successes     storj.NodeIDList
	Fails         storj.NodeIDList
	Offlines      storj.NodeIDList
	PendingAudits []*PendingAudit
	Unknown       storj.NodeIDList
}

// NewReporter instantiates a reporter
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
	offlines := req.Offlines
	pendingAudits := req.PendingAudits

	reporter.log.Debug("Reporting audits",
		zap.Int("successes", len(successes)),
		zap.Int("failures", len(fails)),
		zap.Int("offlines", len(offlines)),
		zap.Int("pending", len(pendingAudits)),
		zap.Binary("Segment", []byte(path)),
		zap.String("Segment Path", path),
	)

	var errlist errs.Group

	tries := 0
	for tries <= reporter.maxRetries {
		if len(successes) == 0 && len(fails) == 0 && len(offlines) == 0 && len(pendingAudits) == 0 {
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
		// We do not report offline nodes to the overlay at this time; see V3-3025.
		if len(offlines) > 0 && reportOfflineDuringAudit {
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
			PendingAudits: pendingAudits,
		}, errs.Combine(Error.New("some nodes failed to be updated in overlay"), err)
	}
	return Report{}, nil
}

// recordAuditFailStatus updates nodeIDs in overlay with isup=true, auditsuccess=false
func (reporter *Reporter) recordAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRequests := make([]*overlay.UpdateRequest, len(failedAuditNodeIDs))
	for i, nodeID := range failedAuditNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: false,
		}
	}
	if len(updateRequests) > 0 {
		failed, err = reporter.overlay.BatchUpdateStats(ctx, updateRequests)
		if err != nil || len(failed) > 0 {
			reporter.log.Debug("failed to record Failed Nodes ", zap.Strings("NodeIDs", failed.Strings()))
			return failed, errs.Combine(Error.New("failed to record some audit fail statuses in overlay"), err)
		}
	}
	return nil, nil
}

// recordOfflineStatus updates nodeIDs in overlay with isup=false. When there
// is any error the function return the list of nodes which haven't been
// recorded.
func (reporter *Reporter) recordOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group
	for _, nodeID := range offlineNodeIDs {
		_, err := reporter.overlay.UpdateUptime(ctx, nodeID, false)
		if err != nil {
			failed = append(failed, nodeID)
			errlist.Add(err)
		}
	}
	if len(failed) > 0 {
		reporter.log.Debug("failed to record Offline Nodes ", zap.Strings("NodeIDs", failed.Strings()))
		return failed, errs.Combine(Error.New("failed to record some audit offline statuses in overlay"), errlist.Err())
	}

	return nil, nil
}

// recordAuditSuccessStatus updates nodeIDs in overlay with isup=true, auditsuccess=true
func (reporter *Reporter) recordAuditSuccessStatus(ctx context.Context, successNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)

	updateRequests := make([]*overlay.UpdateRequest, len(successNodeIDs))
	for i, nodeID := range successNodeIDs {
		updateRequests[i] = &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: true,
		}
	}

	if len(updateRequests) > 0 {
		failed, err = reporter.overlay.BatchUpdateStats(ctx, updateRequests)
		if err != nil || len(failed) > 0 {
			reporter.log.Debug("failed to record Success Nodes ", zap.Strings("NodeIDs", failed.Strings()))
			return failed, errs.Combine(Error.New("failed to record some audit success statuses in overlay"), err)
		}
	}
	return nil, nil
}

// recordPendingAudits updates the containment status of nodes with pending audits
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
				AuditSuccess: false,
			})
		}
	}

	if len(updateRequests) > 0 {
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
	}
	return nil, nil
}
