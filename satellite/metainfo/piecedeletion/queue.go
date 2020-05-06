// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion

import "sync"

// LimitedJobs is a finalizable list of deletion jobs with a limit to how many
// jobs it can handle.
type LimitedJobs struct {
	maxPiecesPerBatch int

	mu sync.Mutex
	// done indicates that no more items will be appended to the queue.
	done bool
	// count is the number of piece ids queued here.
	count int
	// list is the list of delete jobs.
	list []Job
}

// NewLimitedJobs returns a new limited job queue.
func NewLimitedJobs(maxPiecesPerBatch int) *LimitedJobs {
	return &LimitedJobs{
		maxPiecesPerBatch: maxPiecesPerBatch,
	}
}

// TryPush tries to add a job to the queue.
//
// maxPiecesPerBatch < 0, means no limit
func (jobs *LimitedJobs) TryPush(job Job) bool {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	// check whether we have finished work with this jobs queue.
	if jobs.done {
		return false
	}

	// add to the queue, this can potentially overflow `maxPiecesPerBatch`,
	// however splitting a single request and promise across multiple batches, is annoying.
	jobs.count += len(job.Pieces)

	// check whether the queue is at capacity
	if jobs.maxPiecesPerBatch >= 0 && jobs.count >= jobs.maxPiecesPerBatch {
		jobs.done = true
	}

	jobs.list = append(jobs.list, job)
	return true
}

// PopAll returns all the jobs in this list.
func (jobs *LimitedJobs) PopAll() (_ []Job, ok bool) {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	// when we try to pop and the queue is empty, make the queue final.
	if len(jobs.list) == 0 {
		jobs.done = true
		return nil, false
	}

	list := jobs.list
	jobs.list = nil
	return list, true
}

// PopAllWithoutClose returns all the jobs in this list without closing the queue.
func (jobs *LimitedJobs) PopAllWithoutClose() []Job {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()

	list := jobs.list
	jobs.list = nil
	return list
}
