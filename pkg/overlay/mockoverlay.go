// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
)

type MockOverlay struct {
	Nodes map[string]*proto.Node
}

func NewMockOverlay() *MockOverlay {
	return &MockOverlay{Nodes: map[string]*proto.Node{}}
}

var GlobalMockOverlay = NewMockOverlay()

func (mo *MockOverlay) FindStorageNodes(ctx context.Context,
	req *proto.FindStorageNodesRequest) (resp *proto.FindStorageNodesResponse,
	err error) {
	nodes := make([]*proto.Node, 0, len(mo.Nodes))
	for _, node := range mo.Nodes {
		nodes = append(nodes, node)
	}
	if int64(len(nodes)) < req.Opts.GetAmount() {
		return nil, errs.New("not enough farmers exist")
	}
	nodes = nodes[:req.Opts.GetAmount()]
	return &proto.FindStorageNodesResponse{Nodes: nodes}, nil
}

func (mo *MockOverlay) Lookup(ctx context.Context, req *proto.LookupRequest) (
	*proto.LookupResponse, error) {
	return &proto.LookupResponse{Node: mo.Nodes[req.NodeID]}, nil
}

type MockConfig struct{}

func (MockConfig) Run(ctx context.Context, server *provider.Provider) error {
	proto.RegisterOverlayServer(server.GRPC(), GlobalMockOverlay)
	return server.Run(ctx)
}
