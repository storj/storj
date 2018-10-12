// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	dbx "storj.io/storj/pkg/statdb/dbx"
	proto "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/statdb/sdbclient"
)

type reporter interface {
	RecordFailedAudits(ctx context.Context, failedNodes []*pb.Node) (err error)
}

// Reporter records audit reports in statdb and implements the reporter interface
type Reporter struct {
	statdb sdbclient.Client
}

var (
	port   = ":7777"
	apiKey = []byte("")
)

// NewReporter instantiates a reporter
func NewReporter() (reporter *Reporter, err error) {
	ctx := context.Background()
	ca, err := provider.NewCA(ctx, 12, 14)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	client, err := sdbclient.NewClient(identity, port, apiKey)
	if err != nil {
		return nil, err
	}
	return &Reporter{statdb: client}, nil
}

// RecordFailedAudits saves failed audit details to statdb
func (reporter *Reporter) RecordFailedAudits(ctx context.Context, failedNodes []*pb.Node) (err error) {
	nodes, err := reporter.recordEntryIfNotExists(ctx, failedNodes)
	if err != nil {
		return err
	}

	finished := false
	retries := 0
	for !finished && retries < 3 {
		_, failedNodes, err := reporter.statdb.UpdateBatch(ctx, nodes)
		if err != nil {
			return err
		}
		if len(failedNodes) == 0 {
			finished = true
		}
		nodes = failedNodes
		retries++
	}
	if retries == 3 {
		return Error.New("some nodes who failed the audit also failed to be updated in statdb")
	}
	return nil
}

// creates a statdb proto with node information and saves to statdb if it didn't already exist
func (reporter *Reporter) recordEntryIfNotExists(ctx context.Context, failedNodes []*pb.Node) (nodes []*proto.Node, err error) {
	nodes = make([]*proto.Node, len(failedNodes))
	for i, fail := range failedNodes {
		nodes[i] = &proto.Node{
			NodeId:             []byte(fail.GetId()),
			AuditSuccess:       false,
			IsUp:               true,
			UpdateLatency:      false,
			UpdateAuditSuccess: true,
			UpdateUptime:       true,
		}
		_, err = reporter.statdb.Get(ctx, nodes[i].NodeId)
		if err != nil {
			if serr, ok := err.(*dbx.Error); ok && serr.Code == dbx.ErrorCode_NoRows {
				err = reporter.statdb.Create(ctx, nodes[i].NodeId)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return nodes, nil
}
