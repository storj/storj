// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log    *zap.Logger
	config Config
	closed chan struct{}

	queue *queue
}

// NewWorker instantiates Worker and workers.
func NewWorker(log *zap.Logger, config Config) (*Worker, error) {
	return &Worker{
		log:    log,
		config: config,
		closed: make(chan struct{}),

		queue: &queue{},
	}, nil
}

// Run runs audit service 2.0.
func (w *Worker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	w.log.Info("audit 2.0 is starting up")

	var group errgroup.Group
	for i := 0; i < w.config.WorkerCount; i++ {
		group.Go(func() error {
			return w.process(ctx)
		})
	}

	return group.Wait()

}

// Close halts the reservoir chore and audit workers.
func (w *Worker) Close() error {
	close(w.closed)
	return nil
}

// process repeatedly removes an item from the queue and runs an audit.
func (w *Worker) process(ctx context.Context) error {
	ticker := time.NewTicker(w.config.QueueInterval)
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
