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
	"storj.io/storj/satellite/overlay"
)

// SatelliteIDVerifier checks if the connection is from a trusted satellite
type SatelliteIDVerifier interface {
	VerifySatelliteID(ctx context.Context, id storj.NodeID) error
}

// Endpoint implements the contact service Endpoints
type Endpoint struct {
	log       *zap.Logger
	self      overlay.NodeDossier
	pingStats *PingStats
	trust     SatelliteIDVerifier
}

// PingStats contains information regarding who and when the node was last pinged
type PingStats struct {
	mu               sync.Mutex
	lastPinged       time.Time
	whoPingedNodeID  storj.NodeID
	whoPingedAddress string
}

// NewEndpoint returns a new contact service endpoint
func NewEndpoint(log *zap.Logger, self overlay.NodeDossier, pingStats *PingStats, trust SatelliteIDVerifier) *Endpoint {
	return &Endpoint{
		log:       log,
		pingStats: pingStats,
		self:      self,
		trust:     trust,
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

// RequestInf returns the node info
func (endpoint *Endpoint) RequestInf(ctx context.Context, req *pb.InfoReq) (_ *pb.InfoRes, err error) {
	defer mon.Task()(&ctx)(&err)
	self := endpoint.self

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

	//endpoint.pingStats.wasPinged(time.Now(), peerID.ID, p.Addr.String())

	return &pb.InfoRes{
		Type:     self.Type,
		Operator: &self.Operator,
		Capacity: &self.Capacity,
		Version:  &self.Version,
	}, nil

}

// WhenLastPinged returns last time someone pinged this node.
func (stats *PingStats) WhenLastPinged() (when time.Time, who storj.NodeID, addr string) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	return stats.lastPinged, stats.whoPingedNodeID, stats.whoPingedAddress
}

// wasPinged notifies the service it has been remotely pinged.
func (stats *PingStats) wasPinged(when time.Time, srcNodeID storj.NodeID, srcAddress string) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.lastPinged = when
	stats.whoPingedNodeID = srcNodeID
	stats.whoPingedAddress = srcAddress
}
