// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"sync"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

var fooID = teststorj.NodeIDFromString("foo")

func TestGet(t *testing.T) {
	cases := []struct {
		pool          *ConnectionPool
		nodeID        storj.NodeID
		expected      Conn
		expectedError error
	}{
		{
			pool: func() *ConnectionPool {
				p := NewConnectionPool(newTestIdentity(t))
				p.Init()
				p.items[fooID] = &Conn{addr: "foo"}
				return p
			}(),
			nodeID:        fooID,
			expected:      Conn{addr: "foo"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		test, err := v.pool.Get(v.nodeID)
		assert.Equal(t, v.expectedError, err)

		assert.Equal(t, v.expected.addr, test.(*Conn).addr)
	}
}

func TestDisconnect(t *testing.T) {

	conn, err := grpc.Dial("127.0.0.1:0", grpc.WithInsecure())
	assert.NoError(t, err)
	// gc.Close = func() error { return nil }
	cases := []struct {
		pool          ConnectionPool
		nodeID        storj.NodeID
		expected      interface{}
		expectedError error
	}{
		{
			pool: ConnectionPool{
				mu:    sync.RWMutex{},
				items: map[storj.NodeID]*Conn{fooID: &Conn{grpc: unsafe.Pointer(conn)}},
			},
			nodeID:        fooID,
			expected:      nil,
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		err := v.pool.Disconnect(v.nodeID)
		assert.Equal(t, v.expectedError, err)

		test, err := v.pool.Get(v.nodeID)
		assert.Equal(t, v.expectedError, err)

		assert.Equal(t, v.expected, test)
	}
}

func TestDial(t *testing.T) {
	t.Skip()
	cases := []struct {
		pool          *ConnectionPool
		node          *pb.Node
		expectedError error
		expected      *Conn
	}{
		{
			pool:          NewConnectionPool(newTestIdentity(t)),
			node:          &pb.Node{Id: fooID, Address: &pb.NodeAddress{Address: "127.0.0.1:0"}},
			expected:      nil,
			expectedError: nil,
		},
	}

	for _, v := range cases {
		wg := sync.WaitGroup{}
		wg.Add(4)
		go testDial(t, &wg, v.pool, v.node, v.expectedError)
		go testDial(t, &wg, v.pool, v.node, v.expectedError)
		go testDial(t, &wg, v.pool, v.node, v.expectedError)
		go testDial(t, &wg, v.pool, v.node, v.expectedError)
		wg.Wait()
	}

}

func testDial(t *testing.T, wg *sync.WaitGroup, p *ConnectionPool, n *pb.Node, eerr error) {
	defer wg.Done()
	ctx := context.Background()
	actual, err := p.Dial(ctx, n)
	assert.Equal(t, eerr, err)
	assert.NotNil(t, actual)
}
