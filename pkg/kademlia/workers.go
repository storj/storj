// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

const (
	uncontacted = iota
	inProgress
	completed
)

var (
	// WorkerError is the class of errors for the worker struct
	WorkerError = errs.Class("worker error")
)

type chore struct {
	status int
	node   *proto.Node
}

type worker struct {
	// node id to chore
	workingSet  map[string]*chore
	mu          *sync.Mutex
	maxResponse time.Duration
	cancel      context.CancelFunc
	nodeClient  node.Client
	find        proto.Node
	k           int
}

func (w *worker) work(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		nodes := w.lookup(ctx, w.getWork())
		if nodes == nil {
			continue
		}

		if err := w.update(nodes); err != nil {
			//TODO(coyle): determine best way to handle this error
		}

		return nil
	}

}

// sortByXOR: helper, quick sorts node IDs by xor from local node, smallest xor to largest
func (w *worker) sortByXOR(nodeIDs [][]byte, comp []byte) [][]byte {
	if len(nodeIDs) < 2 {
		return nodeIDs
	}
	left, right := 0, len(nodeIDs)-1
	pivot := rand.Int() % len(nodeIDs)
	nodeIDs[pivot], nodeIDs[right] = nodeIDs[right], nodeIDs[pivot]
	for i := range nodeIDs {
		xorI := xorTwoIds(nodeIDs[i], comp)
		xorR := xorTwoIds(nodeIDs[right], comp)
		if bytes.Compare(xorI, xorR) < 0 {
			nodeIDs[left], nodeIDs[i] = nodeIDs[i], nodeIDs[left]
			left++
		}
	}
	nodeIDs[left], nodeIDs[right] = nodeIDs[right], nodeIDs[left]
	w.sortByXOR(nodeIDs[:left], comp)
	w.sortByXOR(nodeIDs[left+1:], comp)
	return nodeIDs
}

func (w *worker) getWork() *chore {
	var wk *chore
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, v := range w.workingSet {
		if v.status == uncontacted {
			wk = v
			w.workingSet[i].status = inProgress
			break
		}
	}

	if wk == nil {
		w.mu.Unlock()
		time.AfterFunc(2*w.maxResponse, w.cancel)
		return nil
	}

	return wk
}

func (w *worker) lookup(ctx context.Context, wk *chore) []*proto.Node {
	start := time.Now()
	nodes, err := w.nodeClient.Lookup(ctx, *wk.node, w.find)
	if err != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
		delete(w.workingSet, wk.node.GetId())
		return nil
	}
	latency := time.Now().Sub(start)
	if latency > w.maxResponse {
		w.maxResponse = latency
	}

	return nodes
}

func (w *worker) update(nodes []*proto.Node) error {
	if nodes == nil {
		return WorkerError.New("nodes must not be nil")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if (len(w.workingSet) + len(nodes)) < w.k {
		for _, v := range nodes {
			w.workingSet[v.GetId()] = &chore{node: v, status: uncontacted}
		}
		return nil
	}

	// sort all nodes from map
	mapNodes := make([][]byte, len(w.workingSet))
	ii := 0
	for i := range w.workingSet {
		mapNodes[ii] = []byte(i)
		ii++
	}

	// sort all nodes from map
	lookupNodes := make([][]byte, len(nodes))
	for i, v := range nodes {
		lookupNodes[i] = []byte(v.GetId())
	}

	self := []byte(w.find.GetId())
	smapNodes := w.sortByXOR(mapNodes, self)
	// sort all nodes returned from lookup
	slookupNodes := w.sortByXOR(lookupNodes, self)
	// update the nearest-k data structure with the results of the request, bumping nodes out of the data structure that are farther than k.
	ll := 0
	for i := len(smapNodes) - 1; i >= 0; {
		if bytes.Compare(smapNodes[i], slookupNodes[ll]) < 0 {
			i--
			ll++
		} else {
			delete(w.workingSet, string(smapNodes[i]))
			w.workingSet[string(slookupNodes[ll])] = &chore{node: nodes[ll], status: uncontacted}
			i--
			ll++
		}
	}

	return nil
}
