// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
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
	// onEmpty is a callback used to swap the active and pending queues when the active queue is empty.
	onEmpty func()
}

// WaitForSwap waits for the active queue to be empty, then replaces it with a new pending queue.
// DO NOT CALL AGAIN UNTIL PREVIOUS CALL HAS RETURNED - there should only ever be one routine that calls WaitForSwap.
// Otherwise, there is a possibility of one call getting stuck until the context is canceled.
func (q *Queue) WaitForSwap(ctx context.Context, newQueue []storj.Path) error {
	q.mu.Lock()
	if q.onEmpty != nil {
		q.mu.Unlock()
		panic("massive internal error, this shouldn't happen")
	}

	if len(q.queue) == 0 {
		q.queue = newQueue
		q.mu.Unlock()
		return nil
	}

	onEmptyCalledChan := make(chan struct{})
	cleanup := func() {
		q.onEmpty = nil
		close(onEmptyCalledChan)
	}
	// onEmpty assumes the mutex is locked when it is called.
	q.onEmpty = func() {
		q.queue = newQueue
		cleanup()
	}
	q.mu.Unlock()

	select {
	case <-onEmptyCalledChan:
	case <-ctx.Done():
		q.mu.Lock()
		defer q.mu.Unlock()

		if q.onEmpty != nil {
			cleanup()
		}
	}
	return ctx.Err()
}

// Next gets the next item in the queue.
func (q *Queue) Next() (storj.Path, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// if the queue is empty, call onEmpty to swap queues (if there is a pending queue)
	// otherwise, return empty queue error
	if len(q.queue) == 0 {
		if q.onEmpty != nil {
			q.onEmpty()
		}
		if len(q.queue) == 0 {
			return "", ErrEmptyQueue.New("")
		}
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
