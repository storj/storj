// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrouting

import (
	"sort"
	"sync"
	"time"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

type nodeData struct {
	node      *pb.Node
	ordering  int64
	timestamp time.Time
	fails     int
}

// Table is a routing table that tries to be as correct as possible at
// the expense of performance.
type Table struct {
	self            storj.NodeID
	bucketSize      int
	cacheSize       int
	allowedFailures int

	mu      sync.Mutex
	counter int64
	nodes   map[storj.NodeID]*nodeData
}

// New creates a new Table. self is the owning node's node id, bucketSize is
// the kademlia k value, cacheSize is the size of each bucket's replacement
// cache, and allowedFailures is the number of failures on a given node before
// the node is removed from the table.
func New(self storj.NodeID, bucketSize, cacheSize, allowedFailures int) *Table {
	return &Table{
		self:            self,
		bucketSize:      bucketSize,
		cacheSize:       cacheSize,
		allowedFailures: allowedFailures,
		nodes:           map[storj.NodeID]*nodeData{},
	}
}

// make sure the Table implements the right interface
var _ dht.RoutingTable = (*Table)(nil)

// K returns the Table's routing depth, or Kademlia k value
func (t *Table) K() int { return t.bucketSize }

// CacheSize returns the size of
func (t *Table) CacheSize() int { return t.cacheSize }

// ConnectionSuccess should be called whenever a node is successfully connected
// to. It will add or update the node's entry in the routing table.
func (t *Table) ConnectionSuccess(node *pb.Node) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// don't add ourselves
	if node.Id == t.self {
		return nil
	}

	// if the node is already here, update everything but its placement order
	if cell, exists := t.nodes[node.Id]; exists {
		cell.node = node
		cell.timestamp = time.Now()
		cell.fails = 0
		return nil
	}

	// add unconditionally (it might be going into a replacement cache)
	t.nodes[node.Id] = &nodeData{
		node:      node,
		ordering:  t.counter,
		timestamp: time.Now(),
		fails:     0,
	}
	t.counter += 1

	// prune replacement caches
	t.makeTree().walkLeaves(func(b *bucket) {
		if len(b.cache) <= t.cacheSize {
			return
		}
		for _, node := range b.cache[:len(b.cache)-t.cacheSize] {
			delete(t.nodes, node.node.Id)
		}
	})
	return nil
}

// ConnectionFailed should be called whenever a node can't be contacted.
// If a node fails more than allowedFailures times, it will be removed from
// the routing table. The failure count is reset every successful connection.
func (t *Table) ConnectionFailed(node *pb.Node) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// if the node exists and the failure is with the address we have, record
	// a failure
	if data, exists := t.nodes[node.Id]; exists &&
		addressEqual(data.node.Address, node.Address) {
		data.fails += 1

		// if we've failed too many times, remove the node
		if t.allowedFailures < data.fails {
			delete(t.nodes, node.Id)
		}
	}
	return nil
}

// FindNear will return up to limit nodes in the routing table ordered by
// kademlia xor distance from the given id.
func (t *Table) FindNear(id storj.NodeID, limit int) ([]*pb.Node, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// find all non-cache nodes
	nodes := make([]*nodeData, 0, len(t.nodes))
	t.makeTree().walkLeaves(func(b *bucket) {
		nodes = append(nodes, b.nodes...)
	})

	// sort by distance
	sort.Sort(nodeDataDistanceSorter{self: id, nodes: nodes})

	// return up to limit nodes
	if len(nodes) < limit {
		limit = len(nodes)
	}
	rv := make([]*pb.Node, 0, limit)
	for _, data := range nodes[:limit] {
		rv = append(rv, data.node)
	}
	return rv, nil
}

// Local implements the dht.RoutingTable interface
func (t *Table) Local() pb.Node {
	// the routing table has no idea what the right address of ourself is,
	// so this is the wrong place to get this information. we could return
	// our own id only?
	panic("Unimplementable")
}

// Self returns the node's configured node id.
func (t *Table) Self() storj.NodeID { return t.self }

// MaxBucketDepth returns the largest depth of the routing table tree. This
// is useful for determining which buckets should be refreshed.
func (t *Table) MaxBucketDepth() (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var maxDepth int
	t.makeTree().walkLeaves(func(b *bucket) {
		if b.depth > maxDepth {
			maxDepth = b.depth
		}
	})
	return maxDepth, nil
}

// GetNodes implements the dht.RoutingTable interface
func (t *Table) GetNodes(id storj.NodeID) (nodes []*pb.Node, ok bool) {
	panic("TODO")
}

// GetBucketIds implements the dht.RoutingTable interface
func (t *Table) GetBucketIds() (storage.Keys, error) {
	panic("TODO")
}

// SetBucketTimestamp implements the dht.RoutingTable interface
func (t *Table) SetBucketTimestamp(id []byte, now time.Time) error {
	panic("TODO")
}

// GetBucketTimestamp implements the dht.RoutingTable interface
func (t *Table) GetBucketTimestamp(id []byte) (time.Time, error) {
	panic("TODO")
}

type bucket struct {
	prefix string
	depth  int

	split      bool
	similar    *bucket
	dissimilar *bucket

	nodes []*nodeData
	cache []*nodeData
}

func (b *bucket) walkLeaves(fn func(b *bucket)) {
	if !b.split {
		fn(b)
	} else {
		b.similar.walkLeaves(fn)
		b.dissimilar.walkLeaves(fn)
	}
}

func (t *Table) makeTree() *bucket {
	// to make sure we get the logic right, we're going to reconstruct the
	// routing table binary tree data structure from first principles every time.
	nodes := make([]*nodeData, 0, len(t.nodes))
	for _, node := range t.nodes {
		nodes = append(nodes, node)
	}
	var root bucket

	// we'll replay the nodes in original placement order
	sort.Sort(nodeDataOrderingSorter(nodes))
	nearest := make([]*nodeData, 0, t.bucketSize+1)
	for _, node := range nodes {
		// is the node in the nearest k and therefore should be force-added?
		nearest = append(nearest, node)
		sort.Sort(nodeDataDistanceSorter{self: t.self, nodes: nearest})
		if t.bucketSize < len(nearest) {
			nearest = nearest[:t.bucketSize]
		}
		force := false
		for _, near := range nearest {
			if near.node.Id == node.node.Id {
				force = true
				break
			}
		}

		// add the node
		t.add(&root, node, force, false)
	}
	return &root
}

func (t *Table) add(b *bucket, node *nodeData, force, dissimilar bool) {
	if b.split {
		if bitAtDepth(node.node.Id, b.depth) == bitAtDepth(t.self, b.depth) {
			t.add(b.similar, node, force, dissimilar)
		} else {
			t.add(b.dissimilar, node, force, true)
		}
		return
	}

	if len(b.nodes) < t.bucketSize {
		b.nodes = append(b.nodes, node)
		return
	}

	if dissimilar && !force {
		b.cache = append(b.cache, node)
		return
	}

	b.split = true
	similarBit := bitAtDepth(t.self, b.depth)
	b.similar = &bucket{depth: b.depth + 1, prefix: extendPrefix(b.prefix, similarBit)}
	b.dissimilar = &bucket{depth: b.depth + 1, prefix: extendPrefix(b.prefix, !similarBit)}
	if len(b.cache) > 0 {
		panic("unreachable codepath")
	}
	nodes := b.nodes
	b.nodes = nil
	for _, existingNode := range nodes {
		t.add(b, existingNode, false, dissimilar)
	}
	t.add(b, node, force, dissimilar)
}

func extendPrefix(prefix string, bit bool) string {
	if bit {
		return prefix + "1"
	}
	return prefix + "0"
}
