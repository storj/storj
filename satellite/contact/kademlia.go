// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
)

// KademliaEndpoint implements the NodesServer Interface for backwards compatibility
type KademliaEndpoint struct {
	log *zap.Logger
}

// NewKademliaEndpoint returns a new endpoint
func NewKademliaEndpoint(log *zap.Logger) *KademliaEndpoint {
	return &KademliaEndpoint{
		log: log,
	}
}

// Query is a node to node communication query
func (endpoint *KademliaEndpoint) Query(ctx context.Context, req *pb.QueryRequest) (_ *pb.QueryResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.QueryResponse{}, nil
}

// Ping provides an easy way to verify a node is online and accepting requests
func (endpoint *KademliaEndpoint) Ping(ctx context.Context, req *pb.PingRequest) (_ *pb.PingResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.PingResponse{}, nil
}

// RequestInfo returns the node info
func (endpoint *KademliaEndpoint) RequestInfo(ctx context.Context, req *pb.InfoRequest) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.InfoResponse{}, nil
}
