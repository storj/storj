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
	rt dht.RoutingTable
}

//TODO: add limit to query request proto
// Query is a node to node communication query
func (s *Server) Query(ctx context.Context, req proto.QueryRequest) (proto.QueryResponse, error) {
	// TODO: ping sender
	// Add sender to rt
	// look for receiver in routing table
	// return receiver or find nearest to receiver
	return proto.QueryResponse{}, nil
}
