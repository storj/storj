// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// ErrEmptyQueue is used to indicate that the queue is empty.
var ErrEmptyQueue = errs.Class("empty audit queue")

// Queue is a list of paths to audit, shared between the reservoir chore and audit workers.
type Queue struct {
	mu    sync.Mutex
	queue []storj.Path
}

// Swap switches the backing queue slice with a new queue slice.
func (q *Queue) Swap(newQueue []storj.Path) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = newQueue
}

// Next gets the next item in the queue.
func (q *Queue) Next() (storj.Path, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// return error if queue is empty
	if len(q.queue) == 0 {
		return "", ErrEmptyQueue.New("")
	}

	next := q.queue[0]
	q.queue = q.queue[1:]

	return next, nil
}

// Size returns the size of the queue.
func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.queue)
}
