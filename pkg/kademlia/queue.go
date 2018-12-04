// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"math/bits"
	"sort"
	"sync"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Queue is a limited priority queue with xor distance
type Queue struct {
	maxLen int
	mu     sync.Mutex
	added  map[storj.NodeID]int
	items  []queueItem
}

// An queueItem is something we manage in a priority queue.
type queueItem struct {
	node     *pb.Node
	priority storj.NodeID
	// TODO: switch to using pb.NodeAddress to avoid pointer to *pb.Node
}

// xorNodeID returns the xor of each byte in NodeID
func xorNodeID(a, b storj.NodeID) storj.NodeID {
	r := storj.NodeID{}
	for i, av := range a {
		r[i] = av ^ b[i]
	}
	return r
}

// reverseNodeID reverses NodeID bit representation
func reverseNodeID(a storj.NodeID) storj.NodeID {
	r := storj.NodeID{}
	for i, v := range a {
		r[len(a)-i-1] = bits.Reverse8(v)
	}
	return r
}

// NewQueue returns a items with priority based on XOR from targetBytes
func NewQueue(size int) *Queue {
	return &Queue{
		added:  make(map[storj.NodeID]int),
		maxLen: size,
	}
}

// Insert adds nodes into the queue.
func (queue *Queue) Insert(target storj.NodeID, nodes ...*pb.Node) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	unique := nodes[:0]
	for _, node := range nodes {
		nodeID := node.Id
		if _, added := queue.added[nodeID]; !added {
			queue.added[nodeID]++
			unique = append(unique, node)
		}
	}

	queue.insert(target, unique...)
}

// Reinsert adds a Nodes into the queue, only if it's has been added less than limit times.
func (queue *Queue) Reinsert(target storj.NodeID, node *pb.Node, limit int) bool {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	nodeID := node.Id
	if queue.added[nodeID] >= limit {
		return false
	}
	queue.added[nodeID]++

	queue.insert(target, node)
	return true
}

// insert must hold lock while adding
func (queue *Queue) insert(target storj.NodeID, nodes ...*pb.Node) {
	for _, node := range nodes {
		queue.items = append(queue.items, queueItem{
			node:     node,
			priority: reverseNodeID(xorNodeID(target, node.Id)),
		})
	}

	sort.Slice(queue.items, func(i, k int) bool {
		return queue.items[i].priority.Less(queue.items[k].priority)
	})

	if len(queue.items) > queue.maxLen {
		queue.items = queue.items[:queue.maxLen]
	}
}

// Closest returns the closest item in the queue
func (queue *Queue) Closest() *pb.Node {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	if len(queue.items) == 0 {
		return nil
	}

	var item queueItem
	item, queue.items = queue.items[0], queue.items[1:]
	return item.node
}

// Len returns the number of items in the queue
func (queue *Queue) Len() int {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	return len(queue.items)
}
