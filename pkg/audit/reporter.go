// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/storj"
)

type reporter interface {
	RecordAudits(ctx context.Context, req *RecordAuditsInfo) (failed *RecordAuditsInfo, err error)
}

// Reporter records audit reports in overlay and implements the reporter interface
type Reporter struct {
	overlay     *overlay.Cache
	containment *Containment
	maxRetries  int
}

// RecordAuditsInfo is a struct containing arguments/return values for RecordAudits()
type RecordAuditsInfo struct {
	SuccessNodeIDs storj.NodeIDList
	FailNodeIDs    storj.NodeIDList
	OfflineNodeIDs storj.NodeIDList
	PendingAudits  []*PendingAudit
}

// NewReporter instantiates a reporter
func NewReporter(overlay *overlay.Cache, containment *Containment, maxRetries int) *Reporter {
	return &Reporter{overlay: overlay, containment: containment, maxRetries: maxRetries}
}

// RecordAudits saves failed audit details to overlay
func (reporter *Reporter) RecordAudits(ctx context.Context, req *RecordAuditsInfo) (failed *RecordAuditsInfo, err error) {
	successNodeIDs := req.SuccessNodeIDs
	failNodeIDs := req.FailNodeIDs
	offlineNodeIDs := req.OfflineNodeIDs
	pendingAudits := req.PendingAudits

	var errNodeIDs storj.NodeIDList

	retries := 0
	for retries < reporter.maxRetries {
		if len(successNodeIDs) == 0 && len(failNodeIDs) == 0 && len(offlineNodeIDs) == 0 {
			return nil, nil
		}

		errNodeIDs = storj.NodeIDList{}

		if len(successNodeIDs) > 0 {
			successNodeIDs, err = reporter.recordAuditSuccessStatus(ctx, successNodeIDs)
			if err != nil {
				errNodeIDs = append(errNodeIDs, successNodeIDs...)
			}
		}
		if len(failNodeIDs) > 0 {
			failNodeIDs, err = reporter.recordAuditFailStatus(ctx, failNodeIDs)
			if err != nil {
				errNodeIDs = append(errNodeIDs, failNodeIDs...)
			}
		}
		if len(offlineNodeIDs) > 0 {
			offlineNodeIDs, err = reporter.recordOfflineStatus(ctx, offlineNodeIDs)
			if err != nil {
				errNodeIDs = append(errNodeIDs, offlineNodeIDs...)
			}
		}
		if len(pendingAudits) > 0 {
			pendingAudits, err = reporter.recordPendingAudits(ctx, pendingAudits)
			if err != nil {
				for _, pendingAudit := range pendingAudits {
					errNodeIDs = append(errNodeIDs, pendingAudit.NodeID)
				}
			}
		}

		retries++
	}
	if retries >= reporter.maxRetries && len(errNodeIDs) > 0 {
		return &RecordAuditsInfo{
			SuccessNodeIDs: successNodeIDs,
			FailNodeIDs:    failNodeIDs,
			OfflineNodeIDs: offlineNodeIDs,
			PendingAudits:  pendingAudits,
		}, Error.New("some nodes failed to be updated in overlay")
	}
	return nil, nil
}

// recordAuditFailStatus updates nodeIDs in overlay with isup=true, auditsuccess=false
func (reporter *Reporter) recordAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	failedIDs := storj.NodeIDList{}

	for _, nodeID := range failedAuditNodeIDs {
		_, err := reporter.overlay.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: false,
		})
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
		}
	}
	if len(failedIDs) > 0 {
		return failedIDs, Error.New("failed to record some audit fail statuses in overlay")
	}
	return nil, nil
}

// recordOfflineStatus updates nodeIDs in overlay with isup=false
// TODO: offline nodes should maybe be marked as failing the audit in the future
func (reporter *Reporter) recordOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	failedIDs := storj.NodeIDList{}

	for _, nodeID := range offlineNodeIDs {
		_, err := reporter.overlay.UpdateUptime(ctx, nodeID, false)
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
		}
	}
	if len(failedIDs) > 0 {
		return failedIDs, Error.New("failed to record some audit offline statuses in overlay")
	}
	return nil, nil
}

// recordAuditSuccessStatus updates nodeIDs in overlay with isup=true, auditsuccess=true
func (reporter *Reporter) recordAuditSuccessStatus(ctx context.Context, successNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	failedIDs := storj.NodeIDList{}

	for _, nodeID := range successNodeIDs {
		_, err := reporter.overlay.UpdateStats(ctx, &overlay.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: true,
		})
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
		}
	}
	if len(failedIDs) > 0 {
		return failedIDs, Error.New("failed to record some audit success statuses in overlay")
	}
	return nil, nil
}

// recordPendingAudits updates the containment status of nodes with pending audits
func (reporter *Reporter) recordPendingAudits(ctx context.Context, pendingAudits []*PendingAudit) (failed []*PendingAudit, err error) {
	for _, pendingAudit := range pendingAudits {
		err = reporter.containment.IncrementPending(ctx, pendingAudit)
		if err != nil {
			failed = append(failed, pendingAudit)
		}
	}
	if len(failed) > 0 {
		return failed, Error.New("failed to record some pending audits")
	}
	return nil, nil
}
