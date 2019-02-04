// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
)

type reporter interface {
	RecordAudits(ctx context.Context, req *RecordAuditsInfo) (failed *RecordAuditsInfo, err error)
}

// Reporter records audit reports in statdb and implements the reporter interface
type Reporter struct {
	statdb     statdb.DB
	maxRetries int
}

// RecordAuditsInfo is a struct containing arguments/return values for RecordAudits()
type RecordAuditsInfo struct {
	SuccessNodeIDs storj.NodeIDList
	FailNodeIDs    storj.NodeIDList
	OfflineNodeIDs storj.NodeIDList
}

// NewReporter instantiates a reporter
func NewReporter(sdb statdb.DB, maxRetries int) *Reporter {
	return &Reporter{statdb: sdb, maxRetries: maxRetries}
}

// RecordAudits saves failed audit details to statdb
func (reporter *Reporter) RecordAudits(ctx context.Context, req *RecordAuditsInfo) (failed *RecordAuditsInfo, err error) {
	successNodeIDs := req.SuccessNodeIDs
	failNodeIDs := req.FailNodeIDs
	offlineNodeIDs := req.OfflineNodeIDs

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

		retries++
	}
	if retries >= reporter.maxRetries && len(errNodeIDs) > 0 {
		return &RecordAuditsInfo{
			SuccessNodeIDs: successNodeIDs,
			FailNodeIDs:    failNodeIDs,
			OfflineNodeIDs: offlineNodeIDs,
		}, Error.New("some nodes failed to be updated in statdb")
	}
	return nil, nil
}

// recordAuditFailStatus updates nodeIDs in statdb with isup=true, auditsuccess=false
func (reporter *Reporter) recordAuditFailStatus(ctx context.Context, failedAuditNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	failedIDs := storj.NodeIDList{}

	for _, nodeID := range failedAuditNodeIDs {
		_, err := reporter.statdb.Update(ctx, &statdb.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: false,
		})
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
		}
	}
	if len(failedIDs) > 0 {
		return failedIDs, Error.New("failed to record some audit fail statuses in statdb")
	}
	return nil, nil
}

// recordOfflineStatus updates nodeIDs in statdb with isup=false
// TODO: offline nodes should maybe be marked as failing the audit in the future
func (reporter *Reporter) recordOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	failedIDs := storj.NodeIDList{}

	for _, nodeID := range offlineNodeIDs {
		_, err := reporter.statdb.UpdateUptime(ctx, nodeID, false)
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
		}
	}
	if len(failedIDs) > 0 {
		return failedIDs, Error.New("failed to record some audit offline statuses in statdb")
	}
	return nil, nil
}

// recordAuditSuccessStatus updates nodeIDs in statdb with isup=true, auditsuccess=true
func (reporter *Reporter) recordAuditSuccessStatus(ctx context.Context, successNodeIDs storj.NodeIDList) (failed storj.NodeIDList, err error) {
	failedIDs := storj.NodeIDList{}

	for _, nodeID := range successNodeIDs {
		_, err := reporter.statdb.Update(ctx, &statdb.UpdateRequest{
			NodeID:       nodeID,
			IsUp:         true,
			AuditSuccess: true,
		})
		if err != nil {
			failedIDs = append(failedIDs, nodeID)
		}
	}
	if len(failedIDs) > 0 {
		return failedIDs, Error.New("failed to record some audit success statuses in statdb")
	}
	return nil, nil
}
