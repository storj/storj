// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"sync/atomic"

	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// Server implements the grpc Node Server
type Server struct {
	dht dht.DHT
	log *zap.Logger

	connected int32
}

// NewServer returns a newly instantiated Node Server
func NewServer(log *zap.Logger, dht dht.DHT) *Server {
	return &Server{
		dht: dht,
		log: log,
	}
}

// Query is a node to node communication query
func (server *Server) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	rt, err := server.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.QueryResponse{}, NodeClientErr.New("could not get routing table %server", err)
	}

	if req.GetPingback() {
		_, err = server.dht.Ping(ctx, *req.Sender)
		if err != nil {
			server.log.Debug("connection to node failed", zap.Error(err), zap.String("nodeID", req.Sender.Id.String()))
			err = rt.ConnectionFailed(req.Sender)
			if err != nil {
				server.log.Error("could not respond to connection failed", zap.Error(err))
			}
		} else {
			err = rt.ConnectionSuccess(req.Sender)
			if err != nil {
				server.log.Error("could not respond to connection success", zap.Error(err))
			} else {
				count := atomic.AddInt32(&server.connected, 1)
				if count == 1 {
					server.log.Sugar().Debugf("Successfully connected with %s", req.Sender.Address.Address)
				} else if count%100 == 0 {
					server.log.Sugar().Debugf("Successfully connected with %s %dx times", req.Sender.Address.Address, count)
				}
			}
		}
	}

	nodes, err := rt.FindNear(req.Target.Id, int(req.Limit))
	if err != nil {
		return &pb.QueryResponse{}, NodeClientErr.New("could not find near %server", err)
	}

	return &pb.QueryResponse{Sender: req.Sender, Response: nodes}, nil
}

// Ping provides an easy way to verify a node is online and accepting requests
func (server *Server) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	//TODO
	return &pb.PingResponse{}, nil
}
