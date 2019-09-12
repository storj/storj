// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// SatelliteIDVerifier checks if the connection is from a trusted satellite
type SatelliteIDVerifier interface {
	VerifySatelliteID(ctx context.Context, id storj.NodeID) error
}

// NodesServiceEndpoint implements the NodesServer Interface
type NodesServiceEndpoint struct {
	log     *zap.Logger
	service *Service
	trust   SatelliteIDVerifier
}

// NewNodesServiceEndpoint returns a new nodes service endpoint
func NewNodesServiceEndpoint(log *zap.Logger, service *Service, trust SatelliteIDVerifier) *NodesServiceEndpoint {
	return &NodesServiceEndpoint{
		log:     log,
		service: service,
		trust:   trust,
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
	return &pb.PingResponse{}, nil
}

// RequestInfo returns the node info
func (endpoint *NodesServiceEndpoint) RequestInfo(ctx context.Context, req *pb.InfoRequest) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	self := endpoint.service.Local()

	if endpoint.trust == nil {
		return nil, status.Error(codes.Internal, "missing trust")
	}

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "untrusted peer %v", peer.ID)
	}

	return &pb.InfoResponse{
		Type:     self.Type,
		Operator: &self.Operator,
		Capacity: &self.Capacity,
		Version:  &self.Version,
	}, nil
}
