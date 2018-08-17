// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"container/heap"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestPriorityQueue(t *testing.T) {
	cases := []struct {
		target   *big.Int
		nodes    map[string]*proto.Node
		pq       PriorityQueue
		expected []int
	}{
		{
			target: func() *big.Int {
				i, ok := new(big.Int).SetString("0001", 2)
				assert.True(t, ok)
				return i
			}(),
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
			bn, ok := new(big.Int).SetString(id, 2)
			assert.True(t, ok)
			v.pq[i] = &Item{
				value:    value,
				priority: new(big.Int).Xor(v.target, bn),
				index:    i,
			}
			i++
		}
		heap.Init(&v.pq)

		i = 0
		for v.pq.Len() > 0 {
			item := heap.Pop(&v.pq).(*Item)
			assert.Equal(t, &v.expected[i], item.priority)
			i++
		}

	}

}
