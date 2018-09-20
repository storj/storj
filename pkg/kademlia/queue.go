// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/big"

	"storj.io/storj/pkg/pb"
)

// An Item is something we manage in a priority queue.
type Item struct {
	value    *pb.Node // The value of the item; arbitrary.
	priority *big.Int // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

// Len returns the length of the priority queue
func (pq PriorityQueue) Len() int { return len(pq) }

// Less does what you would think
func (pq PriorityQueue) Less(i, j int) bool {
	// this sorts the nodes where the node popped has the closest location
	if i := pq[i].priority.Cmp(pq[j].priority); i < 0 {
		return true
	}

	return false
}

// Swap swaps two ints
func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds an item to the top of the queue
// must call heap.fix to resort
func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

// Pop returns the item with the lowest priority
func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
