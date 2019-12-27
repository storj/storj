// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
)

// Endpoint implements the contact service Endpoints
//
// architecture: Endpoint
type Endpoint struct {
	log       *zap.Logger
	pingStats *PingStats
}

// PingStats contains information regarding when the node was last pinged
type PingStats struct {
	mu         sync.Mutex
	lastPinged time.Time
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
	peer, err := rpcpeer.FromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	peerID, err := identity.PeerIdentityFromPeer(peer)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	endpoint.log.Debug("pinged", zap.Stringer("by", peerID.ID), zap.Stringer("srcAddr", peer.Addr))
	endpoint.pingStats.WasPinged(time.Now())
	return &pb.ContactPingResponse{}, nil
}

// WhenLastPinged returns last time someone pinged this node.
func (stats *PingStats) WhenLastPinged() (when time.Time) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	return stats.lastPinged
}

// WasPinged notifies the service it has been remotely pinged.
func (stats *PingStats) WasPinged(when time.Time) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.lastPinged = when
}
