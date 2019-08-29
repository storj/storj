// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Service2 contains information for populating audit queue and processing audits.
type Service2 struct {
	log    *zap.Logger
	closed chan struct{}

	workers []*worker
	queue   *queue
}

// NewService2 instantiates Service2 and workers.
func NewService2(log *zap.Logger, config Config) (*Service2, error) {
	queue := &queue{}
	var workers []*worker
	for i := 0; i < config.WorkerCount; i++ {
		workers = append(workers, newWorker(queue, config.QueueInterval))
	}
	return &Service2{
		log:    log,
		closed: make(chan struct{}),

		workers: workers,
		queue:   queue,
	}, nil
}

// Run runs audit service 2.0.
func (service *Service2) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("audit 2.0 is starting up")

	var group errgroup.Group
	for _, worker := range service.workers {
		group.Go(func() error {
			return worker.run(ctx)
		})
	}

	return group.Wait()

}

// Close halts the reservoir chore and audit workers.
func (service *Service2) Close() error {
	close(service.closed)
	return nil
}

// worker processes items on the audit queue.
type worker struct {
	pollInterval time.Duration
	queue        *queue
}

// newWorker instantiates a worker.
func newWorker(queue *queue, pollInterval time.Duration) *worker {
	return &worker{
		pollInterval: pollInterval,
		queue:        queue,
	}
}

// worker removes an item from the queue and runs an audit.
func (w *worker) run(ctx context.Context) error {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := w.queue.next()
		if err != nil && ErrEmptyQueue.Has(err) {
			// wait for next poll interval or until close
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		} else if err != nil {
			return err
		}

		// TODO: audit the path
	}
}
