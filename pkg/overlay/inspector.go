// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"github.com/golang/protobuf/ptypes"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

// Inspector is a gRPC service for inspecting overlay cache internals
type Inspector struct {
	cache *Cache
}

// NewInspector creates an Inspector
func NewInspector(cache *Cache) *Inspector {
	return &Inspector{cache: cache}
}

// CountNodes returns the number of nodes in the cache
func (srv *Inspector) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (*pb.CountNodesResponse, error) {
	overlayKeys, err := srv.cache.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Count: int64(len(overlayKeys)),
	}, nil
}

// DumpNodes returns all of the nodes in the overlay cachea
func (srv *Inspector) DumpNodes(ctx context.Context, req *pb.DumpNodesRequest) (*pb.DumpNodesResponse, error) {
	return &pb.DumpNodesResponse{}, errs.New("Not Implemented")
}

// GetStats returns the stats for a particular node ID
func (srv *Inspector) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	node, err := srv.cache.Get(ctx, req.NodeId)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsResponse{
		AuditCount:  node.GetReputation().GetAuditCount(),
		AuditRatio:  node.GetReputation().GetAuditSuccessRatio(),
		UptimeCount: node.GetReputation().GetUptimeCount(),
		UptimeRatio: node.GetReputation().GetUptimeRatio(),
	}, nil
}

// CreateStats creates a node with specified stats
func (srv *Inspector) CreateStats(ctx context.Context, req *pb.CreateStatsRequest) (*pb.CreateStatsResponse, error) {
	ver := pb.NodeVersion{
		Version:   "v0.1.0",
		Timestamp: ptypes.TimestampNow(),
	}

	stats := &NodeStats{
		AuditCount:         req.AuditCount,
		AuditSuccessCount:  req.AuditSuccessCount,
		UptimeCount:        req.UptimeCount,
		UptimeSuccessCount: req.UptimeSuccessCount,
		Version:            ver,
	}

	_, err := srv.cache.Create(ctx, req.NodeId, stats)
	if err != nil {
		return nil, err
	}

	return &pb.CreateStatsResponse{}, nil
}
