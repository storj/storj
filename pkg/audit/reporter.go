// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/storj"
)

type reporter interface {
	RecordAudits(ctx context.Context, failedNodes []*statdb.UpdateRequest) (err error)
}

// Reporter records audit reports in statdb and implements the reporter interface
type Reporter struct {
	statdb     statdb.DB
	maxRetries int
}

// NewReporter instantiates a reporter
func NewReporter(ctx context.Context, statDBPort string, maxRetries int, apiKey string) (reporter *Reporter, err error) {
	sdb, ok := ctx.Value("masterdb").(interface {
		StatDB() statdb.DB
	})
	if !ok {
		return nil, errs.New("unable to get master db instance")
	}
	return &Reporter{statdb: sdb.StatDB(), maxRetries: maxRetries}, nil
}

// RecordAudits saves failed audit details to statdb
func (reporter *Reporter) RecordAudits(ctx context.Context, nodes []*statdb.UpdateRequest) (err error) {
	retries := 0
	for len(nodes) > 0 && retries < reporter.maxRetries {
		res, err := reporter.statdb.UpdateBatch(ctx, &statdb.UpdateBatchRequest{
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

func setAuditFailStatus(ctx context.Context, failedNodes storj.NodeIDList) (failStatusNodes []*statdb.UpdateRequest) {
	for i := range failedNodes {
		setNode := &statdb.UpdateRequest{
			Node:               failedNodes[i],
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
func setOfflineStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (offlineStatusNodes []*statdb.UpdateRequest) {
	for i := range offlineNodeIDs {
		setNode := &statdb.UpdateRequest{
			Node:         offlineNodeIDs[i],
			IsUp:         false,
			UpdateUptime: true,
		}
		offlineStatusNodes = append(offlineStatusNodes, setNode)
	}
	return offlineStatusNodes
}

func setSuccessStatus(ctx context.Context, offlineNodeIDs storj.NodeIDList) (successStatusNodes []*statdb.UpdateRequest) {
	for i := range offlineNodeIDs {
		setNode := &statdb.UpdateRequest{
			Node:               offlineNodeIDs[i],
			AuditSuccess:       true,
			IsUp:               true,
			UpdateAuditSuccess: true,
			UpdateUptime:       true,
		}
		successStatusNodes = append(successStatusNodes, setNode)
	}
	return successStatusNodes
}
