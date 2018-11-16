// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package mocks

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// Overlay is a mocked overlay implementation
type Overlay struct {
	nodes map[storj.NodeID]storj.Node
}

// NewOverlay returns a newly initialized mock overlal
func NewOverlay(nodes []storj.Node) *Overlay {
	rv := &Overlay{nodes: map[storj.NodeID]storj.Node{}}
	for _, node := range nodes {
		rv.nodes[node.Id] = node
	}
	return rv

}

// FindStorageNodes is the mock implementation
func (mo *Overlay) FindStorageNodes(ctx context.Context, req *pb.FindStorageNodesRequest) (resp *pb.FindStorageNodesResponse, err error) {
	nodes := make([]storj.Node, 0, len(mo.nodes))
	for _, node := range mo.nodes {
		nodes = append(nodes, node)
	}
	if int64(len(nodes)) < req.Opts.GetAmount() {
		return nil, errs.New("not enough farmers exist")
	}
	nodes = nodes[:req.Opts.GetAmount()]
	return &pb.FindStorageNodesResponse{Nodes: storj.ProtoNodes(nodes)}, nil
}

// Lookup finds a single storage node based on the request
func (mo *Overlay) Lookup(ctx context.Context, req *pb.LookupRequest) (
	*pb.LookupResponse, error) {
	nodeID, err := storj.NodeIDFromBytes(req.NodeId)
	if err != nil {
		return nil, err
	}
	return &pb.LookupResponse{Node: mo.nodes[nodeID].Node}, nil
}

//BulkLookup finds multiple storage nodes based on the requests
func (mo *Overlay) BulkLookup(ctx context.Context, reqs *pb.LookupRequests) (
	*pb.LookupResponses, error) {
	var responses []*pb.LookupResponse
	for _, r := range reqs.Lookuprequest {
		// NOTE (Dylan): tests did not catch missing node case, need updating
		nodeID, err := storj.NodeIDFromBytes(r.NodeId)
		if err != nil {
			return nil, err
		}
		n := mo.nodes[nodeID]
		resp := &pb.LookupResponse{Node: n.Node}
		responses = append(responses, resp)
	}
	return &pb.LookupResponses{Lookupresponse: responses}, nil
}

// Config specifies static nodes for mock overlay
type Config struct {
	Nodes string `help:"a comma-separated list of <node-id>:<ip>:<port>" default:""`
}

// Run runs server with mock overlay
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	var nodes []storj.Node
	for _, nodestr := range strings.Split(c.Nodes, ",") {
		parts := strings.SplitN(nodestr, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("malformed node config: %#v", nodestr)
		}

		nidString, addr := parts[0], parts[1]
		nodeID, err := storj.NodeIDFromString(nidString)
		if err != nil {
			return err
		}

		nodes = append(nodes, storj.NewNodeWithID(
			nodeID,
			&pb.Node{
				Address: &pb.NodeAddress{
					Transport: pb.NodeTransport_TCP_TLS_GRPC,
					Address:   addr,
				},
			},
		))
	}

	pb.RegisterOverlayServer(server.GRPC(), NewOverlay(nodes))
	return server.Run(ctx)
}
