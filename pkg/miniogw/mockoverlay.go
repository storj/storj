// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

import (
	"context"

	"storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
)

type MockOverlay struct {
	Nodes map[string]*proto.Node
}

func NewMockOverlay() *MockOverlay {
	return &MockOverlay{Nodes: map[string]*proto.Node{}}
}

var GlobalMockOverlay = NewMockOverlay()

func (mo *MockOverlay) Choose(ctx context.Context, amount int, space int64) (
	rv []*proto.Node, err error) {
	for _, node := range mo.Nodes {
		rv = append(rv, node)
	}
	return rv, nil
}

func (mo *MockOverlay) Lookup(ctx context.Context, nodeID dht.NodeID) (
	*proto.Node, error) {
	return mo.Nodes[nodeID.String()], nil
}
