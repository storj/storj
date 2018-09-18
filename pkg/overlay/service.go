// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
)

// NewServer creates a new Overlay Service Server
func NewServer(k *kademlia.Kademlia, cache *Cache, l *zap.Logger, m *monkit.Registry) *grpc.Server {
	grpcServer := grpc.NewServer()
	pb.RegisterOverlayServer(grpcServer, &Server{
		dht:     k,
		cache:   cache,
		logger:  l,
		metrics: m,
	})

	return grpcServer
}

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr string, opts ...grpc.DialOption) (pb.OverlayClient, error) {
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return pb.NewOverlayClient(conn), nil
}
