// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"time"

	"github.com/prometheus/common/log"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// Endpoint implements the contact service Endpoints
type Endpoint struct {
	log     *zap.Logger
	service *Service
}

// NewEndpoint returns a new contact service endpoint
func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// Ping provides an easy way to verify a node is online and accepting requests
func (endpoint *Endpoint) Ping(ctx context.Context, req *pb.PingRequest) (_ *pb.PingResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	if endpoint.service.Local().Type == pb.NodeType_STORAGE {
		p, ok := peer.FromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Internal, "unable to get grpc peer from context")
		}
		peerID, err := identity.PeerIdentityFromPeer(p)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		log.Debug("pinged", zap.Stringer("by", peerID.ID), zap.Stringer("srcAddr", p.Addr))
		endpoint.service.Pinged(time.Now(), peerID.ID, p.Addr.String())
	}
	return &pb.PingResponse{}, nil
}
