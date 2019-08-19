// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
)

// Error is the default audit errs class
var Error = errs.Class("audit error")

// Config contains configurable values for audit service
type Config struct {
	MaxRetriesStatDB   int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	Interval           time.Duration `help:"how frequently segments are audited" default:"30s"`
	MinBytesPerSecond  memory.Size   `help:"the minimum acceptable bytes that storage nodes can transfer per second to the satellite" default:"128B"`
	MinDownloadTimeout time.Duration `help:"the minimum duration for downloading a share from storage nodes before timing out" default:"25s"`
	MaxReverifyCount   int           `help:"limit above which we consider an audit is failed" default:"3"`

	Slots int `help:"number of reservoir slots allotted for nodes" default:"1"`
}

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	log *zap.Logger

	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter

	Loop sync2.Cycle
}

// Service2 is a temp name for the service struct during the audit 2.0 refactor.
// Once V3-2363 and V3-2364 are implemented, Service2 will replace the existing Service struct.
type Service2 struct {
	log *zap.Logger

	reservoirSlots int
	Reservoirs     map[storj.NodeID]*Reservoir

	MetainfoLoop  *metainfo.Loop
	ReservoirLoop sync2.Cycle
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(log *zap.Logger, config Config, metainfo *metainfo.Service,
	orders *orders.Service, transport transport.Client, overlay *overlay.Service,
	containment Containment, identity *identity.FullIdentity, metaLoop *metainfo.Loop) (*Service, error) {
	return &Service{
		log: log,

		Cursor:   NewCursor(metainfo),
		Verifier: NewVerifier(log.Named("audit:verifier"), metainfo, transport, overlay, containment, orders, identity, config.MinBytesPerSecond, config.MinDownloadTimeout),
		Reporter: NewReporter(log.Named("audit:reporter"), overlay, containment, config.MaxRetriesStatDB, int32(config.MaxReverifyCount)),

		Loop: *sync2.NewCycle(config.Interval),
	}, nil
}

// NewService2 instantiates Service2
func NewService2(log *zap.Logger, metaLoop *metainfo.Loop, config Config) (*Service2, error) {
	return &Service2{
		log: log,

		reservoirSlots: config.Slots,
		Reservoirs:     make(map[storj.NodeID]*Reservoir),

		MetainfoLoop:  metaLoop,
		ReservoirLoop: *sync2.NewCycle(config.Interval),
	}, nil
}

// Run runs auditing service 2.0
func (service *Service2) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("audit 2.0 is starting up")

	group, ctx := errgroup.WithContext(ctx)

	service.ReservoirLoop.Start(ctx, group, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)
		pathCollector := NewPathCollector(service.reservoirSlots)
		err = service.MetainfoLoop.Join(ctx, pathCollector)
		if err != nil {
			service.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}
		for nodeID, res := range pathCollector.Reservoirs {
			service.Reservoirs[nodeID] = res
		}
		return nil
	})

	return group.Wait()
}

// Run runs auditing service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("Audit cron is starting up")

	group, ctx := errgroup.WithContext(ctx)

	service.Loop.Start(ctx, group, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)
		err = service.process(ctx)
		if err != nil {
			service.log.Error("process", zap.Error(err))
		}
		return nil
	})

	return group.Wait()
}

// Close halts the audit loop
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}

// process picks a random stripe and verifies correctness
func (service *Service) process(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	var stripe *Stripe
	for {
		s, more, err := service.Cursor.NextStripe(ctx)
		if err != nil {
			return err
		}
		if s != nil {
			stripe = s
			break
		}
		if !more {
			return nil
		}
	}

	var errlist errs.Group

	report, err := service.Verifier.Reverify(ctx, stripe)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = service.Reporter.RecordAudits(ctx, report)
	if err != nil {
		errlist.Add(err)
	}

	// skip all reverified nodes in the next Verify step
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

	report, err = service.Verifier.Verify(ctx, stripe, skip)
	if err != nil {
		errlist.Add(err)
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = service.Reporter.RecordAudits(ctx, report)
	if err != nil {
		errlist.Add(err)
	}

	return errlist.Err()
}
