// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	_ "embed"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/dbutil/spannerutil"
	"storj.io/storj/shared/flightrecorder"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.View[*DB, DB](ball, mud.Dereference[DB])
	// TODO: there are cases when we need all the adapters (like changefeed). We need a better way to configure which should be used for these usescases.
	mud.View[*DB, Adapter](ball, func(db *DB) Adapter {
		for _, v := range db.adapters {
			return v
		}
		panic("no adapters found")
	})

}

// DatabaseConfig is the minimum required configuration for metabase.
type DatabaseConfig struct {
	MigrationUnsafe string `help:"comma separated migration types to run during every startup (none: no migration, snapshot: creating db from latest test snapshot (for testing only), testdata: create testuser in addition to a migration, full: do the normal migration (equals to 'satellite run migration'" default:"none" hidden:"true"`
	URL             string
	Config
}

// OpenDatabaseWithMigration will open the database (and update schema, if required).
func OpenDatabaseWithMigration(ctx context.Context, logger *zap.Logger, cfg DatabaseConfig) (*DB, error) {
	metabaseDB, err := Open(ctx, logger, cfg.URL, Config{
		ApplicationName:  cfg.ApplicationName,
		MinPartSize:      cfg.MinPartSize,
		MaxNumberOfParts: cfg.MaxNumberOfParts,
	})
	if err != nil {
		return nil, errs.New("Error creating metabase connection on satellite api: %+v", err)
	}

	err = MigrateMetainfoDB(ctx, logger, metabaseDB, cfg.MigrationUnsafe)
	if err != nil {
		return nil, err
	}
	return metabaseDB, err
}

//go:embed adapter_spanner_scheme.sql
var spannerDDL string
var spannerDDLs = spannerutil.MustSplitSQLStatements(spannerDDL)

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

	mud.Provide[*flightrecorder.Box](ball, flightrecorder.NewBox)
	mud.Provide[flightrecorder.Config](ball, flightrecorder.NewTestConfig)
}

// SpannerTestDatabase manages Spanner database and migration for tests.
type SpannerTestDatabase struct {
	ephemeral *spannerutil.EphemeralDB
}

// NewSpannerTestDatabase creates the database (=creates / migrates the database).
func NewSpannerTestDatabase(ctx context.Context, logger *zap.Logger, connstr string, withMigration bool) (SpannerTestDatabase, error) {
	var ddls []string
	if withMigration {
		ddls = spannerDDLs
	}

	ephemeral, err := spannerutil.CreateEphemeralDB(ctx, connstr, "", ddls...)
	if err != nil {
		return SpannerTestDatabase{}, errs.Wrap(err)
	}

	return SpannerTestDatabase{
		ephemeral: ephemeral,
	}, nil
}

// Connection returns with the used connection string (with added unique suffix).
func (d SpannerTestDatabase) Connection() string {
	return d.ephemeral.Params.ConnStr()
}

// Close drops the temporary test database.
func (d SpannerTestDatabase) Close() error {
	// TODO: this should not use context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return errs.Wrap(d.ephemeral.Close(ctx))
}

// NewTestSpannerConfig creates SpannerConfig for testing.
func NewTestSpannerConfig(database SpannerTestDatabase) SpannerConfig {
	return SpannerConfig{
		Database: database.ephemeral.Params.ConnStr(),

		HealthCheckWorkers:  2,
		HealthCheckInterval: 50 * time.Minute,
		MinOpenedSesssions:  100,
		TrackSessionHandles: true,
	}
}
