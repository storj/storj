// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"testing"

	"storj.io/storj/internal/pkg/node"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/protos/overlay"
)

func TestBoostrap(t *testing.T) {
	rt := createRT(nil)

	cases := []struct {
		k           Kademlia
		expected    []*overlay.Node
		expectedErr error
		n           []*overlay.Node
	}{
		{
			k: Kademlia{
				routingTable: rt,
				nodeClient:   node.NewMockClient(nil),
			},
			expected:    nil,
			expectedErr: BootstrapErr.New("no bootstrap nodes provided"),
		},
		{
			k: Kademlia{
				routingTable:   rt,
				nodeClient:     node.NewMockClient(nil),
				bootstrapNodes: []overlay.Node{overlay.Node{Id: "hello"}},
			},
			expected:    nil,
			expectedErr: BootstrapErr.New("Bootstrap node provided no known nodes"),
		},
		{
			k: Kademlia{
				routingTable:   rt,
				nodeClient:     node.NewMockClient([]*overlay.Node{&overlay.Node{Id: "world"}}),
				bootstrapNodes: []overlay.Node{overlay.Node{Id: "hello"}},
			},
			expected:    nil,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		err := v.k.Bootstrap(context.Background())
		if v.expectedErr != nil || err != nil {
			assert.EqualError(t, v.expectedErr, err.Error())
		}

		// TODO(coyle): check routing tables after that portion has been completed
	}

}

func TestLookup(t *testing.T) {
	rt := createRT(nil)

	cases := []struct {
		k           Kademlia
		expected    []*overlay.Node
		expectedErr error
		n           []*overlay.Node
	}{
		{
			k: Kademlia{
				routingTable: rt,
				nodeClient:   node.NewMockClient([]*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}}),
			},
			expected:    []*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}},
			expectedErr: nil,
			n:           []*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}},
		},
		{
			k: Kademlia{
				routingTable: rt,
				nodeClient:   node.NewMockClient([]*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}}),
			},
			expected:    []*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}, &overlay.Node{Id: string([]byte{255, 255})}},
			expectedErr: nil,
			n:           []*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}, &overlay.Node{Id: string([]byte{255, 255})}},
		},
		{
			k: Kademlia{
				routingTable: rt,
				nodeClient:   node.NewMockClient([]*overlay.Node{&overlay.Node{Id: string([]byte{255, 255})}}),
			},
			expected:    nil,
			expectedErr: NodeErr.New("no nodes provided for lookup"),
			n:           []*overlay.Node{},
		},
	}

	for _, _ = range cases {
		// actual, err := v.k.lookup(context.Background(), v.n)
		// if v.expectedErr != nil || err != nil {
		// 	assert.EqualError(t, v.expectedErr, err.Error())
		// }

		// assert.Equal(t, v.expected, actual)
	}
}
