// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/protos/overlay"
)

type MockOverlay struct {
	nodes map[string]*proto.Node
}

func NewMockOverlay(nodes []*proto.Node) *MockOverlay {
	rv := &MockOverlay{nodes: map[string]*proto.Node{}}
	for _, node := range nodes {
		rv.nodes[node.Id] = node
	}
	return rv
}

func (mo *MockOverlay) FindStorageNodes(ctx context.Context,
	req *proto.FindStorageNodesRequest) (resp *proto.FindStorageNodesResponse,
	err error) {
	nodes := make([]*proto.Node, 0, len(mo.nodes))
	for _, node := range mo.nodes {
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
	return &proto.LookupResponse{Node: mo.nodes[req.NodeID]}, nil
}

type MockConfig struct {
	Nodes string `help:"a comma-separated list of <node-id>:<ip>:<port>" default:""`
}

func (c MockConfig) Run(ctx context.Context, server *provider.Provider) error {
	var nodes []*proto.Node
	for _, nodestr := range strings.Split(c.Nodes, ",") {
		parts := strings.SplitN(nodestr, ":", 2)
		if len(parts) != 2 {
			return Error.New("malformed node config: %#v", nodestr)
		}
		id, addr := parts[0], parts[1]
		nodes = append(nodes, &proto.Node{
			Id: id,
			Address: &proto.NodeAddress{
				Transport: proto.NodeTransport_TCP,
				Address:   addr,
			}})
	}

	proto.RegisterOverlayServer(server.GRPC(), NewMockOverlay(nodes))
	return server.Run(ctx)
}
