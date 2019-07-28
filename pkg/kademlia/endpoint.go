// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync/atomic"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
)

// EndpointError defines errors class for Endpoint
var EndpointError = errs.Class("kademlia endpoint error")

// Endpoint implements the kademlia Endpoints
type Endpoint struct {
	log          *zap.Logger
	service      *Kademlia
	routingTable *RoutingTable
	connected    int32
}

// NewEndpoint returns a new kademlia endpoint
func NewEndpoint(log *zap.Logger, service *Kademlia, routingTable *RoutingTable) *Endpoint {
	return &Endpoint{
		service:      service,
		routingTable: routingTable,
		log:          log,
	}
}

// Query is a node to node communication query
func (endpoint *Endpoint) Query(ctx context.Context, req *pb.QueryRequest) (_ *pb.QueryResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	endpoint.service.Queried()

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
	endpoint.service.Pinged()
	return &pb.PingResponse{}, nil
}

// RequestInfo returns the node info
func (endpoint *Endpoint) RequestInfo(ctx context.Context, req *pb.InfoRequest) (_ *pb.InfoResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	self := endpoint.service.Local()

	return &pb.InfoResponse{
		Type:     self.Type,
		Operator: &self.Operator,
		Capacity: &self.Capacity,
		Version:  &self.Version,
	}, nil
}
