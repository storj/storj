// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"context"
	"math/rand"
	"sync"
	"time"

	"storj.io/storj/pkg/node"
	proto "storj.io/storj/protos/overlay"
)

const (
	uncontacted = iota
	inProgress
	completed
)

type chore struct {
	status int
	node   *proto.Node
}

type worker struct {
	// node id to chore
	contacted   map[string]*chore
	mu          *sync.Mutex
	maxResponse time.Duration
	cancel      context.CancelFunc
	nodeClient  node.Client
	find        proto.Node
	k           int
}

func (w *worker) work(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			w.mu.Lock()
			var wk *chore
			for i, v := range w.contacted {
				if v.status == uncontacted {
					wk = v
					w.contacted[i].status = inProgress
					w.mu.Unlock()
					break
				}
			}

			if wk == nil {
				w.mu.Unlock()
				time.AfterFunc(2*w.maxResponse, w.cancel)
				return nil
			}

			start := time.Now()
			nodes, err := w.nodeClient.Lookup(ctx, *wk.node, w.find)
			if err != nil {
				return err
			}
			latency := time.Now().Sub(start)
			if latency > w.maxResponse {
				w.maxResponse = latency
			}

			w.mu.Lock()
			if (len(w.contacted) + len(nodes)) < w.k {
				for _, v := range nodes {
					w.contacted[v.GetId()] = &chore{node: v, status: uncontacted}
				}
				w.mu.Unlock()
			}

			// sort all nodes from map
			mapNodes := make([][]byte, len(w.contacted))
			ii := 0
			for i := range w.contacted {
				mapNodes[ii] = []byte(i)
				ii++
			}

			// sort all nodes from map
			lookupNodes := make([][]byte, len(nodes))
			for i, v := range nodes {
				lookupNodes[i] = []byte(v.GetId())
			}

			self := []byte(w.find.GetId())
			smapNodes := sortByXOR(mapNodes, self)
			// sort all nodes returned from lookup
			slookupNodes := sortByXOR(lookupNodes, self)
			// update the nearest-k data structure with the results of the request, bumping nodes out of the data structure that are farther than k.
			ll := 0
			for i := len(smapNodes) - 1; i >= 0; {
				if bytes.Compare(smapNodes[i], slookupNodes[ll]) < 0 {
					i--
					ll++
				} else {
					delete(w.contacted, string(smapNodes[i]))
					w.contacted[string(slookupNodes[ll])] = &chore{node: nodes[ll], status: uncontacted}
					i--
					ll++
				}
			}

			w.mu.Unlock()
		}

	}

}

// sortByXOR: helper, quick sorts node IDs by xor from local node, smallest xor to largest
func sortByXOR(nodeIDs [][]byte, comp []byte) [][]byte {
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
	sortByXOR(nodeIDs[:left], comp)
	sortByXOR(nodeIDs[left+1:], comp)
	return nodeIDs
}
