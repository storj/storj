// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/statdb"
	statsproto "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/storj"
)

type reporter interface {
	RecordAudits(ctx context.Context, failedNodes []*pb.Node) (err error)
}

// Reporter records audit reports in statdb and implements the reporter interface
type Reporter struct {
	statdb     *statdb.StatDB
	maxRetries int
}

// NewReporter instantiates a reporter
func NewReporter(ctx context.Context, statDBPort string, maxRetries int, apiKey string) (reporter *Reporter, err error) {
	sdb := statdb.LoadFromContext(ctx)

	return &Reporter{statdb: sdb, maxRetries: maxRetries}, nil
}

// RecordAudits saves failed audit details to statdb
func (reporter *Reporter) RecordAudits(ctx context.Context, nodes []*pb.Node) (err error) {
	retries := 0
	for len(nodes) > 0 && retries < reporter.maxRetries {
		res, err := reporter.statdb.UpdateBatch(ctx, &statsproto.UpdateBatchRequest{
			NodeList: nodes,
		})
		if err != nil {
			return err
		}
		nodes = res.GetFailedNodes()
		retries++
	}
	if retries >= reporter.maxRetries && len(nodes) > 0 {
		return Error.New("some nodes who failed the audit also failed to be updated in statdb")
	}
	return nil
}

func setAuditFailStatus(ctx context.Context, failedNodes storj.NodeIDList) (failStatusNodes []*pb.Node) {
	for i := range failedNodes {
		setNode := &pb.Node{
			Id:                 failedNodes[i],
			AuditSuccess:       false,
			IsUp:               true,
			UpdateAuditSuccess: true,
			UpdateUptime:       true,
		}
		failStatusNodes = append(failStatusNodes, setNode)
	}
	return failStatusNodes
}

// TODO: offline nodes should maybe be marked as failing the audit in the future
func setOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (offlineStatusNodes []*pb.Node) {
	for i := range offlineNodeIDs {
		setNode := &pb.Node{
			Id:           offlineNodeIDs[i],
			IsUp:         false,
			UpdateUptime: true,
		}
		offlineStatusNodes = append(offlineStatusNodes, setNode)
	}
	return offlineStatusNodes
}

func setSuccessStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (successStatusNodes []*pb.Node) {
	for i := range offlineNodeIDs {
		setNode := &pb.Node{
			Id:                 offlineNodeIDs[i],
			AuditSuccess:       true,
			IsUp:               true,
			UpdateAuditSuccess: true,
			UpdateUptime:       true,
		}
		successStatusNodes = append(successStatusNodes, setNode)
	}
	return successStatusNodes
}
