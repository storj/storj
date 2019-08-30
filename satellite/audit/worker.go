// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
)

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log    *zap.Logger
	config Config

	Limiter *sync2.Limiter

	queue *queue
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, config Config) (*Worker, error) {
	return &Worker{
		log:    log,
		config: config,

		Limiter: sync2.NewLimiter(config.WorkerConcurrency),

		queue: &queue{},
	}, nil
}

// Run runs audit service 2.0.
func (w *Worker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	w.log.Info("audit 2.0 is starting up")

	// wait for all audits to run
	defer w.Limiter.Wait()

	return w.process(ctx)
}

// Close halts the worker.
func (w *Worker) Close() error {
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

		w.Limiter.Go(ctx, func() {
			// TODO: audit the path
		})
	}
}
