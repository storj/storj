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
var Error = errs.Class("audit error")

// Config contains configurable values for audit chore and workers.
type Config struct {
	MaxRetriesStatDB   int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	MinBytesPerSecond  memory.Size   `help:"the minimum acceptable bytes that storage nodes can transfer per second to the satellite" default:"128B"`
	MinDownloadTimeout time.Duration `help:"the minimum duration for downloading a share from storage nodes before timing out" default:"5m0s"`
	MaxReverifyCount   int           `help:"limit above which we consider an audit is failed" default:"3"`

	ChoreInterval     time.Duration `help:"how often to run the reservoir chore" releaseDefault:"24h" devDefault:"1m"`
	QueueInterval     time.Duration `help:"how often to recheck an empty audit queue" releaseDefault:"1h" devDefault:"1m"`
	Slots             int           `help:"number of reservoir slots allotted for nodes, currently capped at 3" default:"3"`
	WorkerConcurrency int           `help:"number of workers to run audits on paths" default:"1"`
}

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log      *zap.Logger
	queue    *Queue
	verifier *Verifier
	reporter *Reporter
	Loop     sync2.Cycle
	limiter  sync2.Limiter
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, queue *Queue, verifier *Verifier, reporter *Reporter, config Config) (*Worker, error) {
	return &Worker{
		log: log,

		queue:    queue,
		verifier: verifier,
		reporter: reporter,
		Loop:     *sync2.NewCycle(config.QueueInterval),
		limiter:  *sync2.NewLimiter(config.WorkerConcurrency),
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
	worker.Loop.Close()
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
				worker.log.Error("audit failed", zap.Binary("Segment", []byte(path)), zap.Error(err))
			}
		})
	}
}

func (worker *Worker) work(ctx context.Context, path storj.Path) (err error) {
	defer mon.Task()(&ctx)(&err)

	var errlist errs.Group

	// First, attempt to reverify nodes for this segment that are in containment mode.
	report, err := worker.verifier.Reverify(ctx, path)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = worker.reporter.RecordAudits(ctx, report, path)
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
	report, err = worker.verifier.Verify(ctx, path, skip)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = worker.reporter.RecordAudits(ctx, report, path)
	if err != nil {
		errlist.Add(err)
	}

	return errlist.Err()
}
