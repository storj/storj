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
func (endpoint *Endpoint) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	if req.GetPingback() {
		endpoint.pingback(ctx, req.Sender)
	}

	nodes, err := endpoint.routingTable.FindNear(req.Target.Id, int(req.Limit))
	if err != nil {
		return &pb.QueryResponse{}, EndpointError.New("could not find near endpoint: %v", err)
	}

	return &pb.QueryResponse{Sender: req.Sender, Response: nodes}, nil
}

// pingback implements pingback for queries
func (endpoint *Endpoint) pingback(ctx context.Context, target *pb.Node) {
	_, err := endpoint.service.Ping(ctx, *target)
	if err != nil {
		endpoint.log.Debug("connection to node failed", zap.Error(err), zap.String("nodeID", target.Id.String()))
		err = endpoint.routingTable.ConnectionFailed(target)
		if err != nil {
			endpoint.log.Error("could not respond to connection failed", zap.Error(err))
		}
	} else {
		err = endpoint.routingTable.ConnectionSuccess(target)
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
func (endpoint *Endpoint) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	//TODO
	return &pb.PingResponse{}, nil
}
