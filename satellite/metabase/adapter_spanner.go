// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"log"
	"os"

	"github.com/storj/exp-spanner"
	"github.com/zeebo/errs"
)

// SpannerConfig includes all the configuration required by using spanner.
type SpannerConfig struct {
	Database string `help:"Database definition for spanner connection in the form  projects/P/instances/I/databases/DB"`
}

// SpannerAdapter implements Adapter for Google Spanner connections..
type SpannerAdapter struct {
	client *spanner.Client
}

// NewSpannerAdapter creates a new Spanner adapter.
func NewSpannerAdapter(ctx context.Context, cfg SpannerConfig) (*SpannerAdapter, error) {
	client, err := spanner.NewClientWithConfig(ctx, cfg.Database,
		spanner.ClientConfig{
			Logger:               log.New(os.Stdout, "spanner", log.LstdFlags),
			SessionPoolConfig:    spanner.DefaultSessionPoolConfig,
			DisableRouteToLeader: false})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &SpannerAdapter{
		client: client,
	}, nil
}

// Close closes the internal client.
func (s *SpannerAdapter) Close() error {
	s.client.Close()
	return nil
}

// Name returns the name of the adapter.
func (s *SpannerAdapter) Name() string {
	return "spanner"
}

var _ Adapter = &SpannerAdapter{}
