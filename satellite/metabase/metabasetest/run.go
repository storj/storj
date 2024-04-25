// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"context"
	"testing"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/storj/private/mud"
	"storj.io/storj/private/mud/mudtest"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
)

// RunWithConfig runs tests with specific metabase configuration.
func RunWithConfig(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), flags ...interface{}) {
	RunWithConfigAndMigration(t, config, fn, func(ctx context.Context, db *metabase.DB) error {
		return db.TestMigrateToLatest(ctx)
	}, flags...)
}

// RunWithConfigAndMigration runs tests with specific metabase configuration and migration type.
func RunWithConfigAndMigration(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), migration func(ctx context.Context, db *metabase.DB) error, flags ...interface{}) {
	spannerTestEnabled := slices.ContainsFunc(flags, func(flag interface{}) bool {
		return flag == withSpanner
	})

	for _, dbinfo := range satellitedbtest.DatabasesWithSpanner() {
		if dbinfo.Spanner != "" && !spannerTestEnabled {
			continue
		}
		dbinfo := dbinfo
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			mudtest.Run[*metabase.DB](t, mudtest.WithTestLogger(t, func(ball *mud.Ball) {
				TestModule(ball, dbinfo, config)
			}), func(ctx context.Context, t *testing.T, db *metabase.DB) {
				tctx := testcontext.New(t)
				defer tctx.Cleanup()

				if err := migration(ctx, db); err != nil {
					t.Fatal(err)
				}

				fn(tctx, t, db)
			})
		})
	}
}

var withSpanner = struct{}{}

// WithSpanner flags the metabase test as ready for testing it with spanner.
func WithSpanner() struct{} {
	return withSpanner
}

// Run runs tests against all configured databases.
func Run(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), flags ...interface{}) {
	var config metainfo.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &config,
		cfgstruct.UseTestDefaults(),
	)

	RunWithConfig(t, metabase.Config{
		ApplicationName:  "satellite-metabase-test",
		MinPartSize:      config.MinPartSize,
		MaxNumberOfParts: config.MaxNumberOfParts,

		ServerSideCopy:         config.ServerSideCopy,
		ServerSideCopyDisabled: config.ServerSideCopyDisabled,
		UseListObjectsIterator: config.UseListObjectsIterator,

		TestingUniqueUnversioned: true,
	}, fn, flags...)
}

// Bench runs benchmark for all configured databases.
func Bench(b *testing.B, fn func(ctx *testcontext.Context, b *testing.B, db *metabase.DB)) {
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		b.Run(dbinfo.Name, func(b *testing.B) {
			config := metabase.Config{
				ApplicationName:  "satellite-bench",
				MinPartSize:      5 * memory.MiB,
				MaxNumberOfParts: 10000,
			}

			mudtest.Run[*metabase.DB](b, mudtest.WithTestLogger(b, func(ball *mud.Ball) {
				TestModule(ball, dbinfo, config)
			}), func(ctx context.Context, b *testing.B, db *metabase.DB) {
				tctx := testcontext.New(b)
				defer tctx.Cleanup()

				if err := db.TestMigrateToLatest(ctx); err != nil {
					b.Fatal(err)
				}

				b.ResetTimer()
				fn(tctx, b, db)
			})
		})
	}
}

// TestModule provides all dependencies to run metabase tests.
func TestModule(ball *mud.Ball, dbinfo satellitedbtest.SatelliteDatabases, config metabase.Config) {
	mud.Provide[*metabase.DB](ball, createMetabaseDBOnTopOf)
	mud.Provide[*dbutil.TempDatabase](ball, satellitedbtest.CreateTempDB)
	mud.Provide[metabase.Config](ball, func() metabase.Config {
		cfg := metabase.Config{
			ApplicationName:  "satellite-metabase-test" + pgutil.CreateRandomTestingSchemaName(6),
			MinPartSize:      config.MinPartSize,
			MaxNumberOfParts: config.MaxNumberOfParts,

			ServerSideCopy:         config.ServerSideCopy,
			ServerSideCopyDisabled: config.ServerSideCopyDisabled,

			TestingUniqueUnversioned: true,
		}
		return cfg
	})
	mud.Supply[satellitedbtest.Database](ball, satellitedbtest.Database{
		Name:    dbinfo.MetabaseDB.Name,
		URL:     dbinfo.MetabaseDB.URL,
		Message: dbinfo.MetabaseDB.Message,
	})
	mud.Supply[satellitedbtest.TempDBSchemaConfig](ball, satellitedbtest.TempDBSchemaConfig{
		Name:     "test",
		Category: "M",
		Index:    0,
	})
	mud.RegisterImplementation[[]metabase.Adapter](ball)
	if dbinfo.Spanner != "" {
		metabase.SpannerTestModule(ball, dbinfo.Spanner)
	}

}

func createMetabaseDBOnTopOf(ctx context.Context, log *zap.Logger, tempDB *dbutil.TempDatabase, config metabase.Config, adapters []metabase.Adapter) (*metabase.DB, error) {
	db, err := metabase.Open(ctx, log.Named("metabase"), tempDB.ConnStr, config, adapters...)
	if err != nil {
		return nil, err
	}
	return db, nil
}
