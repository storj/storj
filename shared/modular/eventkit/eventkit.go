// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventkit

import (
	"context"

	"github.com/zeebo/errs"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"

	"storj.io/eventkit"
	"storj.io/eventkit/bigquery"
)

// Config holds configuration for eventkit.
type Config struct {
	Destination string `help:"destination to send eventkit telemetry to. Use 'otel' to emit events as OpenTelemetry log records, or a comma-separated BQ definition (like bigquery:app=...,project=...,dataset=...), depends on the config/usage"`
}

// Eventkit manages eventkit event publishing.
type Eventkit struct {
	destination eventkit.Destination
}

// NewEventkit creates a new Eventkit instance.
func NewEventkit(ctx context.Context, log *zap.Logger, provider *sdklog.LoggerProvider, cfg Config) (_ *Eventkit, err error) {
	eventRegistry := eventkit.DefaultRegistry

	var destination eventkit.Destination
	if cfg.Destination != "" {
		log.Info("Event collection enabled")
		if cfg.Destination == "otel" {
			destination = newOtelDestination(provider)
		} else {
			destination, err = bigquery.CreateDestination(ctx, cfg.Destination)
			if err != nil {
				return nil, errs.Wrap(err)
			}
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
