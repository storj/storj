// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"storj.io/storj/pkg/pb"
)

// Inspector is a gRPC service for inspecting statdb internals
type Inspector struct {
	statdb DB
}

// NewInspector creates an Inspector
func NewInspector(sdb DB) *Inspector {
	return &Inspector{statdb: sdb}
}

// GetStats returns the stats for a particular node ID
func (srv *Inspector) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	stats, err := srv.statdb.Get(ctx, req.NodeId)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsResponse{
		AuditCount:  stats.AuditCount,
		AuditRatio:  stats.AuditSuccessRatio,
		UptimeCount: stats.UptimeCount,
		UptimeRatio: stats.UptimeRatio,
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

	_, err := srv.statdb.Create(ctx, req.NodeId, stats)
	if err != nil {
		return nil, err
	}

	return &pb.CreateStatsResponse{}, nil
}
