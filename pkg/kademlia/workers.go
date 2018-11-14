// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
)

var (
	// WorkerError is the class of errors for the worker struct
	WorkerError = errs.Class("worker error")
	// default timeout is the minimum timeout for worker cancellation
	// 250ms was the minimum value allowing current workers to finish work
	// before returning
	defaultTimeout = 250 * time.Millisecond
)

// worker pops work off a priority queue and does lookups on the work received
type worker struct {
	contacted      map[string]bool
	pq             *XorQueue
	mu             *sync.Mutex
	maxResponse    time.Duration
	cancel         context.CancelFunc
	nodeClient     node.Client
	find           dht.NodeID
	workInProgress int
	k              int
}

func newWorker(ctx context.Context, rt *RoutingTable, nodes []*pb.Node, nc node.Client, target dht.NodeID, k int) *worker {
	pq := NewXorQueue(k)
	pq.Insert(target, nodes)
	return &worker{
		contacted:      map[string]bool{},
		pq:             pq,
		mu:             &sync.Mutex{},
		maxResponse:    0 * time.Millisecond,
		nodeClient:     nc,
		find:           target,
		workInProgress: 0,
		k:              k,
	}
}

// create x workers
// have a worker that gets work off the queue
// send that work on a channel
// have workers get work available off channel
// after queue is empty and no work is in progress, close channel.

func (w *worker) work(ctx context.Context, ch chan *pb.Node) {
	// grab uncontacted node from working set
	// change status to inprogress
	// ask node for target
	// if node has target cancel ctx and send node
	for {
		select {
		case <-ctx.Done():
			return
		case n := <-ch:
			// network lookup for nodes
			nodes := w.lookup(ctx, n)
			// update our priority queue
			w.update(nodes)
		}
	}
}

func (w *worker) getWork(ctx context.Context, ch chan *pb.Node) {
	for {
		if ctx.Err() != nil {
			return
		}

		w.mu.Lock()
		if w.pq.Len() <= 0 && w.workInProgress <= 0 {
			w.mu.Unlock()
			timeout := defaultTimeout
			if timeout < (2 * w.maxResponse) {
				timeout = 2 * w.maxResponse
			}

			time.AfterFunc(timeout, w.cancel)
			return
		}

		if w.pq.Len() <= 0 {
			w.mu.Unlock()
			// if there is nothing left to get off the queue
			// and the work-in-progress is not empty
			// let's wait a bit for the workers to populate the queue
			time.Sleep(50 * time.Millisecond)
			continue
		}

		w.workInProgress++
		node, _ := w.pq.Closest()
		ch <- node
		w.mu.Unlock()
	}

}

func (w *worker) lookup(ctx context.Context, node *pb.Node) []*pb.Node {
	start := time.Now()
	if node.GetAddress() == nil {
		return nil
	}

	nodes, err := w.nodeClient.Lookup(ctx, *node, pb.Node{Id: w.find.String()})
	if err != nil {
		// TODO(coyle): I think we might want to do another look up on this node or update something
		// but for now let's just log and ignore.
		log.Printf("Error occurred during lookup for %s on %s :: error = %s", w.find.String(), node.GetId(), err.Error())
		return []*pb.Node{}
	}

	// add node to the previously contacted list so we don't duplicate lookups
	w.mu.Lock()
	w.contacted[node.GetId()] = true
	w.mu.Unlock()

	latency := time.Since(start)
	if latency > w.maxResponse {
		w.maxResponse = latency
	}

	return nodes
}

func (w *worker) update(nodes []*pb.Node) {
	w.mu.Lock()
	defer w.mu.Unlock()

	uncontactedNodes := []*pb.Node{}
	for _, v := range nodes {
		// if we have already done a lookup on this node we don't want to do it again for this lookup loop
		if !w.contacted[v.GetId()] {
			uncontactedNodes = append(uncontactedNodes, v)
		}
	}
	w.pq.Insert(w.find, uncontactedNodes)
	w.workInProgress--
}

// SetCancellation adds the cancel function to the worker
func (w *worker) SetCancellation(cf context.CancelFunc) {
	w.cancel = cf
}
