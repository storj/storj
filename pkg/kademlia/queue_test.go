// Copyright (C) 2018 Storj Labs, Inc.
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

func TestQueue(t *testing.T) {
	target := storj.NodeID{1, 1} // 00000001

	//                                          // id                -> id ^ target     -> reverse id ^ target
	nodeA := &pb.Node{Id: storj.NodeID{3, 2}}   // 00000011:00000010:0 -> 00000010:00000011:0 -> 0:11000000:01000000
	nodeB := &pb.Node{Id: storj.NodeID{6, 5}}   // 00000110:00000101:0 -> 00000111:00000100:0 -> 0:00100000:11100000
	nodeC := &pb.Node{Id: storj.NodeID{7, 7}}   // 00000111:00000111:0 -> 00000110:00000110:0 -> 0:01100000:01100000
	nodeD := &pb.Node{Id: storj.NodeID{8, 4}}   // 00001000:00000100:0 -> 00001001:00000101:0 -> 0:10100000:10010000
	nodeE := &pb.Node{Id: storj.NodeID{12, 1}}  // 00001100:00000001:0 -> 00001101:00000000:0 -> 0:00000000:10110000
	nodeF := &pb.Node{Id: storj.NodeID{15, 16}} // 00001111:00010000:0 -> 00001110:00010001:0 -> 0:10001000:01110000
	nodeG := &pb.Node{Id: storj.NodeID{18, 74}} // 00010010:01001010:0 -> 00010011:01001011:0 -> 0:11010010:11001000
	nodeH := &pb.Node{Id: storj.NodeID{25, 61}} // 00011001:00111101:0 -> 00011000:00111100:0 -> 0:00111100:00011000

	nodes := []*pb.Node{nodeA, nodeB, nodeC, nodeD, nodeE, nodeF, nodeG, nodeH}

	expected := []*pb.Node{
		nodeE, // 00001100:00000001:0 -> 00001101:00000000:0 -> 0:00000000:10110000
		nodeB, // 00000110:00000101:0 -> 00000111:00000100:0 -> 0:00100000:11100000
		nodeH, // 00011001:00111101:0 -> 00011000:00111100:0 -> 0:00111100:00011000
		nodeC, // 00000111:00000111:0 -> 00000110:00000110:0 -> 0:01100000:01100000
		nodeF, // 00001111:00010000:0 -> 00001110:00010001:0 -> 0:10001000:01110000
		nodeD, // 00001000:00000100:0 -> 00001001:00000101:0 -> 0:10100000:10010000
	}

	// // code for outputting the bits above
	// for _, node := range nodes {
	//     xor := xorNodeID(target, node.Id)
	//     rxor := reverseNodeID(xor)
	//     t.Logf("%08b,%08b -> %08b,%08b -> %08b,%08b", node.Id[0], node.Id[1], xor[0], xor[1], rxor[30], rxor[31])
	// }

	queue := NewQueue(6)
	queue.Insert(target, nodes...)

	assert.Equal(t, queue.Len(), 6)

	for i, expect := range expected {
		node := queue.Closest()
		assert.Equal(t, node.Id, expect.Id, strconv.Itoa(i))
	}

	assert.Nil(t, queue.Closest())
}

func TestQueueRandom(t *testing.T) {
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

		queue := NewQueue(maxLen)
		queue.Insert(target, initial...)

		for k := 0; k < 10; k++ {
			var nodeID storj.NodeID
			_, _ = r.Read(nodeID[:])
			queue.Insert(target, &pb.Node{Id: nodeID})
		}

		assert.Equal(t, queue.Len(), maxLen)

		previousPriority := storj.NodeID{}
		for queue.Len() > 0 {
			next := queue.Closest()
			priority := reverseNodeID(xorNodeID(target, next.Id))
			// ensure that priority is monotonically increasing
			assert.False(t, priority.Less(previousPriority))
		}
	}
}
