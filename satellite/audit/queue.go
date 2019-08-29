// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"sync"
	"time"

	"storj.io/storj/pkg/storj"
)

// queue is a list of paths to audit, shared between the reservoir chore and audit workers.
type queue struct {
	cond         sync.Cond
	queue        []storj.Path
	closed       chan struct{}
	pollInterval time.Duration
}

func newQueue(interval time.Duration) *queue {
	return &queue{
		cond:         *sync.NewCond(&sync.Mutex{}),
		closed:       make(chan struct{}),
		pollInterval: interval,
	}
}

// swap switches the backing queue slice with a new queue slice.
func (queue *queue) swap(newQueue []storj.Path) {
	queue.cond.L.Lock()
	queue.queue = newQueue
	// Notify workers that queue has been repopulated.
	queue.cond.Broadcast()
	queue.cond.L.Unlock()
}

// next gets the next item in the queue.
func (queue *queue) next(ctx context.Context) (storj.Path, error) {
	ticker := time.NewTicker(queue.pollInterval)
	defer ticker.Stop()

	// This waits until the queue is repopulated, closed, or context is canceled.
	for len(queue.queue) == 0 {
		select {
		case <-queue.closed:
			return "", Error.New("queue is closed")
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}

	queue.cond.L.Lock()
	defer queue.cond.L.Unlock()

	next := queue.queue[0]
	queue.queue = queue.queue[1:]

	return next, nil
}

func (queue *queue) close() {
	close(queue.closed)
	// Wake up workers that are waiting.
	queue.cond.Broadcast()
}
