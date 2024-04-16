// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"log"
	"os"

	"cloud.google.com/go/spanner"
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

// TestingBatchInsertSegments implements Adapter.
func (s *SpannerAdapter) TestingBatchInsertSegments(ctx context.Context, segments []RawSegment) (err error) {
	// TODO implement
	return nil
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

// TestingBeginObjectExactVersion implements Adapter.
func (s *SpannerAdapter) TestingBeginObjectExactVersion(ctx context.Context, opts BeginObjectExactVersion, object *Object) error {
	panic("implement me")
}

// GetObjectLastCommitted implements Adapter.
func (s *SpannerAdapter) GetObjectLastCommitted(ctx context.Context, opts GetObjectLastCommitted, object *Object) error {
	panic("implement me")
}

// Close closes the internal client.
func (s *SpannerAdapter) Close() error {
	s.client.Close()
	return nil
}

var _ Adapter = &SpannerAdapter{}
