// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/tagsql"
)

// SpannerConfig includes all the configuration required by using spanner.
type SpannerConfig struct {
	Database        string `help:"Database definition for spanner connection in the form  projects/P/instances/I/databases/DB"`
	ApplicationName string `help:"Application name to be used in spanner client as a tag for queries and transactions"`
	Compression     string `help:"Compression type to be used in spanner client for gRPC calls (gzip)"`

	HealthCheckWorkers  int           `hidden:"true" help:"Number of workers used by health checker for the connection pool." default:"10" testDefault:"1"`
	HealthCheckInterval time.Duration `hidden:"true" help:"How often the health checker pings a session." default:"50ms" testDefault:"200ms"`
}

// SpannerAdapter implements Adapter for Google Spanner connections..
type SpannerAdapter struct {
	log         *zap.Logger
	client      *spanner.Client
	adminClient *database.DatabaseAdminClient
	sqlClient   tagsql.DB

	connParams spannerutil.ConnParams
}

// NewSpannerAdapter creates a new Spanner adapter.
func NewSpannerAdapter(ctx context.Context, cfg SpannerConfig, log *zap.Logger) (*SpannerAdapter, error) {
	params, err := spannerutil.ParseConnStr(cfg.Database)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	adminClient, err := database.NewDatabaseAdminClient(ctx, params.ClientOptions()...)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	log = log.Named("spanner")

	poolConfig := spanner.DefaultSessionPoolConfig
	poolConfig.HealthCheckWorkers = cfg.HealthCheckWorkers
	poolConfig.HealthCheckInterval = cfg.HealthCheckInterval

	client, err := spanner.NewClientWithConfig(ctx, params.DatabasePath(),
		spanner.ClientConfig{
			Logger:               zap.NewStdLog(log.Named("stdlog")),
			SessionPoolConfig:    poolConfig,
			Compression:          cfg.Compression,
			DisableRouteToLeader: false,
		}, params.ClientOptions()...)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	sqlClient, err := sql.Open("spanner", params.GoSqlSpannerConnStr())
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &SpannerAdapter{
		client:      client,
		connParams:  params,
		adminClient: adminClient,
		sqlClient:   tagsql.Wrap(sqlClient),
		log:         log,
	}, nil
}

// Close closes the internal client.
func (s *SpannerAdapter) Close() error {
	s.client.Close()
	return s.adminClient.Close()
}

// Name returns the name of the adapter.
func (s *SpannerAdapter) Name() string {
	return "spanner"
}

// UnderlyingDB returns a handle to the underlying DB.
func (s *SpannerAdapter) UnderlyingDB() *spanner.Client {
	return s.client
}

// Implementation returns the dbutil.Implementation code for the adapter.
func (s *SpannerAdapter) Implementation() dbutil.Implementation {
	return dbutil.Spanner
}

var _ Adapter = &SpannerAdapter{}
