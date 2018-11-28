// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"storj.io/storj/pkg/pb"
)

func TestGet(t *testing.T) {
	cases := []struct {
		pool          *ConnectionPool
		key           string
		expected      Conn
		expectedError error
	}{
		{
			pool: func() *ConnectionPool {
				p := NewConnectionPool(testidentity.NewTestIdentity(t))
				p.Init()
				p.items["foo"] = &Conn{addr: "foo"}
				return p
			}(),
			key:           "foo",
			expected:      Conn{addr: "foo"},
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		test, err := v.pool.Get(v.key)
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
		key           string
		expected      interface{}
		expectedError error
	}{
		{
			pool: ConnectionPool{
				mu:    sync.RWMutex{},
				items: map[string]*Conn{"foo": &Conn{grpc: conn}},
			},
			key:           "foo",
			expected:      nil,
			expectedError: nil,
		},
	}

	for i := range cases {
		v := &cases[i]
		err := v.pool.Disconnect(v.key)
		assert.Equal(t, v.expectedError, err)

		test, err := v.pool.Get(v.key)
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
			pool:          NewConnectionPool(testidentity.NewTestIdentity(t)),
			node:          &pb.Node{Id: "foo", Address: &pb.NodeAddress{Address: "127.0.0.1:0"}},
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
