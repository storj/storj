// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package transport

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/pkg/provider"
)

func TestDialNode(t *testing.T) {
	ca, err := provider.NewCA(context.Background(), 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	oc := Transport{
		identity: identity,
	}

	// node.Address.Address == "" condition test
	node := proto.Node{
		Id: "DUMMYID1",
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   "",
		},
	}
	conn, err := oc.DialNode(context.Background(), &node)
	assert.Error(t, err)
	assert.Nil(t, conn)

	// node.Address == nil condition test
	node = proto.Node{
		Id:      "DUMMYID2",
		Address: nil,
	}
	conn, err = oc.DialNode(context.Background(), &node)
	assert.Error(t, err)
	assert.Nil(t, conn)

	// node is valid argument condition test
	node = proto.Node{
		Id: "DUMMYID3",
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   "127.0.0.0:9000",
		},
	}
	conn, err = oc.DialNode(context.Background(), &node)
	assert.NoError(t, err)
	assert.NotNil(t, conn)
}
