// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/provider"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestNewKademlia(t *testing.T) {
	cases := []struct {
		id          dht.NodeID
		bn          []proto.Node
		addr        string
		expectedErr error
	}{
		{
			id: func() *node.ID {
				id, err := node.NewID()
				assert.NoError(t, err)
				return id
			}(),
			bn:   []proto.Node{proto.Node{Id: "foo"}},
			addr: "127.0.0.1:8080",
		},
	}

	for _, v := range cases {
		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)
		actual, err := NewKademlia(v.id, v.bn, v.addr, identity)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, actual.bootstrapNodes, v.bn)
		assert.Equal(t, actual.stun, true)
		assert.NotNil(t, actual.nodeClient)
		assert.NotNil(t, actual.routingTable)
	}
}

func TestLookup(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)

	srv, mns := newTestServer([]*proto.Node{&proto.Node{Id: "foo"}})
	go srv.Serve(lis)
	defer srv.Stop()

	k := func() *Kademlia {
		// make new identity
		id, err := node.NewID()
		assert.NoError(t, err)
		id2, err := node.NewID()
		assert.NoError(t, err)
		// initialize kademlia
		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)
		k, err := NewKademlia(id, []proto.Node{proto.Node{Id: id2.String(), Address: &proto.NodeAddress{Address: lis.Addr().String()}}}, lis.Addr().String(), identity)
		assert.NoError(t, err)
		return k
	}()

	cases := []struct {
		k           *Kademlia
		target      dht.NodeID
		opts        lookupOpts
		expected    *proto.Node
		expectedErr error
	}{
		{
			k: k,
			target: func() *node.ID {
				id, err := node.NewID()
				assert.NoError(t, err)
				mns.returnValue = []*proto.Node{&proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: ":0"}}}
				return id
			}(),
			opts:        lookupOpts{amount: 5},
			expected:    &proto.Node{},
			expectedErr: nil,
		},
		{
			k: k,
			target: func() *node.ID {
				id, err := node.NewID()
				assert.NoError(t, err)
				return id
			}(),
			opts:        lookupOpts{amount: 5},
			expected:    nil,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		err := v.k.lookup(context.Background(), v.target, v.opts)
		assert.Equal(t, v.expectedErr, err)

		time.Sleep(1 * time.Second)
	}

}

func TestBootstrap(t *testing.T) {
	bn, s := testNode(t, []proto.Node{})
	defer s.Stop()

	n1, s1 := testNode(t, []proto.Node{*bn.routingTable.self})
	defer s1.Stop()

	err := n1.Bootstrap(context.Background())
	assert.NoError(t, err)

	n2, s2 := testNode(t, []proto.Node{*bn.routingTable.self})
	defer s2.Stop()

	err = n2.Bootstrap(context.Background())
	assert.NoError(t, err)
	time.Sleep(time.Second)

	nodeIDs, err := n2.routingTable.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Len(t, nodeIDs, 3)

}

func testNode(t *testing.T, bn []proto.Node) (*Kademlia, *grpc.Server) {
	// new address
	lis, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	// new ID
	id, err := node.NewID()
	assert.NoError(t, err)
	// New identity
	ca, err := provider.NewCA(ctx, 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)
	// new kademlia
	k, err := NewKademlia(id, bn, lis.Addr().String(), identity)
	assert.NoError(t, err)
	s := node.NewServer(k)

	identOpt, err := identity.ServerOption()
	assert.NoError(t, err)

	grpcServer := grpc.NewServer(identOpt)

	proto.RegisterNodesServer(grpcServer, s)
	go grpcServer.Serve(lis)

	return k, grpcServer

}
