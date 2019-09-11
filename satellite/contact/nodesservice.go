// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"storj.io/storj/pkg/pb"
)

// NodesServiceEndpoint implements the NodesServer Interface
type NodesServiceEndpoint struct{}

// NewNodesServiceEndpoint returns a new nodes service endpoint
func NewNodesServiceEndpoint() *NodesServiceEndpoint {
	return &NodesServiceEndpoint{}
}

// Query is a node to node communication query
func (endpoint *NodesServiceEndpoint) Query(ctx context.Context, req *pb.QueryRequest) (_ *pb.QueryResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.QueryResponse{}, nil
}

// Ping provides an easy way to verify a node is online and accepting requests
func (endpoint *NodesServiceEndpoint) Ping(ctx context.Context, req *pb.PingRequest) (_ *pb.PingResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.PingResponse{}, nil
}

// RequestInfo returns the node info
func (endpoint *NodesServiceEndpoint) RequestInfo(ctx context.Context, req *pb.InfoRequest) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.InfoResponse{}, nil
}
