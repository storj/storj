// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestNewKademlia(t *testing.T) {
	cases := []struct {
		expected    *Kademlia
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
		actual, err := NewKademlia(v.id, v.bn, v.addr)
		assert.Equal(t, v.expectedErr, err)
		assert.Equal(t, actual.bootstrapNodes, v.bn)
		assert.Equal(t, actual.stun, true)
		assert.NotNil(t, actual.nodeClient)
		assert.NotNil(t, actual.routingTable)
	}
}

func TestLookup(t *testing.T) {
	t.Skip()
	lis, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)

	srv, mns := newTestServer([]*proto.Node{&proto.Node{Id: "foo"}})
	go srv.Serve(lis)
	defer srv.Stop()

	k := func() *Kademlia {
		id, err := node.NewID()
		assert.NoError(t, err)
		id2, err := node.NewID()
		assert.NoError(t, err)
		fmt.Printf("ADDRESS==%v\n", lis.Addr().String())
		k, err := NewKademlia(id, []proto.Node{proto.Node{Id: id2.String(), Address: &proto.NodeAddress{Address: lis.Addr().String()}}}, lis.Addr().String())
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
				mns.returnValue = []*proto.Node{&proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: lis.Addr().String()}}}
				return id
			}(),
			opts:        lookupOpts{amount: 5},
			expected:    &proto.Node{},
			expectedErr: nil,
		},
		// {
		// 	k: k,
		// 	target: func() *node.ID {
		// 		id, err := node.NewID()
		// 		assert.NoError(t, err)
		// 		return id
		// 	}(),
		// 	opts:        lookupOpts{amount: 5},
		// 	expected:    nil,
		// 	expectedErr: NodeNotFound,
		// },
	}

	for _, v := range cases {
		actual, err := v.k.lookup(context.Background(), v.target, v.opts)
		assert.Equal(t, v.expectedErr, err)
		if v.expected != nil {
			assert.Equal(t, v.target.String(), actual.GetId())
		} else {
			assert.Nil(t, actual)
		}

		time.Sleep(1 * time.Second)
	}

	assert.NoError(t, os.Remove("kbucket.db"))
	assert.NoError(t, os.Remove("nbucket.db"))
}

func TestBootstrap(t *testing.T) {
	// m := map[string]bool{}
	// id, err := node.NewID()
	// assert.NoError(t, err)
	// m[id.String()] = false
	nodes := createTestNodes(20)
	defer cleanup(nodes)

	// var last *grpc.Server
	// for k, v := range nodes {
	// 	// v.returnValue = []proto.Node{&proto.Node{Address: &proto.NodeAddress{Address: v.}}}
	// }

	// bn, err := testServer([]*proto.Node{&proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: ":0"}}, &proto.Node{Id: "2", Address: &proto.NodeAddress{Address: "127.0.0.1:8882"}}})
	// assert.NoError(t, err)
	// defer bn.Stop()

	// id, err = node.NewID()
	// assert.NoError(t, err)
	// m[id.String()] = false
	// bn1, err := testServer([]*proto.Node{&proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: ":0"}}})
	// assert.NoError(t, err)
	// defer bn1.Stop()

	// id, err = node.NewID()
	// assert.NoError(t, err)
	// m[id.String()] = false
	// bn2, err := testServer([]*proto.Node{&proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: ":0"}}})
	// assert.NoError(t, err)
	// defer bn2.Stop()

	// nn1, err := testServer([]*proto.Node{})
	// assert.NoError(t, err)
	// defer nn1.Stop()

	// nn2, err := testServer([]*proto.Node{})
	// assert.NoError(t, err)
	// defer nn2.Stop()

	// id, err = node.NewID()
	// assert.NoError(t, err)
	// m[id.String()] = false
	// bid, err := node.NewID()
	// assert.NoError(t, err)
	// m[id.String()] = false
	// self, err := testServer([]*proto.Node{})
	// assert.NoError(t, err)
	// defer self.Stop()
	// k, err := NewKademlia(id, []proto.Node{proto.Node{Id: bid.String(), Address: &proto.NodeAddress{Address: "127.0.0.1:8880"}}}, "127.0.0.1:8080")
	// assert.NoError(t, err)

	// assert.Error(t, k.Bootstrap(context.Background()))

	// keys, err := k.routingTable.nodeBucketDB.List(nil, 0)

	// for _, v := range keys {
	// 	m[v.String()] = true
	// }
	// for _, v := range m {
	// 	assert.True(t, v)
	// }

	// assert.NoError(t, err)
	// assert.Len(t, keys, 6)

	// assert.NoError(t, os.Remove("kbucket.db"))
	// assert.NoError(t, os.Remove("nbucket.db"))
}

func testServer() (*grpc.Server, *mockNodeServer, error) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, nil, err
	}
	srv, mns := newTestServer([]*proto.Node{})

	mns.listener = lis.Addr()

	go srv.Serve(lis)

	return srv, mns, nil
}

func createTestNodes(n int) map[*grpc.Server]*mockNodeServer {
	m := map[*grpc.Server]*mockNodeServer{}

	for i := 0; i < n; i++ {
		srv, mns, err := testServer()
		if err != nil {

		}
		m[srv] = mns
	}

	return m
}

func cleanup(n map[*grpc.Server]*mockNodeServer) {
	for k := range n {
		k.Stop()
	}
}

// bootstrap node
// want bootstrap node to tell it about two nodes
// each of the two nodes to tell it about a node
// each of those nodes to return empty
// routing table should contain all contacted nodes
