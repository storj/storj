// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package outreach

import (
	"context"
	"time"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

type Config struct {
	Interval time.Duration `help:"how frequently the node outreach service should run" releaseDefault:"1h" devDefault:"30s"`
}

var (
	mon = monkit.Package()
)

// Service is the outreach service for nodes announcing themselves to their trusted satellites
type Service struct {
	log    *zap.Logger
	ticker *time.Ticker
}

// NewService creates a new outreach service
func NewService(log *zap.Logger, interval time.Duration) *Service {
	return &Service{
		log:    log,
		ticker: time.NewTicker(interval),
	}
}

// Run the outreach service on a regular interval with jitter
func (service *Service) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("Storagenode outreach service starting up")
	// TODO create jitter
	for {
		// TODO: update this section if needed
		if err = service.pingSatellites(ctx); err != nil {
			service.log.Error("pingSatellites failed", zap.Error(err))
		}
		select {
		case <-service.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or outreach is canceled via context
			return ctx.Err()
		}
	}
	return nil
}

func (service *Service) pingSatellites(ctx context.Context) error {
	// loop through the trusted satellites
	// don't error out if an individual satellite ping fails
	// call awaitPingback helper method
	return nil
}

func awaitPingback() {
	// TODO: write a method that listens for each satellite to ping back, or it times out and receives a log message
	//  with which satellite they failed
	//  make sure to close the connections regardless
}
