// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/storj"
)

type reporter interface {
	RecordAudits(ctx context.Context, req *Report) (failed *Report, err error)
}

// Reporter records audit reports in overlay and implements the reporter interface
type Reporter struct {
	overlay          *overlay.Cache
	containment      Containment
	maxRetries       int
	maxReverifyCount int32
}

// Report contains audit result lists for nodes that succeeded, failed, were offline, or have pending audits
type Report struct {
	Successes     storj.NodeIDList
	Fails         storj.NodeIDList
	Offlines      storj.NodeIDList
	PendingAudits []*PendingAudit
}

// NewReporter instantiates a reporter
func NewReporter(overlay *overlay.Cache, containment Containment, maxRetries int, maxReverifyCount int32) *Reporter {
	return &Reporter{overlay: overlay, containment: containment, maxRetries: maxRetries, maxReverifyCount: maxReverifyCount}
}

// RecordAudits saves audit details to overlay
func (reporter *Reporter) RecordAudits(ctx context.Context, req *Report) (failed *Report, err error) {
	defer mon.Task()(&ctx)(&err)
	if req == nil {
		return nil, nil
	}

	successes := req.Successes
	fails := req.Fails
	offlines := req.Offlines
	pendingAudits := req.PendingAudits

	var errlist errs.Group

	retries := 0
	for retries < reporter.maxRetries {
		if len(successes) == 0 && len(fails) == 0 && len(offlines) == 0 {
			return nil, nil
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

		retries++
	}

	err = errlist.Err()
	if retries >= reporter.maxRetries && err != nil {
		return &Report{
			Successes:     successes,
			Fails:         fails,
			Offlines:      offlines,
			PendingAudits: pendingAudits,
		}, errs.Combine(Error.New("some nodes failed to be updated in overlay"), err)
	}
	return nil, nil
}

// recordAuditFailStatus updates nodeIDs in overlay with isup=true, auditsuccess=false
func (reporter *Reporter) recordAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group
	for _, nodeID := range failedAuditNodeIDs {
		_, err := reporter.overlay.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: false,
		})
		if err != nil {
			failed = append(failed, nodeID)
			errlist.Add(err)
		}

		// TODO(kaloyan): Perhaps, this should be executed in the same Tx as overlay.UpdateStats above
		_, err = reporter.containment.Delete(ctx, nodeID)
		if err != nil {
			failed = append(failed, nodeID)
			errlist.Add(err)
		}
	}
	if len(failed) > 0 {
		return failed, errs.Combine(Error.New("failed to record some audit fail statuses in overlay"), errlist.Err())
	}
	return nil, nil
}

// recordOfflineStatus updates nodeIDs in overlay with isup=false
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
		return failed, errs.Combine(Error.New("failed to record some audit offline statuses in overlay"), errlist.Err())
	}
	return nil, nil
}

// recordAuditSuccessStatus updates nodeIDs in overlay with isup=true, auditsuccess=true
func (reporter *Reporter) recordAuditSuccessStatus(ctx context.Context, successNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group
	for _, nodeID := range successNodeIDs {
		_, err := reporter.overlay.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: true,
		})
		if err != nil {
			failed = append(failed, nodeID)
			errlist.Add(err)
		}

		// TODO(kaloyan): Perhaps, this should be executed in the same Tx as overlay.UpdateStats above
		_, err = reporter.containment.Delete(ctx, nodeID)
		if err != nil {
			failed = append(failed, nodeID)
			errlist.Add(err)
		}
	}
	if len(failed) > 0 {
		return failed, errs.Combine(Error.New("failed to record some audit success statuses in overlay"), errlist.Err())
	}
	return nil, nil
}

// recordPendingAudits updates the containment status of nodes with pending audits
func (reporter *Reporter) recordPendingAudits(ctx context.Context, pendingAudits []*PendingAudit) (failed []*PendingAudit, err error) {
	defer mon.Task()(&ctx)(&err)
	var errlist errs.Group
	for _, pendingAudit := range pendingAudits {
		if pendingAudit.ReverifyCount < reporter.maxReverifyCount {
			err := reporter.containment.IncrementPending(ctx, pendingAudit)
			if err != nil {
				failed = append(failed, pendingAudit)
				errlist.Add(err)
			}
		} else {
			// record failure -- max reverify count reached
			_, err := reporter.overlay.UpdateStats(ctx, &overlay.UpdateRequest{
				NodeID:       pendingAudit.NodeID,
				IsUp:         true,
				AuditSuccess: false,
			})
			if err != nil {
				failed = append(failed, pendingAudit)
				errlist.Add(err)
			}

			// TODO(kaloyan): Perhaps, this should be executed in the same Tx as overlay.UpdateStats above
			_, err = reporter.containment.Delete(ctx, pendingAudit.NodeID)
			if err != nil {
				failed = append(failed, pendingAudit)
				errlist.Add(err)
			}
		}
	}
	if len(failed) > 0 {
		return failed, errs.Combine(Error.New("failed to record some pending audits"), errlist.Err())
	}
	return nil, nil
}
