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
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
	assert.NoError(t, err)

	srv, mns := newTestServer([]*proto.Node{&proto.Node{Id: "foo"}})
	go srv.Serve(lis)
	defer srv.GracefulStop()

	k := func() *Kademlia {
		id, err := node.NewID()
		assert.NoError(t, err)
		id2, err := node.NewID()
		assert.NoError(t, err)

		k, err := NewKademlia(id, []proto.Node{proto.Node{Id: id2.String(), Address: &proto.NodeAddress{Address: "127.0.0.1:8080"}}}, "127.0.0.1:8080")
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
				mns.returnValue = []*proto.Node{&proto.Node{Id: id.String(), Address: &proto.NodeAddress{Address: "127.0.0.1:8080"}}}
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
			expectedErr: NodeNotFound,
		},
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
	bn, err := testServer([]*proto.Node{&proto.Node{Id: "foobar0", Address: &proto.NodeAddress{Address: "127.0.0.1:8881"}}, &proto.Node{Id: "foobar1", Address: &proto.NodeAddress{Address: "127.0.0.1:8882"}}}, 8880)
	assert.NoError(t, err)
	defer bn.GracefulStop()

	bn1, err := testServer([]*proto.Node{&proto.Node{Id: "foobar1", Address: &proto.NodeAddress{Address: "127.0.0.1:8883"}}}, 8881)
	assert.NoError(t, err)
	defer bn1.GracefulStop()

	bn2, err := testServer([]*proto.Node{&proto.Node{Id: "foobar2", Address: &proto.NodeAddress{Address: "127.0.0.1:8884"}}}, 8882)
	assert.NoError(t, err)
	defer bn2.GracefulStop()

	nn1, err := testServer([]*proto.Node{}, 8883)
	assert.NoError(t, err)
	defer nn1.GracefulStop()

	nn2, err := testServer([]*proto.Node{}, 8884)
	assert.NoError(t, err)
	defer nn2.GracefulStop()

	id, err := node.NewID()
	assert.NoError(t, err)
	k, err := NewKademlia(id, []proto.Node{proto.Node{Address: &proto.NodeAddress{Address: "127.0.0.1:8880"}}}, "127.0.0.1:8080")
	assert.NoError(t, err)

	assert.NoError(t, k.Bootstrap(context.Background()))
}

func testServer(bn []*proto.Node, port int) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	srv, mns := newTestServer(bn)
	mns.returnValue = bn
	go srv.Serve(lis)

	return srv, nil
}

// bootstrap node
// want bootstrap node to tell it about two nodes
// each of the two nodes to tell it about a node
// each of those nodes to return empty
// routing table should contain all contacted nodes
