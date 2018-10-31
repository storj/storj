// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"math/big"
	"sync"

	"go.uber.org/zap"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
)

type peerDiscovery struct {
	client node.Client
	target dht.NodeID
	opts   discoveryOptions

	cond      sync.Cond
	queue     *XorQueue
	contacted map[string]int
}

func newPeerDiscovery(nodes []*pb.Node, client node.Client, target dht.NodeID, opts discoveryOptions) *peerDiscovery {
	queue := NewXorQueue(opts.concurrency)
	queue.Insert(target, nodes)

	return &peerDiscovery{
		client: client,
		target: target,
		opts:   opts,

		cond:      sync.Cond{L: &sync.Mutex{}},
		queue:     queue,
		contacted: map[string]int{},
	}
}

func (lookup *peerDiscovery) Run(ctx context.Context) error {
	wg := sync.WaitGroup{}

	// protected by `lookup.cond.L`
	working := 0
	allDone := false

	wg.Add(lookup.opts.concurrency)
	for i := 0; i < lookup.opts.concurrency; i++ {
		go func() {
			defer wg.Done()
			for {
				var (
					next     *pb.Node
					priority big.Int
				)

				lookup.cond.L.Lock()
				for {
					// everything is done, this routine can return
					if allDone {
						lookup.cond.L.Unlock()
						return
					}

					next, priority = lookup.queue.Closest()
					if !lookup.opts.bootstrap && priority.Cmp(&big.Int{}) == 0 {
						allDone = true
						lookup.cond.L.Unlock()
						return // closest node is the target and is already in routing table (i.e. no lookup required)
					}

					if next != nil {
						working++
						break
					}

					// no work, wait until some other routine inserts into the queue
					lookup.cond.Wait()
				}

				nextID := next.GetId()
				lookup.contacted[nextID]++
				tries := lookup.contacted[nextID]
				lookup.cond.L.Unlock()

				neighbors, err := lookup.client.Lookup(ctx, *next, pb.Node{Id: lookup.target.String()})
				if err != nil {
					if tries < lookup.opts.retries {
						neighbors = append(neighbors, next)
					} else {
						zap.S().Errorf("Error occurred during lookup for %s on %s :: error = %s", lookup.target.String(), next.GetId(), err.Error())
					}
				}

				lookup.cond.L.Lock()
				var toContact []*pb.Node
				for _, neighbor := range neighbors {
					if lookup.contacted[neighbor.GetId()] == 0 {
						toContact = append(toContact, neighbor)
					}
				}
				lookup.queue.UniqueInsert(lookup.target, toContact)

				working--
				allDone = isDone(ctx) || working == 0 && lookup.queue.Len() == 0
				lookup.cond.L.Unlock()
				lookup.cond.Broadcast()
			}
		}()
	}

	wg.Wait()
	return ctx.Err()
}

func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
