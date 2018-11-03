// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
)

type peerDiscovery struct {
	client node.Client
	target dht.NodeID
	opts   discoveryOptions

	cond  sync.Cond
	queue *XorQueue
}

// ErrMaxRetries is used when a lookup has been retried the max number of times
var ErrMaxRetries = errs.Class("max retries exceeded for id:")

func newPeerDiscovery(nodes []*pb.Node, client node.Client, target dht.NodeID, opts discoveryOptions) *peerDiscovery {
	queue := NewXorQueue(opts.concurrency)
	queue.Insert(target, nodes)

	return &peerDiscovery{
		client: client,
		target: target,
		opts:   opts,

		cond:  sync.Cond{L: &sync.Mutex{}},
		queue: queue,
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
					next *pb.Node
				)

				lookup.cond.L.Lock()
				for {
					// everything is done, this routine can return
					if allDone {
						lookup.cond.L.Unlock()
						return
					}

					next, _ = lookup.queue.Closest()
					if !lookup.opts.bootstrap && next.GetId() == lookup.target.String() {
						allDone = true
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

				neighbors, err := lookup.client.Lookup(ctx, *next, pb.Node{Id: lookup.target.String()})
				if err != nil {
					ok := lookup.queue.Reinsert(lookup.target, next, lookup.opts.retries)
					if !ok {
						zap.S().Errorf(
							"Error occurred during lookup of %s :: %s :: error = %s",
							lookup.target.String(),
							ErrMaxRetries.New("%s", next.GetId()),
							err.Error(),
						)
					}
				}

				lookup.queue.Insert(lookup.target, neighbors)

				lookup.cond.L.Lock()
				working--
				allDone = allDone || isDone(ctx) || working == 0 && lookup.queue.Len() == 0
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
