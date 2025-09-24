// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"time"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	sqlspanner "github.com/googleapis/go-sql-spanner"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/api/option"

	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/recordeddb"
	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/flightrecorder"
	"storj.io/storj/shared/tagsql"
)

// SpannerConfig includes all the configuration required by using spanner.
type SpannerConfig struct {
	Database        string `help:"Database definition for spanner connection in the form  projects/P/instances/I/databases/DB"`
	ApplicationName string `help:"Application name to be used in spanner client as a tag for queries and transactions"`
	Compression     string `help:"Compression type to be used in spanner client for gRPC calls (gzip)"`

	HealthCheckWorkers  int           `hidden:"true" help:"Number of workers used by health checker for the connection pool." default:"10" testDefault:"1"`
	HealthCheckInterval time.Duration `hidden:"true" help:"How often the health checker pings a session." default:"50m"`
	MinOpenedSesssions  uint64        `hidden:"true" help:"Minimum number of sessions that client tries to keep open." default:"100"`
	TrackSessionHandles bool          `hidden:"true" help:"Track session handles." default:"false" testDefault:"true"`

	TestingTimestampVersioning bool `hidden:"true" help:"Use timestamps for assigning version numbers instead of strictly incrementing integers." default:"false"`
}

// SpannerAdapter implements Adapter for Google Spanner connections..
type SpannerAdapter struct {
	log         *zap.Logger
	client      *recordeddb.SpannerClient
	adminClient *database.DatabaseAdminClient
	sqlClient   tagsql.DB

	connParams spannerutil.ConnParams

	testingTimestampVersioning bool
}

// NewSpannerAdapter creates a new Spanner adapter.
func NewSpannerAdapter(ctx context.Context, cfg SpannerConfig, log *zap.Logger, recorder *flightrecorder.Box) (*SpannerAdapter, error) {
	params, err := spannerutil.ParseConnStr(cfg.Database)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	opts := params.ClientOptions()
	// TODO user agent is added in two ways to verify which one is what we need
	opts = append(opts, option.WithUserAgent(cfg.ApplicationName+"-alt"))

	adminClient, err := database.NewDatabaseAdminClient(ctx, opts...)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	log = log.Named("spanner")

	poolConfig := spanner.DefaultSessionPoolConfig
	poolConfig.MinOpened = cfg.MinOpenedSesssions
	poolConfig.HealthCheckWorkers = cfg.HealthCheckWorkers
	poolConfig.HealthCheckInterval = cfg.HealthCheckInterval
	poolConfig.TrackSessionHandles = cfg.TrackSessionHandles

	rawClient, err := spanner.NewClientWithConfig(ctx, params.DatabasePath(),
		spanner.ClientConfig{
			Logger:               zap.NewStdLog(log.Named("stdlog")),
			SessionPoolConfig:    poolConfig,
			Compression:          cfg.Compression,
			DisableRouteToLeader: false,
			UserAgent:            cfg.ApplicationName,
		}, opts...)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	client := recordeddb.WrapSpannerClient(rawClient, recorder)

	sqlconfig, err := sqlspanner.ExtractConnectorConfig(params.GoSqlSpannerConnStr())
	if err != nil {
		return nil, errs.Wrap(err)
	}

	sqlconfig.Configurator = func(config *spanner.ClientConfig, opts *[]option.ClientOption) {
		config.Logger = zap.NewStdLog(log.Named("sqllog"))
		config.Compression = cfg.Compression
		config.SessionPoolConfig = poolConfig
		config.DisableRouteToLeader = false
		config.UserAgent = cfg.ApplicationName
	}

	connector, err := sqlspanner.CreateConnector(sqlconfig)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	sqlClient := sql.OpenDB(connector)

	return &SpannerAdapter{
		client:      client,
		connParams:  params,
		adminClient: adminClient,
		sqlClient:   tagsql.WrapWithRecorder(sqlClient, recorder),
		log:         log,

		testingTimestampVersioning: cfg.TestingTimestampVersioning,
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
func (s *SpannerAdapter) UnderlyingDB() *recordeddb.SpannerClient {
	return s.client
}

// Implementation returns the dbutil.Implementation code for the adapter.
func (s *SpannerAdapter) Implementation() dbutil.Implementation {
	return dbutil.Spanner
}

// IsEmulator returns true if the underlying DB is spanner emulator
func (s *SpannerAdapter) IsEmulator() bool {
	return s.connParams.Emulator
}

var _ Adapter = &SpannerAdapter{}
