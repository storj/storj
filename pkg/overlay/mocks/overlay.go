// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mocks

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Overlay is a mocked overlay implementation
type Overlay struct {
	nodes map[storj.NodeID]*pb.Node
}

// NewOverlay returns a newly initialized mock overlal
func NewOverlay(nodes []*pb.Node) *Overlay {
	rv := &Overlay{nodes: map[storj.NodeID]*pb.Node{}}
	for _, node := range nodes {
		rv.nodes[node.Id] = node
	}
	return rv

}

// FindStorageNodes is the mock implementation
func (mo *Overlay) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse, err error) {
	nodes := make([]*pb.Node, 0, len(mo.nodes))
	for _, node := range mo.nodes {
		nodes = append(nodes, node)
	}
	if int64(len(nodes)) < req.Opts.GetAmount() {
		return nil, errs.New("not enough storage nodes exist")
	}
	nodes = nodes[:req.Opts.GetAmount()]
	return &pb.FindStorageNodesResponse{Nodes: nodes}, nil
}

// Lookup finds a single storage node based on the request
func (mo *Overlay) Lookup(ctx context.Context, req *pb.LookupRequest) (
	*pb.LookupResponse, error) {
	return &pb.LookupResponse{Node: mo.nodes[req.NodeId]}, nil
}

//BulkLookup finds multiple storage nodes based on the requests
func (mo *Overlay) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (
	*pb.LookupResponses, error) {
	var responses []*pb.LookupResponse
	for _, r := range reqs.LookupRequest {
		// NOTE (Dylan): tests did not catch missing node case, need updating
		n := mo.nodes[r.NodeId]
		resp := &pb.LookupResponse{Node: n}
		responses = append(responses, resp)
	}
	return &pb.LookupResponses{LookupResponse: responses}, nil
}

// Config specifies static nodes for mock overlay
type Config struct {
	Nodes string `help:"a comma-separated list of <node-id>:<ip>:<port>" default:""`
}
