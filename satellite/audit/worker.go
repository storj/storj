// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/sync2"
)

// Error is the default audit errs class.
var Error = errs.Class("audit")

// Config contains configurable values for audit chore and workers.
type Config struct {
	MaxRetriesStatDB   int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	MinBytesPerSecond  memory.Size   `help:"the minimum acceptable bytes that storage nodes can transfer per second to the satellite" default:"128B" testDefault:"1.00 KB"`
	MinDownloadTimeout time.Duration `help:"the minimum duration for downloading a share from storage nodes before timing out" default:"5m0s" testDefault:"5s"`
	MaxReverifyCount   int           `help:"limit above which we consider an audit is failed" default:"3"`

	ChoreInterval     time.Duration `help:"how often to run the reservoir chore" releaseDefault:"24h" devDefault:"1m" testDefault:"$TESTINTERVAL"`
	QueueInterval     time.Duration `help:"how often to recheck an empty audit queue" releaseDefault:"1h" devDefault:"1m" testDefault:"$TESTINTERVAL"`
	Slots             int           `help:"number of reservoir slots allotted for nodes, currently capped at 3" default:"3"`
	WorkerConcurrency int           `help:"number of workers to run audits on segments" default:"2"`
}

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log         *zap.Logger
	queues      *Queues
	verifier    *Verifier
	reporter    Reporter
	Loop        *sync2.Cycle
	concurrency int
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, queues *Queues, verifier *Verifier, reporter Reporter, config Config) (*Worker, error) {
	return &Worker{
		log: log,

		queues:      queues,
		verifier:    verifier,
		reporter:    reporter,
		Loop:        sync2.NewCycle(config.QueueInterval),
		concurrency: config.WorkerConcurrency,
	}, nil
}

// Run runs audit service 2.0.
func (worker *Worker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

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
	worker.Loop.Close()
	return nil
}

// process repeatedly removes an item from the queue and runs an audit.
func (worker *Worker) process(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// get the current queue
	queue := worker.queues.Fetch()

	limiter := sync2.NewLimiter(worker.concurrency)
	defer limiter.Wait()

	for {
		segment, err := queue.Next()
		if err != nil {
			if ErrEmptyQueue.Has(err) {
				// get a new queue and return if empty; otherwise continue working.
				queue = worker.queues.Fetch()
				if queue.Size() == 0 {
					return nil
				}
				continue
			}
			return err
		}

		started := limiter.Go(ctx, func() {
			err := worker.work(ctx, segment)
			if err != nil {
				worker.log.Error("error(s) during audit",
					zap.String("Segment StreamID", segment.StreamID.String()),
					zap.Uint64("Segment Position", segment.Position.Encode()),
					zap.Error(err))
			}
		})
		if !started {
			return ctx.Err()
		}
	}
}

func (worker *Worker) work(ctx context.Context, segment Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	var errlist errs.Group

	// First, attempt to reverify nodes for this segment that are in containment mode.
	report, err := worker.verifier.Reverify(ctx, segment)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = worker.reporter.RecordAudits(ctx, report)
	if err != nil {
		errlist.Add(err)
	}

	// Skip all reverified nodes in the next Verify step.
	skip := make(map[storj.NodeID]bool)
	for _, nodeID := range report.Successes {
		skip[nodeID] = true
	}
	for _, nodeID := range report.Offlines {
		skip[nodeID] = true
	}
	for _, nodeID := range report.Fails {
		skip[nodeID] = true
	}
	for _, pending := range report.PendingAudits {
		skip[pending.NodeID] = true
	}
	for _, nodeID := range report.Unknown {
		skip[nodeID] = true
	}

	// Next, audit the the remaining nodes that are not in containment mode.
	report, err = worker.verifier.Verify(ctx, segment, skip)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = worker.reporter.RecordAudits(ctx, report)
	if err != nil {
		errlist.Add(err)
	}

	return errlist.Err()
}
