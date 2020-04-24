// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion

import (
	"context"
	"sync"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// Handler handles piece deletion requests from a queue.
type Handler interface {
	// Handle should call queue.PopAll until finished.
	Handle(ctx context.Context, node *pb.Node, queue Queue)
}

// NewQueue is a constructor func for queues.
type NewQueue func() Queue

// Queue is a queue for jobs.
type Queue interface {
	// TryPush tries to push a new job to the queue.
	TryPush(job Job) bool

	// PopAll fetches all jobs in the queue.
	//
	// When there are no more jobs, the queue must stop accepting new jobs.
	PopAll() ([]Job, bool)

	// PopAllWithoutClose fetches all jobs in the queue,
	// but without closing the queue for new requests.
	PopAllWithoutClose() []Job
}

// Job is a single of deletion.
type Job struct {
	// Pieces are the pieces id-s that need to be deleted.
	Pieces []storj.PieceID
	// Resolve is for notifying the job issuer about the outcome.
	Resolve Promise
}

// Promise is for signaling to the deletion requests about the result.
type Promise interface {
	// Success is called when the job has been successfully handled.
	Success()
	// Failure is called when the job didn't complete successfully.
	Failure()
}

// Combiner combines multiple concurrent deletion requests into batches.
type Combiner struct {
	// ctx context to pass down to the handler.
	ctx    context.Context
	cancel context.CancelFunc

	// handler defines what to do with the jobs.
	handler Handler
	// newQueue creates a new queue.
	newQueue NewQueue
	// workers contains all worker goroutines.
	workers sync2.WorkGroup

	// mu protects workerByID
	mu         sync.Mutex
	workerByID map[storj.NodeID]*worker
}

// worker handles a batch of jobs.
type worker struct {
	waitFor chan struct{}
	node    *pb.Node
	jobs    Queue
	done    chan struct{}
}

// NewCombiner creates a new combiner.
func NewCombiner(parent context.Context, handler Handler, newQueue NewQueue) *Combiner {
	ctx, cancel := context.WithCancel(parent)
	return &Combiner{
		ctx:        ctx,
		cancel:     cancel,
		handler:    handler,
		newQueue:   newQueue,
		workerByID: map[storj.NodeID]*worker{},
	}
}

// Close shuts down all workers.
func (combiner *Combiner) Close() {
	combiner.cancel()
	combiner.workers.Close()
}

// Enqueue adds a deletion job to the queue.
func (combiner *Combiner) Enqueue(node *pb.Node, job Job) {
	combiner.mu.Lock()
	defer combiner.mu.Unlock()

	last := combiner.workerByID[node.Id]

	// Check whether we can use the last worker.
	if last != nil && last.jobs.TryPush(job) {
		// We've successfully added a job to an existing worker.
		return
	}

	// Create a new worker when one doesn't exist or the last one was full.
	next := &worker{
		node: node,
		jobs: combiner.newQueue(),
		done: make(chan struct{}),
	}
	if last != nil {
		next.waitFor = last.done
	}
	combiner.workerByID[node.Id] = next
	if !next.jobs.TryPush(job) {
		// This should never happen.
		job.Resolve.Failure()
	}

	// Start the worker.
	next.start(combiner)
}

// schedule starts the worker.
func (worker *worker) start(combiner *Combiner) {
	// Try to add to worker pool, this may fail when we are shutting things down.
	workerStarted := combiner.workers.Go(func() {
		defer close(worker.done)
		// Ensure we fail any jobs that the handler didn't handle.
		defer FailPending(worker.jobs)

		if worker.waitFor != nil {
			// Wait for previous worker to finish work to ensure fairness between nodes.
			select {
			case <-worker.waitFor:
			case <-combiner.ctx.Done():
				return
			}
		}

		// Handle the job queue.
		combiner.handler.Handle(combiner.ctx, worker.node, worker.jobs)
	})

	// If we failed to start a worker, then mark all the jobs as failures.
	if !workerStarted {
		FailPending(worker.jobs)
	}
}

// FailPending fails all the jobs in the queue.
func FailPending(jobs Queue) {
	for {
		list, ok := jobs.PopAll()
		if !ok {
			return
		}

		for _, job := range list {
			job.Resolve.Failure()
		}
	}
}
