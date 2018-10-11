// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
)

// helper function to get kademlia base configs without root Config struct
func kadconfig() KadConfig {
	return KadConfig{
		Alpha:                       5,
		DefaultIDLength:             256,
		DefaultBucketSize:           20,
		DefaultReplacementCacheSize: 5,
	}
}

func TestNewKademlia(t *testing.T) {
	cases := []struct {
		id          dht.NodeID
		bn          []pb.Node
		addr        string
		expectedErr error
		setup       func() error
	}{
		{
			id: func() *node.ID {
				id, err := node.NewID()
				assert.NoError(t, err)
				return id
			}(),
			bn:    []pb.Node{pb.Node{Id: "foo"}},
			addr:  "127.0.0.1:8080",
			setup: func() error { return nil },
		},
		{
			id: func() *node.ID {
				id, err := node.NewID()
				assert.NoError(t, err)
				return id
			}(),
			bn:    []pb.Node{pb.Node{Id: "foo"}},
			addr:  "127.0.0.1:8080",
			setup: func() error { return os.RemoveAll("db") },
		},
	}

	for _, v := range cases {
		assert.NoError(t, v.setup())
		kc := kadconfig()
		ca, err := provider.NewCA(ctx, 12, 4)
		assert.NoError(t, err)
		identity, err := ca.NewIdentity()
		assert.NoError(t, err)
		actual, err := NewKademlia(v.id, v.bn, v.addr, identity, "db", kc)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, actual.bootstrapNodes, v.bn)
		assert.NotNil(t, actual.nodeClient)
		assert.NotNil(t, actual.routingTable)
	}
}

func TestLookup(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	addr := lis.Addr().String()

	assert.NoError(t, err)
	kc := kadconfig()

	srv, mns := newTestServer([]*pb.Node{&pb.Node{Id: "foo"}})
	go func() { _ = srv.Serve(lis) }()
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
		k, err := NewKademlia(id, []pb.Node{pb.Node{Id: id2.String(), Address: &pb.NodeAddress{Address: addr}}}, addr, identity, "db", kc)
		assert.NoError(t, err)
		return k
	}()

	cases := []struct {
		k           *Kademlia
		target      dht.NodeID
		opts        lookupOpts
		expected    *pb.Node
		expectedErr error
	}{
		{
			k: k,
			target: func() *node.ID {
				id, err := node.NewID()
				assert.NoError(t, err)
				mns.returnValue = []*pb.Node{&pb.Node{Id: id.String(), Address: &pb.NodeAddress{Address: addr}}}
				return id
			}(),
			opts:        lookupOpts{amount: 5},
			expected:    &pb.Node{},
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
	}

}

func TestBootstrap(t *testing.T) {
	bn, s := testNode(t, []pb.Node{})
	defer s.Stop()

	n1, s1 := testNode(t, []pb.Node{*bn.routingTable.self})
	defer s1.Stop()

	err := n1.Bootstrap(context.Background())
	assert.NoError(t, err)

	n2, s2 := testNode(t, []pb.Node{*bn.routingTable.self})
	defer s2.Stop()

	err = n2.Bootstrap(context.Background())
	assert.NoError(t, err)
	
	nodeIDs, err := n2.routingTable.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Len(t, nodeIDs, 3)

}

func testNode(t *testing.T, bn []pb.Node) (*Kademlia, *grpc.Server) {
	// new address
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	// new config
	kc := kadconfig()
	// new ID
	id, err := node.NewID()
	assert.NoError(t, err)
	// New identity
	ca, err := provider.NewCA(ctx, 12, 4)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)
	// new kademlia
	k, err := NewKademlia(id, bn, lis.Addr().String(), identity, "db", kc)
	assert.NoError(t, err)
	s := node.NewServer(k)

	identOpt, err := identity.ServerOption()
	assert.NoError(t, err)

	grpcServer := grpc.NewServer(identOpt)

	pb.RegisterNodesServer(grpcServer, s)
	go func() { _ = grpcServer.Serve(lis) }()

	return k, grpcServer

}
