// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

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
	overlayKeys, err := srv.cache.DumpNodes(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Count: int64(len(overlayKeys)),
	}, nil
}

// DumpNodes is a GRPC method that returns all the nodes in the overlay cache
func (srv *Inspector) DumpNodes(ctx context.Context, req *pb.DumpNodesRequest) (*pb.DumpNodesResponse, error) {
	nodes, err := srv.cache.DumpNodes(ctx)
	if err != nil {
		return &pb.DumpNodesResponse{}, err
	}

	return &pb.DumpNodesResponse{
		Nodes: nodes,
	}, nil
}
