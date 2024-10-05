// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"strings"
	"time"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/mud"
	"storj.io/storj/shared/dbutil/spannerutil"
)

//go:embed adapter_spanner_scheme.sql
var spannerDDL string

// SpannerTestModule adds all the required dependencies for Spanner migration and adapter.
func SpannerTestModule(ball *mud.Ball, spannerConnection string) {
	mud.Provide[*SpannerAdapter](ball, NewSpannerAdapter)
	mud.Implementation[[]Adapter, *SpannerAdapter](ball)
	mud.RemoveTag[*SpannerAdapter, mud.Optional](ball)
	// Please note that SpannerTestDatabase creates / deletes temporary database via the lifecycle functions.
	mud.Provide[SpannerTestDatabase](ball, func(ctx context.Context, logger *zap.Logger) (SpannerTestDatabase, error) {
		return NewSpannerTestDatabase(ctx, logger, spannerConnection, true)
	})
	mud.Provide[SpannerConfig](ball, NewTestSpannerConfig)
}

// SpannerTestDatabase manages Spanner database and migration for tests.
type SpannerTestDatabase struct {
	connParams spannerutil.ConnParams
	client     *database.DatabaseAdminClient
}

// NewSpannerTestDatabase creates the database (=creates / migrates the database).
func NewSpannerTestDatabase(ctx context.Context, logger *zap.Logger, spannerConnection string, withMigration bool) (SpannerTestDatabase, error) {
	params, err := spannerutil.ParseConnStr(spannerConnection)
	if err != nil {
		return SpannerTestDatabase{}, errs.New("invalid connstr: %w", err)
	}

	data := make([]byte, 8)
	_, err = rand.Read(data)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}

	adminClient, err := database.NewDatabaseAdminClient(ctx, params.ClientOptions()...)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}

	params.Database += "_" + hex.EncodeToString(data)
	logger.Info("Creating temporary spanner database", zap.String("db", params.Database))

	if !params.AllDefined() {
		return SpannerTestDatabase{}, errs.New("database connection should be defined in the form of 'spanner://<host:port>/projects/<PROJECT>/instances/<INSTANCE>/databases/<DATABASE>', but it was %q", spannerConnection)
	}

	req := &databasepb.CreateDatabaseRequest{
		Parent:          params.InstancePath(),
		DatabaseDialect: databasepb.DatabaseDialect_GOOGLE_STANDARD_SQL,
		CreateStatement: "CREATE DATABASE `" + params.Database + "`",
	}

	if withMigration {
		for _, ddl := range strings.Split(spannerDDL, ";") {
			if strings.TrimSpace(ddl) != "" {
				req.ExtraStatements = append(req.ExtraStatements, ddl)
			}
		}
	}
	ddl, err := adminClient.CreateDatabase(ctx, req)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}
	_, err = ddl.Wait(ctx)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}
	return SpannerTestDatabase{
		connParams: params,
		client:     adminClient,
	}, nil
}

// Connection returns with the used connection string (with added unique suffix).
func (d SpannerTestDatabase) Connection() string {
	return d.connParams.ConnStr()
}

// Close drops the temporary test database.
func (d SpannerTestDatabase) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := d.client.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: d.connParams.DatabasePath(),
	})
	return errs.Combine(err, d.client.Close())
}

// NewTestSpannerConfig creates SpannerConfig for testing.
func NewTestSpannerConfig(database SpannerTestDatabase) SpannerConfig {
	return SpannerConfig{
		Database: database.connParams.ConnStr(),
	}
}
