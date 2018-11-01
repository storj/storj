// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"container/heap"
	"math/big"
	"sync"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// XorQueue is a priority queue where the priority is key XOR distance
type XorQueue struct {
	maxLen int

	mu    sync.Mutex
	added map[string]int
	items items
}

// NewXorQueue returns a items with priority based on XOR from targetBytes
func NewXorQueue(size int) *XorQueue {
	return &XorQueue{
		items:  make(items, 0, size),
		added:  make(map[string]int),
		maxLen: size,
	}
}

// Insert adds Nodes onto the queue
func (x *XorQueue) Insert(target dht.NodeID, nodes []*pb.Node) {
	x.mu.Lock()
	defer x.mu.Unlock()

	unique := nodes[:0]
	for _, node := range nodes {
		nodeID := node.GetId()
		if _, added := x.added[nodeID]; !added {
			x.added[nodeID]++
			unique = append(unique, node)
		}
	}

	x.insert(target, unique)
}

// Reinsert adds a Nodes onto the queue if it's been added >= limit times previously
func (x *XorQueue) Reinsert(target dht.NodeID, node *pb.Node, limit int) bool {
	x.mu.Lock()
	defer x.mu.Unlock()

	nodeID := node.GetId()
	if x.added[nodeID] >= limit {
		return false
	}
	x.added[nodeID]++

	x.insert(target, []*pb.Node{node})
	return true
}

// insert must hold lock while adding
func (x *XorQueue) insert(target dht.NodeID, nodes []*pb.Node) {
	targetBytes := new(big.Int).SetBytes(target.Bytes())
	// insert new nodes
	for _, node := range nodes {
		heap.Push(&x.items, &item{
			value:    node,
			priority: new(big.Int).Xor(targetBytes, new(big.Int).SetBytes([]byte(node.GetId()))),
		})
	}
	// resize down if we grew too big
	if x.items.Len() > x.maxLen {
		olditems := x.items
		x.items = items{}
		for i := 0; i < x.maxLen && len(olditems) > 0; i++ {
			item := heap.Pop(&olditems)
			heap.Push(&x.items, item)
		}
		heap.Init(&x.items)
	}
}

// Closest removes the closest priority node from the queue
func (x *XorQueue) Closest() (*pb.Node, big.Int) {
	x.mu.Lock()
	defer x.mu.Unlock()

	if x.Len() == 0 {
		return nil, big.Int{}
	}
	item := *(heap.Pop(&x.items).(*item))
	return item.value, *item.priority
}

// Len returns the number of items in the queue
func (x *XorQueue) Len() int {
	return x.items.Len()
}

// An item is something we manage in a priority queue.
type item struct {
	value    *pb.Node // The value of the item; arbitrary.
	priority *big.Int // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A items implements heap.Interface and holds items.
type items []*item

// Len returns the length of the priority queue
func (items items) Len() int { return len(items) }

// Less does what you would think
func (items items) Less(i, j int) bool {
	// this sorts the nodes where the node popped has the closest location
	return items[i].priority.Cmp(items[j].priority) < 0
}

// Swap swaps two ints
func (items items) Swap(i, j int) {
	items[i], items[j] = items[j], items[i]
	items[i].index = i
	items[j].index = j
}

// Push adds an item to the top of the queue
// must call heap.fix to resort
func (items *items) Push(x interface{}) {
	n := len(*items)
	item := x.(*item)
	item.index = n
	*items = append(*items, item)
}

// Pop returns the item with the lowest priority
func (items *items) Pop() interface{} {
	old := *items
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*items = old[0 : n-1]
	return item
}
