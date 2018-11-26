// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

// NewMockServer provides a mock grpc server for testing
func NewMockServer(items []storage.ListItem, opts ...grpc.ServerOption) *grpc.Server {
	grpcServer := grpc.NewServer(opts...)

	registry := monkit.Default

	k := kademlia.NewMockKademlia()

	c := &Cache{
		DB:  teststore.New(),
		DHT: k,
	}

	_ = storage.PutAll(c.DB, items...)

	s := Server{
		dht:     k,
		cache:   c,
		logger:  zap.NewNop(),
		metrics: registry,
	}
	pb.RegisterOverlayServer(grpcServer, &s)

	return grpcServer
}
