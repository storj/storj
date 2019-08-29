// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

// ErrEmptyQueue is used to indicate that the queue is empty
var ErrEmptyQueue = errs.Class("empty audit queue")

// queue is a list of paths to audit, shared between the reservoir chore and audit workers.
type queue struct {
	mu    sync.Mutex
	queue []storj.Path
}

// swap switches the backing queue slice with a new queue slice.
func (q *queue) swap(newQueue []storj.Path) {
	q.mu.Lock()
	q.queue = newQueue
	q.mu.Unlock()
}

// next gets the next item in the queue.
func (q *queue) next() (storj.Path, error) {
	// return error if queue is empty
	if len(q.queue) == 0 {
		return "", ErrEmptyQueue.New("")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	next := q.queue[0]
	q.queue = q.queue[1:]

	return next, nil
}
