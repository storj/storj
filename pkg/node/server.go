// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// Server implements the grpc Node Server
type Server struct {
	dht    dht.DHT
	logger *zap.Logger
}

// NewServer returns a newly instantiated Node Server
func NewServer(dht dht.DHT) *Server {
	return &Server{
		dht:    dht,
		logger: zap.L(),
	}
}

// Query is a node to node communication query
func (s *Server) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	if s.logger == nil {
		s.logger = zap.L()
	}
	rt, err := s.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.QueryResponse{}, NodeClientErr.New("could not get routing table %s", err)
	}

	if req.GetPingback() {
		_, err = s.dht.Ping(ctx, *req.Sender)
		if err != nil {
			err = rt.ConnectionFailed(req.Sender)
			if err != nil {
				s.logger.Error("could not respond to connection failed", zap.Error(err))
			}
			s.logger.Error("connection to node failed", zap.Error(err), zap.String("nodeID", req.Sender.Id))
		}

		err = rt.ConnectionSuccess(req.Sender)
		if err != nil {
			s.logger.Error("could not respond to connection success", zap.Error(err))
		}
	}

	id := IDFromString(req.Target.Id)
	nodes, err := rt.FindNear(id, int(req.Limit))
	if err != nil {
		return &pb.QueryResponse{}, NodeClientErr.New("could not find near %s", err)
	}

	return &pb.QueryResponse{Sender: req.Sender, Response: nodes}, nil
}

// Ping provides an easy way to verify a node is online and accepting requests
func (s *Server) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{}, nil
}
