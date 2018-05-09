package kademlia

import (
	"context"

	"storj.io/storj/protos/overlay"
)

// Kademlia is an implementation of kademlia adhering to the DHT interface.
type Kademlia struct {
}

func (k Kademlia) GetNodes(ctx context.Context, start string, limit int) ([]overlay.Node, error) {
	return []overlay.Node{}, nil
}

func (k Kademlia) GetRoutingTable(ctx context.Context) (RoutingTable, error) {
	return RouteTable{}, nil
}

func (k Kademlia) Bootstrap(ctx context.Context) error {
	return nil
}

func (k Kademlia) Ping(ctx context.Context, node overlay.Node) (overlay.Node, error) {
	return overlay.Node{}, nil
}

func (k Kademlia) FindNode(ctx context.Context, ID NodeID) (overlay.Node, error) {
	return overlay.Node{}, nil
}
