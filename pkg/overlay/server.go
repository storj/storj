// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

// Server implements our overlay RPC service
type Server struct {
	kad     *kademlia.Kademlia
	cache   *Cache
	logger  *zap.Logger
	metrics *monkit.Registry
}

// Lookup finds the address of a node in our overlay network
func (o *Server) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	na, err := o.cache.Get(ctx, req.NodeID)

	if err != nil {
		o.logger.Error("Error looking up node", zap.Error(err), zap.String("nodeID", req.NodeID))
		return nil, err
	}

	return &proto.LookupResponse{
		Node: &proto.Node{
			Id:      req.GetNodeID(),
			Address: na,
		},
	}, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Server) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	// NB:  call FilterNodeReputation from node_reputation package to find nodes for storage

	// TODO(coyle): need to determine if we will pull the startID and Limit from the request or just use hardcoded data
	// for now just using 40 for demos and empty string which will default the Id to the kademlia node doing the lookup
	nodes, err := o.kad.GetNodes(ctx, "", 40)
	if err != nil {
		o.logger.Error("Error getting nodes", zap.Error(err))
		return nil, err
	}

	return &proto.FindStorageNodesResponse{
		Nodes: nodes,
	}, nil
}
