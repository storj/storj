// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/protos/netstate"
)

// NewServer creates a new NetState Server
func NewServer(logger *zap.Logger, db Client) *grpc.Server {
	grpcServer := grpc.NewServer()
	netstate.RegisterNetStateServer(grpcServer, &NetState{
		DB:     db,
		logger: logger,
	})

	return grpcServer
}

// NewClient connects to a grpc server at the given address with the provided options
// and returns a new instance of a netstate client
func NewClient(serverAddr *string, opts ...grpc.DialOption) (netstate.NetStateClient, error) {
	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return netstate.NewNetStateClient(conn), nil
}
