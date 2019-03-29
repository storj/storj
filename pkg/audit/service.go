// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/orders"
)

// Config contains configurable values for audit service
type Config struct {
	MaxRetriesStatDB  int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	Interval          time.Duration `help:"how frequently segments are audited" default:"30s"`
	MinBytesPerSecond memory.Size   `help:"the minimum acceptable bytes that storage nodes can transfer per second to the satellite" default:"128B"`
}

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	log *zap.Logger

	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter

	Loop sync2.Cycle
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(log *zap.Logger, config Config, pointerdb *pointerdb.Service,
	allocation *pointerdb.AllocationSigner, orders *orders.Service, transport transport.Client, overlay *overlay.Cache,
	identity *identity.FullIdentity) (service *Service, err error) {
	return &Service{
		log: log,

		Cursor:   NewCursor(pointerdb),
		Verifier: NewVerifier(log.Named("audit:verifier"), transport, overlay, orders, identity, config.MinBytesPerSecond),
		Reporter: NewReporter(overlay, config.MaxRetriesStatDB),

		Loop: *sync2.NewCycle(config.Interval),
	}, nil
}

// Run runs auditing service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("Audit cron is starting up")

	return service.Loop.Run(ctx, func(ctx context.Context) error {
		err := service.process(ctx)
		if err != nil {
			service.log.Error("process", zap.Error(err))
		}
		return err
	})
}

// Close halts the audit loop
func (service *Service) Close() error {
	service.Loop.Close()
	return nil
}

// process picks a random stripe and verifies correctness
func (service *Service) process(ctx context.Context) error {
	stripe, err := service.Cursor.NextStripe(ctx)
	if err != nil {
		return err
	}
	if stripe == nil {
		return nil
	}

	verifiedNodes, err := service.Verifier.Verify(ctx, stripe)
	if err != nil {
		return err
	}

	// TODO(moby) we need to decide if we want to do something with nodes that the reporter failed to update
	_, err = service.Reporter.RecordAudits(ctx, verifiedNodes)
	if err != nil {
		return err
	}

	return nil
}
