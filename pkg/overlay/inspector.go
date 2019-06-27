// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

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
func (srv *Inspector) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (_ *pb.CountNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	overlayKeys, err := srv.cache.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Count: int64(len(overlayKeys)),
	}, nil
}

// DumpNodes returns all of the nodes in the overlay cachea
func (srv *Inspector) DumpNodes(ctx context.Context, req *pb.DumpNodesRequest) (_ *pb.DumpNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.DumpNodesResponse{}, errs.New("Not Implemented")
}
