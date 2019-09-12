// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// NodesServiceEndpoint implements the NodesServer Interface
type NodesServiceEndpoint struct {
	log *zap.Logger
}

// NewNodesServiceEndpoint returns a new nodes service endpoint
func NewNodesServiceEndpoint(log *zap.Logger) *NodesServiceEndpoint {
	return &NodesServiceEndpoint{
		log: log,
	}
}

// Query is a node to node communication query
func (endpoint *NodesServiceEndpoint) Query(ctx context.Context, req *pb.QueryRequest) (_ *pb.QueryResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.QueryResponse{}, nil
}

// Ping provides an easy way to verify a node is online and accepting requests
func (endpoint *NodesServiceEndpoint) Ping(ctx context.Context, req *pb.PingRequest) (_ *pb.PingResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "unable to get grpc peer from context")
	}
	peerID, err := identity.PeerIdentityFromPeer(p)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	endpoint.log.Debug("pinged", zap.Stringer("by", peerID.ID), zap.Stringer("srcAddr", p.Addr))
	return &pb.PingResponse{}, nil
}

// RequestInfo returns the node info
func (endpoint *NodesServiceEndpoint) RequestInfo(ctx context.Context, req *pb.InfoRequest) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	return &pb.InfoResponse{}, nil
}
