// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcpeer"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/pkg/storj"
)

// EndpointError defines errors class for Endpoint
var EndpointError = errs.Class("kademlia endpoint error")

// SatelliteIDVerifier checks if the connection is from a trusted satellite
type SatelliteIDVerifier interface {
	VerifySatelliteID(ctx context.Context, id storj.NodeID) error
}

type pingStatsSource interface {
	WasPinged(when time.Time, byID storj.NodeID, byAddr string)
}

// Endpoint implements the kademlia Endpoints
type Endpoint struct {
	log          *zap.Logger
	service      *Kademlia
	pingStats    pingStatsSource
	routingTable *RoutingTable
	trust        SatelliteIDVerifier
	connected    int32
}

// NewEndpoint returns a new kademlia endpoint
func NewEndpoint(log *zap.Logger, service *Kademlia, pingStats pingStatsSource, routingTable *RoutingTable, trust SatelliteIDVerifier) *Endpoint {
	return &Endpoint{
		log:          log,
		service:      service,
		pingStats:    pingStats,
		routingTable: routingTable,
		trust:        trust,
	}
}

// Query is a node to node communication query
func (endpoint *Endpoint) Query(ctx context.Context, req *pb.QueryRequest) (_ *pb.QueryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if req.GetPingback() {
		endpoint.pingback(ctx, req.Sender)
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > endpoint.routingTable.bucketSize {
		limit = endpoint.routingTable.bucketSize
	}

	nodes, err := endpoint.routingTable.FindNear(ctx, req.Target.Id, limit)
	if err != nil {
		return &pb.QueryResponse{}, EndpointError.New("could not find near endpoint: %v", err)
	}

	return &pb.QueryResponse{Sender: req.Sender, Response: nodes}, nil
}

// pingback implements pingback for queries
func (endpoint *Endpoint) pingback(ctx context.Context, target *pb.Node) {
	var err error
	defer mon.Task()(&ctx)(&err)
	_, err = endpoint.service.Ping(ctx, *target)
	if err != nil {
		endpoint.log.Debug("connection to node failed", zap.Error(err), zap.Stringer("nodeID", target.Id))
		err = endpoint.routingTable.ConnectionFailed(ctx, target)
		if err != nil {
			endpoint.log.Error("could not respond to connection failed", zap.Error(err))
		}
	} else {
		err = endpoint.routingTable.ConnectionSuccess(ctx, target)
		if err != nil {
			endpoint.log.Error("could not respond to connection success", zap.Error(err))
		} else {
			count := atomic.AddInt32(&endpoint.connected, 1)
			if count == 1 {
				endpoint.log.Sugar().Debugf("Successfully connected with %s", target.Address.Address)
			} else if count%100 == 0 {
				endpoint.log.Sugar().Debugf("Successfully connected with %s %dx times", target.Address.Address, count)
			}
		}
	}
}

// Ping provides an easy way to verify a node is online and accepting requests
func (endpoint *Endpoint) Ping(ctx context.Context, req *pb.PingRequest) (_ *pb.PingResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	// NOTE: this code is very similar to that in storagenode/contact.(*Endpoint).PingNode().
	// That other will be used going forward, and this will soon be gutted and deprecated. The
	// code similarity will only exist until the transition away from Kademlia is complete.
	peer, err := rpcpeer.FromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}
	peerID, err := identity.PeerIdentityFromPeer(peer)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}
	if endpoint.pingStats != nil {
		endpoint.pingStats.WasPinged(time.Now(), peerID.ID, peer.Addr.String())
	}
	return &pb.PingResponse{}, nil
}

// RequestInfo returns the node info
func (endpoint *Endpoint) RequestInfo(ctx context.Context, req *pb.InfoRequest) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	self := endpoint.service.Local()

	if self.Type == pb.NodeType_STORAGE {
		if endpoint.trust == nil {
			return nil, rpcstatus.Error(rpcstatus.Internal, "missing trust")
		}

		peer, err := identity.PeerIdentityFromContext(ctx)
		if err != nil {
			return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
		}

		err = endpoint.trust.VerifySatelliteID(ctx, peer.ID)
		if err != nil {
			return nil, rpcstatus.Errorf(rpcstatus.PermissionDenied, "untrusted peer %v", peer.ID)
		}
	}

	return &pb.InfoResponse{
		Type:     self.Type,
		Operator: &self.Operator,
		Capacity: &self.Capacity,
		Version:  &self.Version,
	}, nil
}
