// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

// ErrEmptyQueue is used to indicate that the queue is empty
var ErrEmptyQueue = errs.Class("empty audit queue")

// queue is a list of paths to audit, shared between the reservoir chore and audit workers.
type queue struct {
	mu     sync.Mutex
	queue  []storj.Path
	closed chan struct{}
}

func newQueue() *queue {
	return &queue{
		closed: make(chan struct{}),
	}
}

// swap switches the backing queue slice with a new queue slice.
func (q *queue) swap(newQueue []storj.Path) {
	q.mu.Lock()
	q.queue = newQueue
	q.mu.Unlock()
}

// next gets the next item in the queue.
func (q *queue) next(ctx context.Context) (storj.Path, error) {
	// return error if context canceled or queue closed
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-q.closed:
		return "", Error.New("queue is closed")
	default:
	}

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

func (q *queue) close() {
	close(q.closed)
}
