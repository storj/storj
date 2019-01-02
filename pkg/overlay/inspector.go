// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"storj.io/storj/pkg/pb"
)

type Inspector struct {
	cache *Cache
}

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
