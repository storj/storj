// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Inspector is a gRPC service for inspecting kademlia internals
type Inspector struct {
	dht      dht.DHT
	identity *identity.FullIdentity
}

// NewInspector creates an Inspector
func NewInspector(kad dht.DHT, identity *identity.FullIdentity) *Inspector {
	return &Inspector{
		dht:      kad,
		identity: identity,
	}
}

// CountNodes returns the number of nodes in the routing table
func (srv *Inspector) CountNodes(ctx context.Context, req *pb.CountNodesRequest) (*pb.CountNodesResponse, error) {
	// TODO: this is definitely the wrong way to get this
	kadNodes, err := srv.dht.FindNear(ctx, srv.identity.ID, 0)
	if err != nil {
		return nil, err
	}

	return &pb.CountNodesResponse{
		Count: int64(len(kadNodes)),
	}, nil
}

// GetBuckets returns all kademlia buckets for current kademlia instance
func (srv *Inspector) GetBuckets(ctx context.Context, req *pb.GetBucketsRequest) (*pb.GetBucketsResponse, error) {
	rt, err := srv.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.GetBucketsResponse{}, Error.Wrap(err)
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

// FindNear sends back limit of near nodes
func (srv *Inspector) FindNear(ctx context.Context, req *pb.FindNearRequest) (*pb.FindNearResponse, error) {
	start := req.Start
	limit := req.Limit
	nodes, err := srv.dht.FindNear(ctx, start, int(limit))
	if err != nil {
		return &pb.FindNearResponse{}, err
	}
	return &pb.FindNearResponse{
		Nodes: nodes,
	}, nil
}

// PingNode sends a PING RPC to the provided node ID in the Kad network.
func (srv *Inspector) PingNode(ctx context.Context, req *pb.PingNodeRequest) (*pb.PingNodeResponse, error) {
	rt, err := srv.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.PingNodeResponse{}, Error.Wrap(err)
	}

	self := rt.Local()

	nc, err := node.NewNodeClient(srv.identity, self, srv.dht)
	if err != nil {
		return &pb.PingNodeResponse{}, Error.Wrap(err)
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
		return res, Error.Wrap(err)
	}

	return res, nil
}

// LookupNode triggers a Kademlia lookup and returns the node the network found.
func (srv *Inspector) LookupNode(ctx context.Context, req *pb.LookupNodeRequest) (*pb.LookupNodeResponse, error) {
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
