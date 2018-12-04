// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
	statsproto "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/storj"
)

var (
	// ServerError is a gRPC server error for Inspector
	ServerError = errs.Class("inspector server error:")
)

// Server holds references to cache and kad
type Server struct {
	dht      dht.DHT
	cache    *overlay.Cache
	statdb   *statdb.StatDB
	logger   *zap.Logger
	metrics  *monkit.Registry
	identity *provider.FullIdentity
}

// ---------------------
// Kad/Overlay commands:
// ---------------------

// CountNodes returns the number of nodes in the cache and in kademlia
func (srv *Server) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (*pb.CountNodesResponse, error) {
	overlayKeys, err := srv.cache.DB.List(nil, 0)
	if err != nil {
		return nil, err
	}
	kadNodes, err := srv.dht.GetNodes(ctx, srv.identity.ID, 0)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Kademlia: int64(len(kadNodes)),
		Overlay:  int64(len(overlayKeys)),
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
	// TODO(bryanchriswhite): should use bucketID type
	nodeIDs, err := storj.NodeIDsFromBytes(b.ByteSlices())
	if err != nil {
		return nil, err
	}
	return &pb.GetBucketsResponse{
		Total: int64(len(b)),
		// TODO(bryanchriswhite): should use bucketID type
		Ids: nodeIDs,
	}, nil
}

// GetBucket retrieves all of a given K buckets contents
func (srv *Server) GetBucket(ctx context.Context, req *pb.GetBucketRequest) (*pb.GetBucketResponse, error) {
	rt, err := srv.dht.GetRoutingTable(ctx)
	if err != nil {
		return nil, err
	}
	// TODO(bryanchriswhite): should use bucketID type
	bucket, ok := rt.GetBucket(req.Id)
	if !ok {
		return &pb.GetBucketResponse{}, ServerError.New("GetBuckets returned non-OK response")
	}

	return &pb.GetBucketResponse{
		Id:    req.Id,
		Nodes: bucket.Nodes(),
	}, nil
}

// PingNode sends a PING RPC to the provided node ID in the Kad network.
func (srv *Server) PingNode(ctx context.Context, req *pb.PingNodeRequest) (*pb.PingNodeResponse, error) {
	rt, err := srv.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.PingNodeResponse{}, ServerError.Wrap(err)
	}

	self := rt.Local()

	nc, err := node.NewNodeClient(srv.identity, self, srv.dht)
	if err != nil {
		return &pb.PingNodeResponse{}, ServerError.Wrap(err)
	}

	p, err := nc.Ping(ctx, pb.Node{
		Id:   req.Id,
		Type: self.Type,
		Address: &pb.NodeAddress{
			Address: req.Address,
		},
	})
	res := &pb.PingNodeResponse{Ok: p}

	if err != nil {
		return res, ServerError.Wrap(err)
	}

	return res, nil
}

// LookupNode triggers a Kademlia lookup and returns the node the network found.
func (srv *Server) LookupNode(ctx context.Context, req *pb.LookupNodeRequest) (*pb.LookupNodeResponse, error) {
	id, err := storj.NodeIDFromString(req.Id)
	if err != nil {
		return &pb.LookupNodeResponse{}, err
	}
	node, err := srv.dht.FindNode(ctx, id)
	if err != nil {
		return &pb.LookupNodeResponse{}, err
	}

	return &pb.LookupNodeResponse{
		Node: &node,
	}, nil
}

// ---------------------
// StatDB commands:
// ---------------------

// GetStats returns the stats for a particular node ID
func (srv *Server) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	getReq := &statsproto.GetRequest{
		NodeId: req.NodeId,
	}
	res, err := srv.statdb.Get(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsResponse{
		AuditCount:  res.Stats.AuditCount,
		AuditRatio:  res.Stats.AuditSuccessRatio,
		UptimeCount: res.Stats.UptimeCount,
		UptimeRatio: res.Stats.UptimeRatio,
	}, nil
}

// CreateStats creates a node with specified stats
func (srv *Server) CreateStats(ctx context.Context, req *pb.CreateStatsRequest) (*pb.CreateStatsResponse, error) {
	node := &statsproto.Node{
		Id: req.NodeId,
	}
	stats := &statsproto.NodeStats{
		AuditCount:         req.AuditCount,
		AuditSuccessCount:  req.AuditSuccessCount,
		UptimeCount:        req.UptimeCount,
		UptimeSuccessCount: req.UptimeSuccessCount,
	}
	createReq := &statsproto.CreateRequest{
		Node:  node,
		Stats: stats,
	}
	_, err := srv.statdb.Create(ctx, createReq)
	if err != nil {
		return nil, err
	}

	return &pb.CreateStatsResponse{}, nil
}
