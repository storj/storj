// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Endpoint implements the contact service Endpoints
//
// architecture: Endpoint
type Endpoint struct {
	log       *zap.Logger
	pingStats *PingStats
}

// PingStats contains information regarding who and when the node was last pinged
type PingStats struct {
	mu               sync.Mutex
	lastPinged       time.Time
	whoPingedNodeID  storj.NodeID
	whoPingedAddress string
}

// NewEndpoint returns a new contact service endpoint
func NewEndpoint(log *zap.Logger, pingStats *PingStats) *Endpoint {
	return &Endpoint{
		log:       log,
		pingStats: pingStats,
	}
}

// PingNode provides an easy way to verify a node is online and accepting requests
func (endpoint *Endpoint) PingNode(ctx context.Context, req *pb.ContactPingRequest) (_ *pb.ContactPingResponse, err error) {
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
	endpoint.pingStats.WasPinged(time.Now(), peerID.ID, p.Addr.String())
	return &pb.ContactPingResponse{}, nil
}

// WhenLastPinged returns last time someone pinged this node.
func (stats *PingStats) WhenLastPinged() (when time.Time, who storj.NodeID, addr string) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	return stats.lastPinged, stats.whoPingedNodeID, stats.whoPingedAddress
}

// WasPinged notifies the service it has been remotely pinged.
func (stats *PingStats) WasPinged(when time.Time, srcNodeID storj.NodeID, srcAddress string) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.lastPinged = when
	stats.whoPingedNodeID = srcNodeID
	stats.whoPingedAddress = srcAddress
}
