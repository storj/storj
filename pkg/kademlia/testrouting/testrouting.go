// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrouting

import (
	"context"
	"sort"
	"sync"
	"time"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/storage"
)

var (
	mon = monkit.Package()
)

type nodeData struct {
	node        *pb.Node
	ordering    int64
	lastUpdated time.Time
	fails       int
	inCache     bool
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
	splits  map[string]bool
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
		splits:          map[string]bool{},
	}
}

// K returns the Table's routing depth, or Kademlia k value
func (t *Table) K() int { return t.bucketSize }

// CacheSize returns the size of replacement cache
func (t *Table) CacheSize() int { return t.cacheSize }

// ConnectionSuccess should be called whenever a node is successfully connected
// to. It will add or update the node's entry in the routing table.
func (t *Table) ConnectionSuccess(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	t.mu.Lock()
	defer t.mu.Unlock()

	// don't add ourselves
	if node.Id == t.self {
		return nil
	}

	// if the node is already here, update it
	if cell, exists := t.nodes[node.Id]; exists {
		cell.node = node
		cell.lastUpdated = time.Now()
		cell.fails = 0
		// skip placement order and cache status
		return nil
	}

	// add unconditionally (it might be going into a replacement cache)
	t.nodes[node.Id] = &nodeData{
		node:        node,
		ordering:    t.counter,
		lastUpdated: time.Now(),
		fails:       0,

		// makeTree within preserveInvariants might promote this to true
		inCache: false,
	}
	t.counter++

	t.preserveInvariants()
	return nil
}

// ConnectionFailed should be called whenever a node can't be contacted.
// If a node fails more than allowedFailures times, it will be removed from
// the routing table. The failure count is reset every successful connection.
func (t *Table) ConnectionFailed(ctx context.Context, node *pb.Node) (err error) {
	defer mon.Task()(&ctx)(&err)
	t.mu.Lock()
	defer t.mu.Unlock()

	// if the node exists and the failure is with the address we have, record
	// a failure

	if data, exists := t.nodes[node.Id]; exists &&
		pb.AddressEqual(data.node.Address, node.Address) {
		data.fails++ //TODO: we may not need this
		// if we've failed too many times, remove the node
		if data.fails > t.allowedFailures {
			delete(t.nodes, node.Id)

			t.preserveInvariants()
		}
	}
	return nil
}

// FindNear will return up to limit nodes in the routing table ordered by
// kademlia xor distance from the given id.
func (t *Table) FindNear(ctx context.Context, id storj.NodeID, limit int) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	t.mu.Lock()
	defer t.mu.Unlock()

	// find all non-cache nodes
	nodes := make([]*nodeData, 0, len(t.nodes))
	for _, node := range t.nodes {
		if !node.inCache {
			nodes = append(nodes, node)
		}
	}

	// sort by distance
	sort.Sort(nodeDataDistanceSorter{self: id, nodes: nodes})

	// return up to limit nodes
	if limit > len(nodes) {
		limit = len(nodes)
	}
	rv := make([]*pb.Node, 0, limit)
	for _, data := range nodes[:limit] {
		rv = append(rv, data.node)
	}
	return rv, nil
}

// Local returns the local node
func (t *Table) Local() overlay.NodeDossier {
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
	t.walkLeaves(t.makeTree(), func(b *bucket) {
		if b.depth > maxDepth {
			maxDepth = b.depth
		}
	})
	return maxDepth, nil
}

// GetNodes retrieves nodes within the same kbucket as the given node id
func (t *Table) GetNodes(id storj.NodeID) (nodes []*pb.Node, ok bool) {
	panic("TODO")
}

// GetBucketIds returns a storage.Keys type of bucket ID's in the Kademlia instance
func (t *Table) GetBucketIds(context.Context) (storage.Keys, error) {
	panic("TODO")
}

// SetBucketTimestamp records the time of the last node lookup for a bucket
func (t *Table) SetBucketTimestamp(context.Context, []byte, time.Time) error {
	panic("TODO")
}

// GetBucketTimestamp retrieves time of the last node lookup for a bucket
func (t *Table) GetBucketTimestamp(context.Context, []byte) (time.Time, error) {
	panic("TODO")
}

func (t *Table) preserveInvariants() {
	t.walkLeaves(t.makeTree(), func(b *bucket) {
		// pull the latest nodes out of the replacement caches for incomplete
		// buckets
		for len(b.cache) > 0 && len(b.nodes) < t.bucketSize {
			recentNode := b.cache[len(b.cache)-1]
			recentNode.inCache = false
			b.cache = b.cache[:len(b.cache)-1]
			b.nodes = append(b.nodes, recentNode)
		}

		// prune remaining replacement cache entries
		if len(b.cache) > t.cacheSize {
			for _, node := range b.cache[:len(b.cache)-t.cacheSize] {
				delete(t.nodes, node.node.Id)
			}
		}
	})
}

type bucket struct {
	prefix string
	depth  int

	similar    *bucket
	dissimilar *bucket

	nodes []*nodeData
	cache []*nodeData
}

func (t *Table) walkLeaves(b *bucket, fn func(b *bucket)) {
	if !t.splits[b.prefix] {
		fn(b)
	} else if b.similar != nil {
		t.walkLeaves(b.similar, fn)
		t.walkLeaves(b.dissimilar, fn)
	}
}

func (t *Table) makeTree() *bucket {
	// to make sure we get the logic right, we're going to reconstruct the
	// routing table binary tree data structure every time.
	nodes := make([]*nodeData, 0, len(t.nodes))
	for _, node := range t.nodes {
		nodes = append(nodes, node)
	}
	var root bucket

	// we'll replay the nodes in original placement order
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ordering < nodes[j].ordering
	})
	nearest := make([]*nodeData, 0, t.bucketSize+1)
	for _, node := range nodes {
		// keep track of the nearest k nodes
		nearest = append(nearest, node)
		sort.Sort(nodeDataDistanceSorter{self: t.self, nodes: nearest})
		if len(nearest) > t.bucketSize {
			nearest = nearest[:t.bucketSize]
		}

		t.add(&root, node, false, nearest)
	}
	return &root
}

func (t *Table) add(b *bucket, node *nodeData, dissimilar bool, nearest []*nodeData) {
	if t.splits[b.prefix] {
		if b.similar == nil {
			similarBit := bitAtDepth(t.self, b.depth)
			b.similar = &bucket{depth: b.depth + 1, prefix: extendPrefix(b.prefix, similarBit)}
			b.dissimilar = &bucket{depth: b.depth + 1, prefix: extendPrefix(b.prefix, !similarBit)}
		}
		if bitAtDepth(node.node.Id, b.depth) == bitAtDepth(t.self, b.depth) {
			t.add(b.similar, node, dissimilar, nearest)
		} else {
			t.add(b.dissimilar, node, true, nearest)
		}
		return
	}

	if node.inCache {
		b.cache = append(b.cache, node)
		return
	}

	if len(b.nodes) < t.bucketSize {
		node.inCache = false
		b.nodes = append(b.nodes, node)
		return
	}

	if dissimilar && !isNearest(node.node.Id, nearest) {
		node.inCache = true
		b.cache = append(b.cache, node)
		return
	}

	t.splits[b.prefix] = true
	if len(b.cache) > 0 {
		panic("unreachable codepath")
	}
	nodes := b.nodes
	b.nodes = nil
	for _, existingNode := range nodes {
		t.add(b, existingNode, dissimilar, nearest)
	}
	t.add(b, node, dissimilar, nearest)
}

// Close closes without closing dependencies
func (t *Table) Close() error { return nil }
