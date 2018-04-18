// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

// Overlay implements our overlay RPC service
type Overlay struct{}

// Lookup finds the address of a node in our overlay network
func (o *Overlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	// TODO: fill this in with logic to communicate with kademlia
	return &proto.LookupResponse{}, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Overlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	// TODO: fill this in with logic to communicate with kademlia
	return &proto.FindStorageNodesResponse{}, nil
}
