// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// ReverifyWorker processes reverifications (retrying piece audits against nodes that timed out
// during a Verification).
type ReverifyWorker struct {
	log        *zap.Logger
	queue      ReverifyQueue
	reverifier *Reverifier
	reporter   Reporter

	Loop          *sync2.Cycle
	concurrency   int
	retryInterval time.Duration
}

// NewReverifyWorker creates a new ReverifyWorker.
func NewReverifyWorker(log *zap.Logger, queue ReverifyQueue, reverifier *Reverifier, reporter Reporter, config Config) *ReverifyWorker {
	return &ReverifyWorker{
		log:           log,
		queue:         queue,
		reverifier:    reverifier,
		reporter:      reporter,
		Loop:          sync2.NewCycle(config.QueueInterval),
		concurrency:   config.ReverifyWorkerConcurrency,
		retryInterval: config.ReverificationRetryInterval,
	}
}

// Run runs a ReverifyWorker.
func (worker *ReverifyWorker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return worker.Loop.Run(ctx, func(ctx context.Context) (err error) {
		err = worker.process(ctx)
		if err != nil {
			worker.log.Error("failure processing reverify queue", zap.Error(Error.Wrap(err)))
		}
		return nil
	})
}

func (worker *ReverifyWorker) process(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	limiter := sync2.NewLimiter(worker.concurrency)
	defer limiter.Wait()

	for {
		// We start the timeout clock _before_ pulling the next job from
		// the queue. This gives us the best chance of having this worker
		// terminate and get cleaned up before another reverification
		// worker tries to take over the job.
		//
		// (If another worker does take over the job before this worker
		// has been cleaned up, it is ok; the downside should only be
		// duplication of work and monkit stats.)
		ctx, cancel := context.WithTimeout(ctx, worker.retryInterval)

		reverifyJob, err := worker.queue.GetNextJob(ctx, worker.retryInterval)
		if err != nil {
			cancel()
			if ErrEmptyQueue.Has(err) {
				return nil
			}
			return err
		}

		started := limiter.Go(ctx, func() {
			defer cancel()

			logger := worker.log.With(
				zap.Stringer("Segment StreamID", reverifyJob.Locator.StreamID),
				zap.Uint64("Segment Position", reverifyJob.Locator.Position.Encode()),
				zap.Stringer("Node ID", reverifyJob.Locator.NodeID),
				zap.Int("Piece Number", reverifyJob.Locator.PieceNum))
			worker.work(ctx, logger, reverifyJob)
		})
		if !started {
			cancel()
			return ctx.Err()
		}
	}
}

func (worker *ReverifyWorker) work(ctx context.Context, logger *zap.Logger, job *ReverificationJob) {
	defer mon.Task()(&ctx)(nil)

	logger.Debug("beginning piecewise audit")
	outcome, reputation := worker.reverifier.ReverifyPiece(ctx, logger, &job.Locator)
	logger.Debug("piecewise audit complete", zap.Int("outcome", int(outcome)))

	err := worker.reporter.RecordReverificationResult(ctx, job, outcome, reputation)
	if err != nil {
		logger.Error("finished with audit, but failed to remove entry from queue", zap.Error(err))
	}
}

// Close halts the worker.
func (worker *ReverifyWorker) Close() error {
	worker.Loop.Close()
	return nil
}
