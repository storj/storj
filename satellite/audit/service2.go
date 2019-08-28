// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/satellite/metainfo"
)

// Service2 contains information for populating audit queue and processing audits.
type Service2 struct {
	log *zap.Logger

	reservoirChore *ReservoirChore
	workers        []*worker
	queue          *queue
}

// NewService2 instantiates Service2, ReservoirChore and workers.
func NewService2(log *zap.Logger, config Config, metaloop *metainfo.Loop) (*Service2, error) {
	queue := newQueue()
	var workers []*worker
	for i := 0; i < config.WorkerCount; i++ {
		workers = append(workers, newWorker(queue))
	}
	return &Service2{
		log: log,

		reservoirChore: NewReservoirChore(log.Named("reservoir chore"), queue, metaloop, config),
		workers:        workers,
		queue:          queue,
	}, nil
}

// Run runs audit service 2.0.
func (service *Service2) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("audit 2.0 is starting up")

	var group errgroup.Group
	group.Go(func() error {
		return service.reservoirChore.populateQueueJob(ctx)
	})

	for _, worker := range service.workers {
		group.Go(func() error {
			return worker.run(ctx)
		})
	}

	return group.Wait()

}

// Close halts the reservoir chore and audit workers.
func (service *Service2) Close() error {
	service.queue.cond.L.Lock()
	service.queue.close()
	service.queue.cond.L.Unlock()
	return nil
}

// worker processes items on the audit queue.
type worker struct {
	queue *queue
}

// newWorker instantiates a worker.
func newWorker(queue *queue) *worker {
	return &worker{
		queue: queue,
	}
}

// worker removes an item from the queue and runs an audit.
func (w *worker) run(ctx context.Context) error {
	for {
		_, err := w.queue.next(ctx)
		if err != nil {
			return err
		}
		// TODO: audit the path
	}
}
