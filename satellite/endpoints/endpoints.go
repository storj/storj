// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package endpoints

import (
	"context"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
)

var (
	mon = monkit.Package()
)

// Server implements the network state RPC service
type Server struct {
	logger *zap.Logger
	config Config
}

// NewServer creates instance of Server
func NewServer(logger *zap.Logger, config Config) *Server {
	return &Server{
		logger: logger,
		config: config,
	}
}

// Close closes resources
func (s *Server) Close() error { return nil }

// Health returns the health of a specific path
func (s *Server) Health(ctx context.Context, req *pb.FileHealthRequest) (resp *pb.FileHealthResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	resp = &pb.FileHealthResponse{}

	// Find segements by file_name and uplink_id
	// for each segment
	// 		determine number of good nodes and bad nodes
	// 		append to Response

	return resp, nil
}
