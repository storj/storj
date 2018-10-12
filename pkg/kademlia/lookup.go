package kademlia

import (
	"container/heap"
	"context"
	"log"
	"math/big"
	"time"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
)

type sequentialLookup struct {
	contacted       map[string]bool
	queue           PriorityQueue
	slowestResponse time.Duration
	client          node.Client
	target          dht.NodeID
	limit           int
	bootstrap       bool
}

func newSequentialLookup(rt *RoutingTable, nodes []*pb.Node, client node.Client, target dht.NodeID, limit int, bootstrap bool) *sequentialLookup {
	targetBytes := new(big.Int).SetBytes(target.Bytes())

	var queue PriorityQueue
	{
		for i, node := range nodes {
			bnode := new(big.Int).SetBytes([]byte(node.GetId()))
			queue = append(queue, &Item{
				value:    node,
				priority: new(big.Int).Xor(targetBytes, bnode),
				index:    i,
			})
		}
		heap.Init(&queue)
	}

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
	zero := &big.Int{}
	targetBytes := new(big.Int).SetBytes(lookup.target.Bytes())

	for len(lookup.queue) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		item := heap.Pop(&lookup.queue).(*Item)
		if !lookup.bootstrap && item.priority.Cmp(zero) == 0 {
			// found the result
			return nil
		}
		next := item.value

		neighbors := lookup.FetchNeighbors(ctx, next)
		for _, neighbor := range neighbors {
			if lookup.contacted[neighbor.GetId()] {
				continue
			}

			priority := new(big.Int).Xor(targetBytes, new(big.Int).SetBytes(lookup.target.Bytes()))
			heap.Push(&lookup.queue, &Item{
				value:    neighbor,
				priority: priority,
			})
		}

		for len(lookup.queue) > lookup.limit {
			heap.Pop(&lookup.queue)
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
