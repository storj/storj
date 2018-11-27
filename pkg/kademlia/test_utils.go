// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// NewMockKademlia returns a newly intialized MockKademlia struct
func NewMockKademlia() *MockKademlia {
	return &MockKademlia{}
}

// MockKademlia is a mock implementation of the DHT interface used solely for testing
type MockKademlia struct {
	RoutingTable dht.RoutingTable
	Nodes        []*pb.Node
}

// GetNodes increments the GetNodesCalled field on MockKademlia
// returns the Nodes field on MockKademlia
func (k *MockKademlia) GetNodes(ctx context.Context, start string, limit int, restrictions ...pb.Restriction) ([]*pb.Node, error) {
	return k.Nodes, nil
}

// GetRoutingTable increments the GetRoutingTableCalled field on MockKademlia
//
// returns the RoutingTable field on MockKademlia
func (k *MockKademlia) GetRoutingTable(ctx context.Context) (dht.RoutingTable, error) {
	return k.RoutingTable, nil
}

// Bootstrap increments the BootstrapCalled field on MockKademlia
func (k *MockKademlia) Bootstrap(ctx context.Context) error {
	return nil
}

// Ping increments the PingCalled field on MockKademlia
func (k *MockKademlia) Ping(ctx context.Context, node pb.Node) (pb.Node, error) {
	return node, nil
}

// FindNode increments the FindNodeCalled field on MockKademlia
//
// returns the local kademlia node
func (k *MockKademlia) FindNode(ctx context.Context, ID dht.NodeID) (pb.Node, error) {
	return k.RoutingTable.Local(), nil
}

// Disconnect increments the DisconnectCalled field on MockKademlia
func (k *MockKademlia) Disconnect() error {
	return nil
}
