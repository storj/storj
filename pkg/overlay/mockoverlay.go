// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// MockOverlay is a mocked overlay implementation
type MockOverlay struct {
	nodes map[string]*pb.Node
}

// NewMockOverlay creates a new overlay mock
func NewMockOverlay(nodes []*pb.Node) *MockOverlay {
	rv := &MockOverlay{nodes: map[string]*pb.Node{}}
	for _, node := range nodes {
		rv.nodes[node.Id] = node
	}
	return rv
}

// FindStorageNodes finds storage nodes based on the request
func (mo *MockOverlay) FindStorageNodes(ctx context.Context,
	req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse,
	err error) {
	nodes := make([]*pb.Node, 0, len(mo.nodes))
	for _, node := range mo.nodes {
		nodes = append(nodes, node)
	}
	if int64(len(nodes)) < req.Opts.GetAmount() {
		return nil, errs.New("not enough farmers exist")
	}
	nodes = nodes[:req.Opts.GetAmount()]
	return &pb.FindStorageNodesResponse{Nodes: nodes}, nil
}

// Lookup finds a single storage node based on the request
func (mo *MockOverlay) Lookup(ctx context.Context, req *pb.LookupRequest) (
	*pb.LookupResponse, error) {
	return &pb.LookupResponse{Node: mo.nodes[req.NodeID]}, nil
}

//BulkLookup finds multiple storage nodes based on the requests
func (mo *MockOverlay) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (
	*pb.LookupResponses, error) {
	var responses []*pb.LookupResponse
	for _, r := range reqs.Lookuprequest {
		// NOTE (Dylan): tests did not catch missing node case, need updating
		n := mo.nodes[r.NodeID]
		resp := &pb.LookupResponse{Node: n}
		responses = append(responses, resp)
	}
	return &pb.LookupResponses{Lookupresponse: responses}, nil
}

// MockConfig specifies static nodes for mock overlay
type MockConfig struct {
	Nodes string `help:"a comma-separated list of <node-id>:<ip>:<port>" default:""`
}

// Run runs server with mock overlay
func (c MockConfig) Run(ctx context.Context, server *provider.Provider) error {
	var nodes []*pb.Node
	for _, nodestr := range strings.Split(c.Nodes, ",") {
		parts := strings.SplitN(nodestr, ":", 2)
		if len(parts) != 2 {
			return Error.New("malformed node config: %#v", nodestr)
		}
		id, addr := parts[0], parts[1]
		nodes = append(nodes, &pb.Node{
			Id: id,
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP,
				Address:   addr,
			}})
	}

	pb.RegisterOverlayServer(server.GRPC(), NewMockOverlay(nodes))
	return server.Run(ctx)
}
