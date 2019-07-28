// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Inspector is a gRPC service for inspecting kademlia internals
type Inspector struct {
	kademlia *Kademlia
	identity *identity.FullIdentity
}

// NewInspector creates an Inspector
func NewInspector(kademlia *Kademlia, identity *identity.FullIdentity) *Inspector {
	return &Inspector{
		kademlia: kademlia,
		identity: identity,
	}
}

// CountNodes returns the number of nodes in the routing table
func (srv *Inspector) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (_ *pb.CountNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: this is definitely the wrong way to get this
	kadNodes, err := srv.kademlia.FindNear(ctx, srv.identity.ID, 100000)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Count: int64(len(kadNodes)),
	}, nil
}

// GetBuckets returns all kademlia buckets for current kademlia instance
func (srv *Inspector) GetBuckets(ctx context.Context, req *pb.GetBucketsRequest) (_ *pb.GetBucketsResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	b, err := srv.kademlia.GetBucketIds(ctx)
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

// FindNear sends back limit of near nodes
func (srv *Inspector) FindNear(ctx context.Context, req *pb.FindNearRequest) (_ *pb.FindNearResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	start := req.Start
	limit := req.Limit
	nodes, err := srv.kademlia.FindNear(ctx, start, int(limit))
	if err != nil {
		return &pb.FindNearResponse{}, err
	}
	return &pb.FindNearResponse{
		Nodes: nodes,
	}, nil
}

// PingNode sends a PING RPC to the provided node ID in the Kad network.
func (srv *Inspector) PingNode(ctx context.Context, req *pb.PingNodeRequest) (_ *pb.PingNodeResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = srv.kademlia.Ping(ctx, pb.Node{
		Id: req.Id,
		Address: &pb.NodeAddress{
			Address: req.Address,
		},
	})

	res := &pb.PingNodeResponse{Ok: err == nil}

	if err != nil {
		return res, Error.Wrap(err)
	}
	return res, nil
}

// LookupNode triggers a Kademlia lookup and returns the node the network found.
func (srv *Inspector) LookupNode(ctx context.Context, req *pb.LookupNodeRequest) (_ *pb.LookupNodeResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	id, err := storj.NodeIDFromString(req.Id)
	if err != nil {
		return &pb.LookupNodeResponse{}, err
	}
	node, err := srv.kademlia.FindNode(ctx, id)
	if err != nil {
		return &pb.LookupNodeResponse{}, err
	}

	return &pb.LookupNodeResponse{
		Node: &node,
	}, nil
}

// DumpNodes returns all of the nodes in the routing table database.
func (srv *Inspector) DumpNodes(ctx context.Context, req *pb.DumpNodesRequest) (_ *pb.DumpNodesResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	nodes, err := srv.kademlia.DumpNodes(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.DumpNodesResponse{
		Nodes: nodes,
	}, nil
}

// NodeInfo sends a PING RPC to a node and returns its local info.
func (srv *Inspector) NodeInfo(ctx context.Context, req *pb.NodeInfoRequest) (_ *pb.NodeInfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	info, err := srv.kademlia.FetchInfo(ctx, pb.Node{
		Id:      req.Id,
		Address: req.Address,
	})
	if err != nil {
		return &pb.NodeInfoResponse{}, err
	}
	return &pb.NodeInfoResponse{
		Type:     info.GetType(),
		Operator: info.GetOperator(),
		Capacity: info.GetCapacity(),
		Version:  info.GetVersion(),
	}, nil
}

// GetBucketList returns the list of buckets with their routing nodes and their cached nodes
func (srv *Inspector) GetBucketList(ctx context.Context, req *pb.GetBucketListRequest) (_ *pb.GetBucketListResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	bucketIds, err := srv.kademlia.GetBucketIds(ctx)
	if err != nil {
		return nil, err
	}

	buckets := make([]*pb.GetBucketListResponse_Bucket, len(bucketIds))

	for i, b := range bucketIds {
		bucketID := keyToBucketID(b)
		routingNodes, err := srv.kademlia.GetNodesWithinKBucket(ctx, bucketID)
		if err != nil {
			return nil, err
		}
		cachedNodes := srv.kademlia.GetCachedNodesWithinKBucket(bucketID)
		buckets[i] = &pb.GetBucketListResponse_Bucket{
			BucketId:     keyToBucketID(b),
			RoutingNodes: routingNodes,
			CachedNodes:  cachedNodes,
		}

	}
	return &pb.GetBucketListResponse{
		Buckets: buckets,
	}, nil
}
