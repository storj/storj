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

// Query is a node to node communication query
func (s *Server) Query(ctx context.Context, req proto.QueryRequest) (proto.QueryResponse, error) {
	// TODO(coyle): this will need to be added to the overlay service
	//look for node in routing table?
	//If not in there, add node to routing table?
	
	return proto.QueryResponse{}, nil
}
