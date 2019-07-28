// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

func TestDiscoveryQueue(t *testing.T) {
	target := storj.NodeID{1, 1} // 00000001

	//                                          // id                -> id ^ target
	nodeA := &pb.Node{Id: storj.NodeID{3, 2}}   // 00000011:00000010 -> 00000010:00000011
	nodeB := &pb.Node{Id: storj.NodeID{6, 5}}   // 00000110:00000101 -> 00000111:00000100
	nodeC := &pb.Node{Id: storj.NodeID{7, 7}}   // 00000111:00000111 -> 00000110:00000110
	nodeD := &pb.Node{Id: storj.NodeID{8, 4}}   // 00001000:00000100 -> 00001001:00000101
	nodeE := &pb.Node{Id: storj.NodeID{12, 1}}  // 00001100:00000001 -> 00001101:00000000
	nodeF := &pb.Node{Id: storj.NodeID{15, 16}} // 00001111:00010000 -> 00001110:00010001
	nodeG := &pb.Node{Id: storj.NodeID{18, 74}} // 00010010:01001010 -> 00010011:01001011
	nodeH := &pb.Node{Id: storj.NodeID{25, 61}} // 00011001:00111101 -> 00011000:00111100

	nodes := []*pb.Node{nodeA, nodeB, nodeC, nodeD, nodeE, nodeF, nodeG, nodeH}

	expected := []*pb.Node{
		nodeA, // 00000011:00000010 -> 00000010:00000011
		nodeC, // 00000111:00000111 -> 00000110:00000110
		nodeB, // 00000110:00000101 -> 00000111:00000100
		nodeD, // 00001000:00000100 -> 00001001:00000101
		nodeE, // 00001100:00000001 -> 00001101:00000000
		nodeF, // 00001111:00010000 -> 00001110:00010001
		// nodeG, // 00010010:01001010 -> 00010011:01001011
		// nodeH, // 00011001:00111101 -> 00011000:00111100
	}

	// // code for outputting the bits above
	// for _, node := range nodes {
	//     xor := xorNodeID(target, node.Id)
	//     t.Logf("%08b,%08b -> %08b,%08b", node.Id[0], node.Id[1], xor[0], xor[1])
	// }

	queue := newDiscoveryQueue(target, 6)
	queue.Insert(nodes...)

	assert.Equal(t, queue.Unqueried(), 6)

	for i, expect := range expected {
		node := queue.ClosestUnqueried()
		assert.Equal(t, node.Id, expect.Id, strconv.Itoa(i))
	}

	assert.Nil(t, queue.ClosestUnqueried())
}

func TestDiscoveryQueueRandom(t *testing.T) {
	const maxLen = 8

	seed := int64(rand.Uint64())
	t.Logf("seed %v", seed)

	r := rand.New(rand.NewSource(seed))

	for i := 0; i < 100; i++ {
		var target storj.NodeID
		_, _ = r.Read(target[:])

		var initial []*pb.Node
		for k := 0; k < 10; k++ {
			var nodeID storj.NodeID
			_, _ = r.Read(nodeID[:])
			initial = append(initial, &pb.Node{Id: nodeID})
		}

		queue := newDiscoveryQueue(target, maxLen)
		queue.Insert(initial...)

		for k := 0; k < 10; k++ {
			var nodeID storj.NodeID
			_, _ = r.Read(nodeID[:])
			queue.Insert(&pb.Node{Id: nodeID})
		}

		assert.Equal(t, queue.Unqueried(), maxLen)

		previousPriority := storj.NodeID{}
		for queue.Unqueried() > 0 {
			next := queue.ClosestUnqueried()
			priority := xorNodeID(target, next.Id)
			// ensure that priority is monotonically increasing
			assert.False(t, priority.Less(previousPriority))
		}
	}
}
