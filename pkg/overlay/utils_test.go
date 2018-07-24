// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"testing"

	protob "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

// NewMockServer provides a mock grpc server for testing
func NewMockServer(kv test.KvStore) *grpc.Server {
	grpcServer := grpc.NewServer()

	registry := monkit.Default

	k := kademlia.NewMockKademlia()

	c := &Cache{
		DB:  test.NewMockKeyValueStore(kv),
		DHT: k,
	}

	s := Server{
		dht:     k,
		cache:   c,
		logger:  zap.NewNop(),
		metrics: registry,
	}
	proto.RegisterOverlayServer(grpcServer, &s)

	return grpcServer
}

// NewNodeAddressValue provides a convient way to create a storage.Value for testing purposes
func NewNodeAddressValue(t *testing.T, address string) storage.Value {
	na := &proto.NodeAddress{Transport: proto.NodeTransport_TCP, Address: address}
	d, err := protob.Marshal(na)
	assert.NoError(t, err)

	return d
}
