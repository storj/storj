// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/storj"
	"storj.io/storj/pkg/pb"
)

func TestXorQueue(t *testing.T) {
	target := teststorj.NodeIDFromBytes([]byte{1})
	testValues := []byte{3, 6, 7, 8}      // 0011, 0110, 0111, 1000
	expectedPriority := []int{2, 6, 7, 9} // 0010=>2, 0111=>7, 0110=>6, 1001=>9
	expectedIds := []byte{3, 7, 6, 8}

	nodes := make([]*pb.Node, len(testValues))
	for i, v := range testValues {
		nodes[i] = &pb.Node{Id: teststorj.NodeIDFromBytes([]byte{v})}
	}
	// populate queue
	pq := NewXorQueue(3)
	pq.Insert(target, nodes)
	// make sure we remove as many things as the queue should hold
	assert.Equal(t, pq.Len(), 3)
	for i := 0; pq.Len() > 0; i++ {
		node, priority := pq.Closest()
		assert.Equal(t, *big.NewInt(int64(expectedPriority[i])), priority)
		assert.Equal(t, []byte{expectedIds[i]}, node.Id[:1])
	}
	// test that reading beyong length returns nil
	node, _ := pq.Closest()
	assert.Nil(t, node)
}
