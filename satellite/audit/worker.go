// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/storj"
)

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log     *zap.Logger
	queue   *Queue
	Loop    sync2.Cycle
	limiter sync2.Limiter
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, queue *Queue, config Config) (*Worker, error) {
	return &Worker{
		log: log,

		queue:   queue,
		Loop:    *sync2.NewCycle(config.QueueInterval),
		limiter: *sync2.NewLimiter(config.WorkerConcurrency),
	}, nil
}

// Run runs audit service 2.0.
func (worker *Worker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	worker.log.Debug("starting")

	// Wait for all audits to run.
	defer worker.limiter.Wait()

	return worker.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)
		err = worker.process(ctx)
		if err != nil {
			worker.log.Error("process", zap.Error(Error.Wrap(err)))
		}
		return nil
	})
}

// Close halts the worker.
func (worker *Worker) Close() error {
	return nil
}

// process repeatedly removes an item from the queue and runs an audit.
func (worker *Worker) process(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	worker.limiter.Wait()
	for {
		path, err := worker.queue.Next()
		if err != nil {
			if ErrEmptyQueue.Has(err) {
				return nil
			}
			return err
		}

		worker.limiter.Go(ctx, func() {
			err := worker.work(ctx, path)
			if err != nil {
				worker.log.Error("audit failed", zap.Error(err))
			}
		})
	}
}

func (worker *Worker) work(ctx context.Context, path storj.Path) error {
	// TODO: handle work.
	return nil
}
