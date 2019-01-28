// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sort"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type peerDiscovery struct {
	log *zap.Logger

	dialer *Dialer
	self   pb.Node
	target storj.NodeID
	opts   discoveryOptions

	cond  sync.Cond
	queue discoveryQueue
}

// ErrMaxRetries is used when a lookup has been retried the max number of times
var ErrMaxRetries = errs.Class("max retries exceeded for id:")

func newPeerDiscovery(log *zap.Logger, self pb.Node, nodes []*pb.Node, dialer *Dialer, target storj.NodeID, opts discoveryOptions) *peerDiscovery {
	discovery := &peerDiscovery{
		log:    log,
		dialer: dialer,
		self:   self,
		target: target,
		opts:   opts,
		cond:   sync.Cond{L: &sync.Mutex{}},
		queue:  *newDiscoveryQueue(opts.concurrency),
	}
	discovery.queue.Insert(target, nodes...)
	return discovery
}

func (lookup *peerDiscovery) Run(ctx context.Context) (target *pb.Node, err error) {
	if lookup.queue.Len() == 0 {
		return nil, nil // TODO: should we return an error here?
	}

	// protected by `lookup.cond.L`
	working := 0
	allDone := false
	target = nil

	wg := sync.WaitGroup{}
	wg.Add(lookup.opts.concurrency)
	defer wg.Wait()

	for i := 0; i < lookup.opts.concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				var next *pb.Node

				lookup.cond.L.Lock()
				for {
					// everything is done, this routine can return
					if allDone {
						lookup.cond.L.Unlock()
						return
					}

					next = lookup.queue.Closest()

					if !lookup.opts.bootstrap && next != nil && next.Id == lookup.target {
						allDone = true
						target = next
						break // closest node is the target and is already in routing table (i.e. no lookup required)
					}

					if next != nil {
						working++
						break
					}
					// no work, wait until some other routine inserts into the queue
					lookup.cond.Wait()
				}
				lookup.cond.L.Unlock()
				var nodeType pb.NodeType
				if target != nil {
					nodeType = target.Type
					nodeType.DPanicOnInvalid("Peer Discovery Run")
				}
				next.Type.DPanicOnInvalid("next")
				neighbors, err := lookup.dialer.Lookup(ctx, lookup.self, *next, pb.Node{Id: lookup.target, Type: nodeType})

				if err != nil && !isDone(ctx) {
					// TODO: reenable retry after fixing logic
					// ok := lookup.queue.Reinsert(lookup.target, next, lookup.opts.retries)
					ok := false
					if !ok {
						lookup.log.Debug("connecting to node failed",
							zap.Any("target", lookup.target),
							zap.Any("dial", next.Id),
							zap.Any("dial-address", next.Address.Address),
							zap.Error(err),
						)
					}
				}

				lookup.queue.Insert(lookup.target, neighbors...)

				lookup.cond.L.Lock()
				working--
				allDone = allDone || isDone(ctx) || working == 0 && lookup.queue.Len() == 0
				lookup.cond.L.Unlock()
				lookup.cond.Broadcast()
			}
		}()
	}

	err = ctx.Err()
	if err == context.Canceled {
		err = nil
	}
	return target, err
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// discoveryQueue is a limited priority queue for nodes with xor distance
type discoveryQueue struct {
	maxLen int
	mu     sync.Mutex
	added  map[storj.NodeID]int
	items  []queueItem
}

// queueItem is node with a priority
type queueItem struct {
	node     *pb.Node
	priority storj.NodeID
}

// newDiscoveryQueue returns a items with priority based on XOR from targetBytes
func newDiscoveryQueue(size int) *discoveryQueue {
	return &discoveryQueue{
		added:  make(map[storj.NodeID]int),
		maxLen: size,
	}
}

// Insert adds nodes into the queue.
func (queue *discoveryQueue) Insert(target storj.NodeID, nodes ...*pb.Node) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	unique := nodes[:0]
	for _, node := range nodes {
		if _, added := queue.added[node.Id]; added {
			continue
		}
		unique = append(unique, node)
	}

	queue.insert(target, unique...)

	// update counts for the new items that are in the queue
	for _, item := range queue.items {
		if _, added := queue.added[item.node.Id]; !added {
			queue.added[item.node.Id] = 1
		}
	}
}

// Reinsert adds a Nodes into the queue, only if it's has been added less than limit times.
func (queue *discoveryQueue) Reinsert(target storj.NodeID, node *pb.Node, limit int) bool {
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
func (queue *discoveryQueue) insert(target storj.NodeID, nodes ...*pb.Node) {
	for _, node := range nodes {
		queue.items = append(queue.items, queueItem{
			node:     node,
			priority: xorNodeID(target, node.Id),
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
func (queue *discoveryQueue) Closest() *pb.Node {
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
func (queue *discoveryQueue) Len() int {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	return len(queue.items)
}
