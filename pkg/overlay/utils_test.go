// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"testing"

	proto "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
	"storj.io/storj/storage/teststore"
)

// NewMockServer provides a mock grpc server for testing
func NewMockServer(items []storage.ListItem) *grpc.Server {
	grpcServer := grpc.NewServer()

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

// NewNodeAddressValue provides a convient way to create a storage.Value for testing purposes
func NewNodeAddressValue(t *testing.T, address string) storage.Value {
	na := &pb.Node{Id: "", Address: &pb.NodeAddress{Transport: pb.NodeTransport_TCP, Address: address}}
	d, err := proto.Marshal(na)
	assert.NoError(t, err)

	return d
}
