// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

// Inspector is a gRPC service for inspecting statdb internals
type Inspector struct {
	db DB
}

// NewInspector creates an Inspector
func NewInspector(sdb DB) *Inspector {
	return &Inspector{db: sdb}
}

// CountNodes returns the number of nodes in the db
func (srv *Inspector) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (*pb.CountNodesResponse, error) {
	return &pb.CountNodesResponse{}, errs.New("Not Implemented")
}

// DumpNodes returns all of the nodes from the db
func (srv *Inspector) DumpNodes(ctx context.Context, req *pb.DumpNodesRequest) (*pb.DumpNodesResponse, error) {
	return &pb.DumpNodesResponse{}, errs.New("Not Implemented")
}

// GetStats returns the stats for a particular node ID
func (srv *Inspector) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	dossier, err := srv.db.Get(ctx, req.NodeId)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsResponse{
		AuditCount:  dossier.GetReputation().GetAuditCount(),
		AuditRatio:  dossier.GetReputation().GetAuditSuccessRatio(),
		UptimeCount: dossier.GetReputation().GetUptimeCount(),
		UptimeRatio: dossier.GetReputation().GetUptimeRatio(),
	}, nil
}

// CreateStats creates a node with specified stats
func (srv *Inspector) CreateStats(ctx context.Context, req *pb.CreateStatsRequest) (*pb.CreateStatsResponse, error) {
	stats := &NodeStats{
		AuditCount:         req.AuditCount,
		AuditSuccessCount:  req.AuditSuccessCount,
		UptimeCount:        req.UptimeCount,
		UptimeSuccessCount: req.UptimeSuccessCount,
	}

	_, err := srv.db.Create(ctx, req.NodeId, stats)
	if err != nil {
		return nil, err
	}

	return &pb.CreateStatsResponse{}, nil
}
