// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventkit

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/eventkit"
	"storj.io/eventkit/bigquery"
)

// Config holds configuration for eventkit.
type Config struct {
	Destination string `help:"destination address(es) to send eventkit telemetry (comma-separated BQ definition, like bigquery:app=...,project=...,dataset=..., depends on the config/usage)"`
}

// Eventkit manages eventkit event publishing.
type Eventkit struct {
	destination eventkit.Destination
}

// NewEventkit creates a new Eventkit instance.
func NewEventkit(ctx context.Context, log *zap.Logger, cfg Config) (_ *Eventkit, err error) {
	eventRegistry := eventkit.DefaultRegistry

	var destination eventkit.Destination
	if cfg.Destination != "" {
		log.Info("Event collection enabled")
		destination, err = bigquery.CreateDestination(ctx, cfg.Destination)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		eventRegistry.AddDestination(destination)
		eventRegistry.Scope("init").Event("init")
	}

	return &Eventkit{
		destination: destination,
	}, nil
}

// Run starts the eventkit destination.
func (e *Eventkit) Run(ctx context.Context) error {

	if e.destination != nil {
		e.destination.Run(ctx)
	}
	return nil
}
