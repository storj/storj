// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

// Service helps coordinate Cursor and Verifier to run the audit process continuously
type Service struct {
	log      *zap.Logger
	Cursor   *Cursor
	Verifier *Verifier
	Reporter reporter
	ticker   *time.Ticker
}

// Config contains configurable values for audit service
type Config struct {
	APIKey           string        `help:"APIKey to access the statdb" default:""`
	SatelliteAddr    string        `help:"address to contact services on the satellite"`
	MaxRetriesStatDB int           `help:"max number of times to attempt updating a statdb batch" default:"3"`
	Interval         time.Duration `help:"how frequently segments are audited" default:"30s"`
}

// Run runs the repairer with the configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	identity := server.Identity()
	pointers := pointerdb.LoadFromContext(ctx)
	if pointers == nil {
		return Error.New("programmer error: pointerdb responsibility unstarted")
	}

	overlay, err := overlay.NewClient(identity, c.SatelliteAddr)
	if err != nil {
		return err
	}
	transport := transport.NewClient(identity)

	log := zap.L()
	service, err := NewService(ctx, log, c.SatelliteAddr, c.Interval, c.MaxRetriesStatDB, pointers, transport, overlay, *identity, c.APIKey)
	if err != nil {
		return err
	}
	go func() {
		err := service.Run(ctx)
		service.log.Error("audit service failed to run:", zap.Error(err))
	}()
	return server.Run(ctx)
}

// NewService instantiates a Service with access to a Cursor and Verifier
func NewService(ctx context.Context, log *zap.Logger, statDBPort string, interval time.Duration, maxRetries int, pointers *pointerdb.Server, transport transport.Client, overlay overlay.Client,
	identity provider.FullIdentity, apiKey string) (service *Service, err error) {

	//TODO: instead of statDBPort pass in the actual database interface
	cursor := NewCursor(pointers)
	verifier := NewVerifier(transport, overlay, identity)
	reporter, err := NewReporter(ctx, statDBPort, maxRetries, apiKey)
	if err != nil {
		return nil, err
	}

	return &Service{
		log:      log,
		Cursor:   cursor,
		Verifier: verifier,
		Reporter: reporter,
		ticker:   time.NewTicker(interval),
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
