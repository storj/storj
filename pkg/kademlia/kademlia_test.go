// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/protos/overlay"
)

func TestBoostrap(t *testing.T) {

	cases := []struct {
		k           *Kademlia
		expected    []*overlay.Node
		expectedErr error
		n           []*overlay.Node
	}{
		// {
		// 	k: func() *Kademlia {
		// 		id, err := NewID()
		// 		assert.NoError(t, err)
		// 		k, err := NewKademlia(id, []overlay.Node{}, "127.0.0.1:1111")
		// 		assert.NoError(t, err)
		// 		return k
		// 	}(),
		// 	expected:    nil,
		// 	expectedErr: BootstrapErr.New("no bootstrap nodes provided"),
		// },
		{
			k: func() *Kademlia {
				id, err := NewID()
				assert.NoError(t, err)
				k, err := NewKademlia(id, []overlay.Node{overlay.Node{Address: &overlay.NodeAddress{Address: "127.0.0.1:2222"}}}, "127.0.0.1:1111")
				assert.NoError(t, err, "NO!!!~!!!")
				return k
			}(),
			expected:    nil,
			expectedErr: BootstrapErr.New("no bootstrap nodes provided"),
		},
		// {
		// 	k: &Kademlia{
		// 		routingTable:   rt,
		// 		nodeClient:     node.NewMockClient([]*overlay.Node{&overlay.Node{Id: "world"}}),
		// 		bootstrapNodes: []overlay.Node{overlay.Node{Id: "hello"}},
		// 	},
		// 	expected:    nil,
		// 	expectedErr: nil,
		// },
	}

	for _, v := range cases {
		err := v.k.Bootstrap(context.Background())
		if v.expectedErr != nil || err != nil {
			assert.EqualError(t, v.expectedErr, err.Error())
		}

		os.Remove("kbucket")
		os.Remove("nbucket")

		// TODO(coyle): check routing tables after that portion has been completed
	}

}

func TestLookup(t *testing.T) {

}
