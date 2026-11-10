// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/zeebo/errs"
)

// readPriority orders waiting piece reads. Higher values are served first.
type readPriority int

const (
	// readPriorityRepair is the first read of a repair download.
	readPriorityRepair readPriority = 0
	// readPriorityDefault is the first read of a customer download.
	readPriorityDefault readPriority = 1
	// readPriorityAudit is the first read of an audit download. Audits are rare
	// and small, but an audit that waits past the satellite timeout hurts the
	// node's reputation, so they go before new customer downloads.
	readPriorityAudit readPriority = 2
	// readPriorityContinue is any later read of a download that already read
	// from disk. Finishing it is cheaper than starting a new download, and if
	// it timed out the data it already read would be wasted.
	readPriorityContinue readPriority = 3

	numReadPriorities = 4
)

// errReadQueueTimeout is returned when a repair read waited too long for a slot.
var errReadQueueTimeout = errs.Class("read queue timeout")

// readQueue limits how many piece reads run at the same time.
//
// A disk read can't be stopped once it is in the kernel, so on a slow disk
// canceled downloads still use disk time. Reads waiting here leave the queue
// when their client cancels, and never reach the disk.
//
// Reads of downloads that already started go first, oldest first, so a download
// that has read part of a piece can finish. Audits go next, oldest first, so a
// steady stream of new downloads can't starve them. For new customer and repair
// downloads the newest waiting read is served first, because on an overloaded
// disk the oldest waiters are the most likely to be canceled already. New
// customer downloads go before new repair downloads.
type readQueue struct {
	limit      int
	repairWait time.Duration

	// releaseFn is release, built once so Acquire doesn't allocate it per read.
	releaseFn func()

	mu     sync.Mutex
	active int
	// waiting holds a ready channel per waiter, by priority, newest last.
	// release closes the channel to hand over the slot.
	waiting [numReadPriorities][]chan struct{}
}

// newReadQueue returns a queue that allows limit concurrent reads. Repair reads
// that wait longer than repairWait fail; zero means they wait until canceled.
func newReadQueue(limit int, repairWait time.Duration) *readQueue {
	q := &readQueue{limit: limit, repairWait: repairWait}
	q.releaseFn = q.release
	return q
}

// Acquire waits for a read slot. Call release after the read.
func (q *readQueue) Acquire(ctx context.Context, priority readPriority) (release func(), err error) {
	q.mu.Lock()
	if q.active < q.limit {
		q.active++
		q.mu.Unlock()
		return q.releaseFn, nil
	}
	ready := make(chan struct{})
	q.waiting[priority] = append(q.waiting[priority], ready)
	q.mu.Unlock()

	defer mon.Task()(&ctx)(&err)

	var timeout <-chan time.Time
	if priority == readPriorityRepair && q.repairWait > 0 {
		timer := time.NewTimer(q.repairWait)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-ready:
		return q.releaseFn, nil
	case <-ctx.Done():
		err = ctx.Err()
	case <-timeout:
		err = errReadQueueTimeout.New("waited %v", q.repairWait)
	}

	q.mu.Lock()
	var granted bool
	select {
	case <-ready:
		granted = true
	default:
		q.waiting[priority] = slices.DeleteFunc(q.waiting[priority], func(x chan struct{}) bool { return x == ready })
	}
	q.mu.Unlock()
	if granted {
		// we got the slot while giving up, pass it on.
		q.release()
	}
	return nil, err
}

// release hands the slot to the next waiter, or frees it when nobody waits.
func (q *readQueue) release() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for priority := readPriority(numReadPriorities - 1); priority >= readPriorityRepair; priority-- {
		waiting := q.waiting[priority]
		if len(waiting) == 0 {
			continue
		}
		var ready chan struct{}
		if priority == readPriorityContinue || priority == readPriorityAudit {
			ready = waiting[0]
			waiting[0] = nil
			q.waiting[priority] = waiting[1:]
		} else {
			ready = waiting[len(waiting)-1]
			q.waiting[priority] = waiting[:len(waiting)-1]
		}
		close(ready)
		return
	}
	q.active--
}
