// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase"
)

// Error is the default audit errs class.
var Error = errs.Class("audit")

// Config contains configurable values for audit chore and workers.
type Config struct {
	NodeFilter         string        `help:"restrict audit only to the filtered nodes" default:""`
	MaxRetriesStatDB   int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	MinBytesPerSecond  memory.Size   `help:"the minimum acceptable bytes that storage nodes can transfer per second to the satellite" default:"150kB" testDefault:"1.00 KB"`
	MinDownloadTimeout time.Duration `help:"the minimum duration for downloading a share from storage nodes before timing out" default:"15s" testDefault:"5s"`
	MaxReverifyCount   int           `help:"limit above which we consider an audit is failed" default:"3"`

	QueueInterval             time.Duration `help:"how often to recheck an empty audit queue" releaseDefault:"1h" devDefault:"1m" testDefault:"$TESTINTERVAL"`
	Slots                     int           `help:"number of reservoir slots allotted for nodes, currently capped at 3" default:"3"`
	VerificationPushBatchSize int           `help:"number of audit jobs to push at once to the verification queue" devDefault:"10" releaseDefault:"4096"`
	WorkerConcurrency         int           `help:"number of workers to run audits on segments" default:"2"`
	UseRangedLoop             bool          `help:"whether use Audit observer with ranged loop." default:"true"`

	ReverifyWorkerConcurrency   int           `help:"number of workers to run reverify audits on pieces" default:"2"`
	ReverificationRetryInterval time.Duration `help:"how long a single reverification job can take before it may be taken over by another worker" releaseDefault:"6h" devDefault:"10m"`

	ContainmentSyncChoreInterval time.Duration `help:"how often to run the containment-sync chore" releaseDefault:"2h" devDefault:"2m" testDefault:"$TESTINTERVAL"`
}

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log           *zap.Logger
	queue         VerifyQueue
	verifier      *Verifier
	reverifyQueue ReverifyQueue
	reporter      Reporter
	Loop          *sync2.Cycle
	concurrency   int
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, queue VerifyQueue, verifier *Verifier, reverifyQueue ReverifyQueue, reporter Reporter, config Config) *Worker {
	return &Worker{
		log: log,

		queue:         queue,
		verifier:      verifier,
		reverifyQueue: reverifyQueue,
		reporter:      reporter,
		Loop:          sync2.NewCycle(config.QueueInterval),
		concurrency:   config.WorkerConcurrency,
	}
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

	limiter := sync2.NewLimiter(worker.concurrency)
	defer limiter.Wait()

	for {
		segment, err := worker.queue.Next(ctx)
		if err != nil {
			if ErrEmptyQueue.Has(err) {
				return nil
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

	// First, remove nodes that are contained. We do not (currently)
	// audit contained nodes for other pieces until we get an answer
	// for the contained audit. (I suspect this could change without
	// upsetting anything, but for now it's best to keep it the way
	// it was. -thepaul)
	skip, err := worker.verifier.IdentifyContainedNodes(ctx, segment)
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			// no need to add this error; Verify() will encounter it again
			// and will handle the verification job as appropriate.
			err = nil
		} else {
			errlist.Add(err)
		}
	}

	// Next, audit the remaining nodes that are not in containment mode.
	report, err := worker.verifier.Verify(ctx, segment, skip)
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			// no need to add this error; Verify() will encounter it again
			// and will handle the verification job as appropriate.
			err = nil
		} else {
			errlist.Add(err)
		}
	}

	worker.reporter.RecordAudits(ctx, report)

	return errlist.Err()
}
