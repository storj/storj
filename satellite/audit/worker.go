// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
)

// Worker contains information for populating audit queue and processing audits.
type Worker struct {
	log      *zap.Logger
	queue    *Queue
	Verifier *Verifier
	Reporter reporter
	Loop     sync2.Cycle
	limiter  sync2.Limiter
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, queue *Queue, metainfo *metainfo.Service,
	orders *orders.Service, transport transport.Client, overlay *overlay.Service,
	containment Containment, identity *identity.FullIdentity, config Config) (*Worker, error) {
	return &Worker{
		log: log,

		queue:    queue,
		Verifier: NewVerifier(log.Named("audit:verifier"), metainfo, transport, overlay, containment, orders, identity, config.MinBytesPerSecond, config.MinDownloadTimeout),
		Reporter: NewReporter(log.Named("audit:reporter"), overlay, containment, config.MaxRetriesStatDB, int32(config.MaxReverifyCount)),
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
	var errlist errs.Group
	report, err := worker.Verifier.Reverify2(ctx, path)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = worker.Reporter.RecordAudits(ctx, report)
	if err != nil {
		errlist.Add(err)
	}

	// Skip all reverified nodes in the next Verify step.
	skip := make(map[storj.NodeID]bool)
	if report != nil {
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
	}

	report, err = worker.Verifier.Verify2(ctx, path, skip)
	if err != nil {
		errlist.Add(err)
	}

	return errlist.Err()
}
