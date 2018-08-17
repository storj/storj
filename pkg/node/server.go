// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"

	"storj.io/storj/pkg/dht"

	proto "storj.io/storj/protos/overlay"
)

// Server implements the grpc Node Server
type Server struct {
	dht dht.DHT
}

//TODO: add limit to query request proto
// Query is a node to node communication query
func (s *Server) Query(ctx context.Context, req proto.QueryRequest) (proto.QueryResponse, error) {
	rt, err := s.dht.GetRoutingTable(ctx)
	if err != nil {
		return proto.QueryResponse{}, NodeClientErr.New("could not get routing table %v", err)
	}
	//find node?
	//find near to recevier?
	nodes, err := rt.FindNear(req.Receiver, rt.K())
	if err != nil {
		return proto.QueryResponse{}, NodeClientErr.New("could not find near %v", err)
	}
    return proto.QueryResponse{Sender: req.Sender, Response: nodes}, nil
}
