// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestPriorityQueue(t *testing.T) {
	cases := []struct {
		target   []byte
		nodes    map[string]*proto.Node
		pq       PriorityQueue
		expected []int
	}{
		{
			target: []byte("0001"),
			nodes: map[string]*proto.Node{
				"1001": &proto.Node{Id: "1001"},
				"0100": &proto.Node{Id: "0100"},
				"1100": &proto.Node{Id: "1100"},
				"0010": &proto.Node{Id: "0010"},
			},
			pq:       make(PriorityQueue, 4),
			expected: []int{3, 5, 8, 13},
		},
	}

	for _, v := range cases {
		i := 0
		for id, value := range v.nodes {
			priority, _ := xor([]byte(id), v.target)
			v.pq[i] = &Item{
				value:    value,
				priority: priority,
				index:    i,
			}
			i++
		}
		heap.Init(&v.pq)

		i = 0
		for v.pq.Len() > 0 {
			item := heap.Pop(&v.pq).(*Item)
			assert.Equal(t, v.expected[i], item.priority)
			i++
		}

	}

}
