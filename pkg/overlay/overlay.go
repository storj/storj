// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"fmt"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
	"storj.io/storj/storage/redis"
)

// Overlay implements our overlay RPC service
type Overlay struct {
	kad *kademlia.Kademlia
	DB  *redis.OverlayClient
}

// Lookup finds the address of a node in our overlay network
func (o *Overlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	id := kademlia.NodeID(req.NodeID)

	address := make(chan proto.NodeAddress)
	node := make(chan proto.Node)
	err := make(chan error)

	go func(ch chan<- proto.NodeAddress, ech chan<- error) {
		addr, err := o.DB.Get(ctx, string(id))
		if err != nil {
			ech <- err
			return
		}

		ch <- *addr
		return
	}(address, err)

	go func(ch chan<- proto.Node, ech chan<- error) {
		na, err := o.kad.FindNode(ctx, id)
		if err != nil {
			ech <- err
			return
		}
		ch <- na
		return
	}(node, err)

	for {
		select {
		case addr := <-address:
			fmt.Println("address", addr)
			return &proto.LookupResponse{}, nil
		case na := <-node:
			fmt.Println("node", na)
			return &proto.LookupResponse{}, nil
		case e := <-err:
			fmt.Println("error", e)
			if e == redis.ErrNodeNotFound {
				continue
			}
			return nil, e
		}
	}

}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Overlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	// TODO: fill this in with logic to communicate with kademlia
	return &proto.FindStorageNodesResponse{}, nil
}
