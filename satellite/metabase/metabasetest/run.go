// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"context"
	"strings"
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
	migration := func(ctx context.Context, db *metabase.DB) error {
		return db.TestMigrateToLatest(ctx)
	}
	RunWithConfigAndMigration(t, config, fn, migration, flags...)
}

// RunWithConfigAndMigration runs tests with specific metabase configuration and migration type.
func RunWithConfigAndMigration(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), migration func(ctx context.Context, db *metabase.DB) error, flags ...interface{}) {
	spannerTestEnabled := slices.ContainsFunc(flags, func(flag interface{}) bool {
		return flag == withSpanner
	})

	for _, dbinfo := range satellitedbtest.DatabasesWithSpanner() {
		if !spannerTestEnabled && strings.HasPrefix(dbinfo.MetabaseDB.URL, "spanner:") {
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
	mud.Supply[satellitedbtest.SatelliteDatabases](ball, dbinfo)
	switch dbinfo.MetabaseDB.Name {
	case "Spanner":
		mud.Provide[tempDB](ball, func(ctx context.Context, logger *zap.Logger) (tempDB, error) {
			return metabase.NewSpannerTestDatabase(ctx, logger, dbinfo.MetabaseDB.URL, true)
		})
	default:
		mud.Provide[tempDB](ball, newPgTempDB)
	}

	mud.Provide[*metabase.DB](ball, openTempDatabase)
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
	mud.RegisterImplementation[[]metabase.Adapter](ball)

}

type tempDB interface {
	Close() error
	Connection() string
}

// pgTempDB is the temporary database wrapper for cockroach and postgres.
// DB is deleted on close.
type pgTempDB struct {
	*dbutil.TempDatabase
}

func (p pgTempDB) Close() error {
	return p.TempDatabase.Close()
}

func (p pgTempDB) Connection() string {
	return p.ConnStr
}

func newPgTempDB(ctx context.Context, log *zap.Logger, dbinfo satellitedbtest.SatelliteDatabases) (tempDB, error) {
	tempDB, err := satellitedbtest.CreateTempDB(ctx, log, satellitedbtest.TempDBSchemaConfig{
		Name:     "test",
		Category: "M",
		Index:    0,
	}, satellitedbtest.Database{
		Name:    dbinfo.MetabaseDB.Name,
		URL:     dbinfo.MetabaseDB.URL,
		Message: dbinfo.MetabaseDB.Message,
	})
	if err != nil {
		return nil, err
	}
	return pgTempDB{
		TempDatabase: tempDB,
	}, nil
}

func openTempDatabase(ctx context.Context, log *zap.Logger, tempDB tempDB, config metabase.Config) (*metabase.DB, error) {
	db, err := metabase.Open(ctx, log.Named("metabase"), tempDB.Connection(), config)
	if err != nil {
		return nil, err
	}
	return db, nil
}
