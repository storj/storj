// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

// Overlay implements our overlay RPC service
type Overlay struct {
	kad *kademlia.Kademlia
}

// Lookup finds the address of a node in our overlay network
func (o *Overlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	id := kademlia.NodeID(req.NodeID)
	na, err := o.kad.FindNode(ctx, id)

	if err != nil {
		return nil, err
	}

	return &proto.LookupResponse{
		NodeAddress: na.Address,
	}, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Overlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	// TODO: fill this in with logic to communicate with kademlia
	return &proto.FindStorageNodesResponse{}, nil
}
