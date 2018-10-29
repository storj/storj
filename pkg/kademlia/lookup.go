// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
	"log"
	"time"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
)

type sequentialLookup struct {
	contacted       map[string]bool
	queue           *XorQueue
	slowestResponse time.Duration
	client          node.Client
	target          dht.NodeID
	limit           int
	bootstrap       bool
}

func newSequentialLookup(rt *RoutingTable, nodes []*pb.Node, client node.Client, target dht.NodeID, limit int, bootstrap bool) *sequentialLookup {
	queue := NewXorQueue(limit)
	queue.Insert(target, nodes)

	return &sequentialLookup{
		contacted:       map[string]bool{},
		queue:           queue,
		slowestResponse: 0,
		client:          client,
		target:          target,
		limit:           limit,
		bootstrap:       bootstrap,
	}
}

func (lookup *sequentialLookup) Run(ctx context.Context) error {
	for lookup.queue.Len() > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		next, priority := lookup.queue.Closest()
		if !lookup.bootstrap && bytes.Equal(priority.Bytes(), make([]byte, len(priority.Bytes()))) {
			return nil // found the result
		}

		uncontactedNeighbors := []*pb.Node{}
		neighbors := lookup.FetchNeighbors(ctx, next)
		for _, neighbor := range neighbors {
			if !lookup.contacted[neighbor.GetId()] {
				uncontactedNeighbors = append(uncontactedNeighbors, neighbor)
			}
		}
		lookup.queue.Insert(lookup.target, uncontactedNeighbors)

		for lookup.queue.Len() > lookup.limit {
			lookup.queue.Closest()
		}
	}
	return nil
}

func (lookup *sequentialLookup) FetchNeighbors(ctx context.Context, node *pb.Node) []*pb.Node {
	if node.GetAddress() == nil {
		return nil
	}
	lookup.contacted[node.GetId()] = true

	start := time.Now()
	neighbors, err := lookup.client.Lookup(ctx, *node, pb.Node{Id: lookup.target.String()})
	if err != nil {
		// TODO(coyle): I think we might want to do another look up on this node or update something
		// but for now let's just log and ignore.
		log.Printf("Error occurred during lookup for %s on %s :: error = %s", lookup.target.String(), node.GetId(), err.Error())
		return []*pb.Node{}
	}

	latency := time.Since(start)
	if latency > lookup.slowestResponse {
		lookup.slowestResponse = latency
	}

	return neighbors
}
