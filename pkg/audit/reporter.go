// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/statdb/sdbclient"
	"storj.io/storj/pkg/storj"
)

type reporter interface {
	RecordAudits(ctx context.Context, failedNodes []*proto.Node) (err error)
}

// Reporter records audit reports in statdb and implements the reporter interface
type Reporter struct {
	statdb     sdbclient.Client
	maxRetries int
}

// NewReporter instantiates a reporter
func NewReporter(ctx context.Context, statDBPort string, maxRetries int, apiKey string) (reporter *Reporter, err error) {
	ca, err := provider.NewTestCA(ctx)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}

	client, err := sdbclient.NewClient(identity, statDBPort, []byte(apiKey))
	if err != nil {
		return nil, err
	}
	return &Reporter{statdb: client, maxRetries: maxRetries}, nil
}

// RecordAudits saves failed audit details to statdb
func (reporter *Reporter) RecordAudits(ctx context.Context, nodes []*proto.Node) (err error) {
	retries := 0
	for len(nodes) > 0 && retries < reporter.maxRetries {
		_, failedNodes, err := reporter.statdb.UpdateBatch(ctx, nodes)
		if err != nil {
			return err
		}
		nodes = failedNodes
		retries++
	}
	if retries >= reporter.maxRetries && len(nodes) > 0 {
		return Error.New("some nodes who failed the audit also failed to be updated in statdb")
	}
	return nil
}

func setAuditFailStatus(ctx context.Context, failedNodes storj.NodeIDList) (failStatusNodes []*proto.Node) {
	for i := range failedNodes {
		setNode := &proto.Node{
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
func setOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (offlineStatusNodes []*proto.Node) {
	for i := range offlineNodeIDs {
		setNode := &proto.Node{
			Id:           offlineNodeIDs[i],
			IsUp:         false,
			UpdateUptime: true,
		}
		offlineStatusNodes = append(offlineStatusNodes, setNode)
	}
	return offlineStatusNodes
}

func setSuccessStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (successStatusNodes []*proto.Node) {
	for i := range offlineNodeIDs {
		setNode := &proto.Node{
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
