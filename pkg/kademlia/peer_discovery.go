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

	dialer      *Dialer
	self        *pb.Node
	target      storj.NodeID
	k           int
	concurrency int

	cond  sync.Cond
	queue discoveryQueue
}

// ErrMaxRetries is used when a lookup has been retried the max number of times
var ErrMaxRetries = errs.Class("max retries exceeded for id:")

func newPeerDiscovery(log *zap.Logger, dialer *Dialer, target storj.NodeID, startingNodes []*pb.Node, k, alpha int, self *pb.Node) *peerDiscovery {
	discovery := &peerDiscovery{
		log:         log,
		dialer:      dialer,
		self:        self,
		target:      target,
		k:           k,
		concurrency: alpha,
		cond:        sync.Cond{L: &sync.Mutex{}},
		queue:       *newDiscoveryQueue(target, k),
	}
	discovery.queue.Insert(startingNodes...)
	return discovery
}

func (lookup *peerDiscovery) Run(ctx context.Context) (_ []*pb.Node, err error) {
	defer mon.Task()(&ctx)(&err)
	if lookup.queue.Unqueried() == 0 {
		return nil, nil
	}

	// protected by `lookup.cond.L`
	working := 0
	allDone := false

	wg := sync.WaitGroup{}
	wg.Add(lookup.concurrency)

	for i := 0; i < lookup.concurrency; i++ {
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

					next = lookup.queue.ClosestUnqueried()

					if next != nil {
						working++
						break
					}
					// no work, wait until some other routine inserts into the queue
					lookup.cond.Wait()
				}
				lookup.cond.L.Unlock()

				neighbors, err := lookup.dialer.Lookup(ctx, lookup.self, *next, lookup.target, lookup.k)
				if err != nil {
					lookup.queue.QueryFailure(next)
					if !isDone(ctx) {
						lookup.log.Debug("connecting to node failed",
							zap.Any("target", lookup.target),
							zap.Any("dial-node", next.Id),
							zap.Any("dial-address", next.Address.Address),
							zap.Error(err),
						)
					}
				} else {
					lookup.queue.QuerySuccess(next, neighbors...)
				}

				lookup.cond.L.Lock()
				working--
				allDone = allDone || isDone(ctx) || (working == 0 && lookup.queue.Unqueried() == 0)
				lookup.cond.L.Unlock()
				lookup.cond.Broadcast()
			}
		}()
	}

	wg.Wait()

	return lookup.queue.ClosestQueried(), ctx.Err()
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

type queueState int

const (
	stateUnqueried queueState = iota
	stateQuerying
	stateSuccess
	stateFailure
)

// discoveryQueue is a limited priority queue for nodes with xor distance
type discoveryQueue struct {
	target storj.NodeID
	maxLen int
	mu     sync.Mutex
	state  map[storj.NodeID]queueState
	items  []queueItem
}

// queueItem is node with a priority
type queueItem struct {
	node     *pb.Node
	priority storj.NodeID
}

// newDiscoveryQueue returns a items with priority based on XOR from targetBytes
func newDiscoveryQueue(target storj.NodeID, size int) *discoveryQueue {
	return &discoveryQueue{
		target: target,
		state:  make(map[storj.NodeID]queueState),
		maxLen: size,
	}
}

// Insert adds nodes into the queue.
func (queue *discoveryQueue) Insert(nodes ...*pb.Node) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.insert(nodes...)
}

// insert requires the mutex to be locked
func (queue *discoveryQueue) insert(nodes ...*pb.Node) {
	for _, node := range nodes {
		// TODO: empty node ids should be semantically different from the
		// technically valid node id that is all zeros
		if node.Id == (storj.NodeID{}) {
			continue
		}
		if _, added := queue.state[node.Id]; added {
			continue
		}
		queue.state[node.Id] = stateUnqueried

		queue.items = append(queue.items, queueItem{
			node:     node,
			priority: xorNodeID(queue.target, node.Id),
		})
	}

	sort.Slice(queue.items, func(i, k int) bool {
		return queue.items[i].priority.Less(queue.items[k].priority)
	})

	if len(queue.items) > queue.maxLen {
		queue.items = queue.items[:queue.maxLen]
	}
}

// ClosestUnqueried returns the closest unqueried item in the queue
func (queue *discoveryQueue) ClosestUnqueried() *pb.Node {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	for _, item := range queue.items {
		if queue.state[item.node.Id] == stateUnqueried {
			queue.state[item.node.Id] = stateQuerying
			return item.node
		}
	}

	return nil
}

// ClosestQueried returns the closest queried items in the queue
func (queue *discoveryQueue) ClosestQueried() []*pb.Node {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	rv := make([]*pb.Node, 0, len(queue.items))
	for _, item := range queue.items {
		if queue.state[item.node.Id] == stateSuccess {
			rv = append(rv, item.node)
		}
	}

	return rv
}

// QuerySuccess marks the node as successfully queried, and adds the results to the queue
// QuerySuccess marks nodes with a zero node ID as ignored, and ignores incoming
// nodes with a zero id.
func (queue *discoveryQueue) QuerySuccess(node *pb.Node, nodes ...*pb.Node) {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.state[node.Id] = stateSuccess
	queue.insert(nodes...)
}

// QueryFailure marks the node as failing query
func (queue *discoveryQueue) QueryFailure(node *pb.Node) {
	queue.mu.Lock()
	queue.state[node.Id] = stateFailure
	queue.mu.Unlock()
}

// Unqueried returns the number of unqueried items in the queue
func (queue *discoveryQueue) Unqueried() (amount int) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	for _, item := range queue.items {
		if queue.state[item.node.Id] == stateUnqueried {
			amount++
		}
	}
	return amount
}
