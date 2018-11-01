// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter
	ticker   *time.Ticker
}

// Config contains configurable values for audit service
type Config struct {
	StatDBPort       string                `help:"port to contact statDB client" default:":9090"`
	MaxRetriesStatDB int                   `help:"max number of times to attempt updating a statdb batch" default:"3"`
	Pointers         pdbclient.Client      `help:"Pointers for a instantiation of a new service"`
	Transport        transport.Client      `help:"Transport for a instantiation of a new service"`
	Overlay          overlay.Client        `help:"Overlay for a instantiation of a new service"`
	ID               provider.FullIdentity `help:"ID for a instantiation of a new service"`
	Interval         time.Duration         `help:"how frequently segements should audited" default:"30s"`
}

// Run runs the repairer with the configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	service, err := NewService(ctx, c.StatDBPort, c.Interval, c.MaxRetriesStatDB, c.Pointers, c.Transport, c.Overlay, c.ID)
	if err != nil {
		return err
	}
	return service.Run(ctx)
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(ctx context.Context, statDBPort string, interval time.Duration, maxRetries int, pointers pdbclient.Client, transport transport.Client, overlay overlay.Client,
	id provider.FullIdentity) (service *Service, err error) {
	cursor := NewCursor(pointers)
	verifier := NewVerifier(transport, overlay, id)
	reporter, err := NewReporter(ctx, statDBPort, maxRetries)
	if err != nil {
		return nil, err
	}

	return &Service{
		Cursor:   cursor,
		Verifier: verifier,
		Reporter: reporter,
		ticker:   time.NewTicker(interval),
	}, nil
}

// Run runs auditing service
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	zap.S().Info("Audit cron is starting up")

	for {
		err := service.process(ctx)
		if err != nil {
			zap.L().Error("process", zap.Error(err))
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

	authorization, err := service.Cursor.pointers.SignedMessage()
	if err != nil {
		return err
	}

	verifiedNodes, err := service.Verifier.verify(ctx, stripe.Index, stripe.Segment, authorization)
	if err != nil {
		return err
	}

	err = service.Reporter.RecordAudits(ctx, verifiedNodes)
	// TODO: if Error.Has(err) then log the error because it means not all node stats updated
	if err != nil {
		return err
	}

	return nil
}
