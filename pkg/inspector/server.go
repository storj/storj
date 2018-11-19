// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
)

var (
	// ServerError is a gRPC server error for Inspector
	ServerError = errs.Class("inspector server error:")
)

// Server holds references to cache and kad
type Server struct {
	dht     dht.DHT
	cache   *overlay.Cache
	logger  *zap.Logger
	metrics *monkit.Registry
}

// CountNodes returns the number of nodes in the cache and in kademlia
func (srv *Server) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (*pb.CountNodesResponse, error) {
	return &pb.CountNodesResponse{
		Kademlia: 0,
		Overlay:  0,
	}, nil
}

// GetBuckets returns all kademlia buckets for current kademlia instance
func (srv *Server) GetBuckets(ctx context.Context, req *pb.GetBucketsRequest) (*pb.GetBucketsResponse, error) {
	rt, err := srv.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.GetBucketsResponse{}, ServerError.Wrap(err)
	}
	b, err := rt.GetBucketIds()
	if err != nil {
		return nil, err
	}
	bytes := b.ByteSlices()
	return &pb.GetBucketsResponse{
		Total: int64(len(b)),
		Ids:   bytes,
	}, nil
}

// GetBucket retrieves all of a given K buckets contents
func (srv *Server) GetBucket(ctx context.Context, req *pb.GetBucketRequest) (*pb.GetBucketResponse, error) {
	rt, err := srv.dht.GetRoutingTable(ctx)
	if err != nil {
		return nil, err
	}
	bucket, ok := rt.GetBucket(req.Id)
	if !ok {
		return &pb.GetBucketResponse{}, ServerError.New("GetBuckets returned non-OK response")
	}

	return &pb.GetBucketResponse{
		Id:    req.Id,
		Nodes: bucket.Nodes(),
	}, nil
}
