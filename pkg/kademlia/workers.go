// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"sync"

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
	contacted map[string]*chore
	mu        *sync.Mutex
}

//     as long as there are uncontacted nodes in the nearest node data structure,
//     mark the next closest node as in progress, do a lookup, and then get results
//     mark the node as complete
//     update the nearest-k data structure with the results of the request, bumping nodes out of the data structure that are farther than k.
//     start the loop over.
//     as soon as a goroutine discovers that all of the nodes in the list were either contacted or in progress, start a timer, 2x the time of the max response time so far.
//     at the end of the timer cancel all remaining lookups

func (w *worker) work(ctx context.Context, jobs <-chan *proto.Node) {
	for {
		select {
		case <-ctx.Done():
			return
			// case j := <-jobs:

		}

	}

}
