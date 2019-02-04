// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/statdb"
	"storj.io/storj/pkg/transport"
)

// Config contains configurable values for audit service
type Config struct {
	MaxRetriesStatDB int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	Interval         time.Duration `help:"how frequently segments are audited" default:"30s"`
}

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	log *zap.Logger

	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter

	ticker *time.Ticker
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(log *zap.Logger, sdb statdb.DB, interval time.Duration, maxRetries int, pointers *pointerdb.Service, allocation *pointerdb.AllocationSigner, transport transport.Client, overlay *overlay.Cache, identity *identity.FullIdentity) (service *Service, err error) {
	return &Service{
		log: log,
		// TODO: instead of overlay.Client use overlay.Service
		Cursor:   NewCursor(pointers, allocation, identity),
		Verifier: NewVerifier(transport, overlay, identity),
		Reporter: NewReporter(sdb, maxRetries),

		ticker: time.NewTicker(interval),
	}, nil
}

// Run runs auditing service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("Audit cron is starting up")
	for {
		err := service.process(ctx)
		if err != nil {
			service.log.Error("process", zap.Error(err))
		}

		select {
		case <-service.ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
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

	verifiedNodes, err := service.Verifier.verify(ctx, stripe)
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
