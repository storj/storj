// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"context"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/storj/private/mud"
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
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			tctx := testcontext.New(t)
			defer tctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(tctx, zaptest.NewLogger(t), t.Name(), "M", 0, dbinfo.MetabaseDB, config)
			require.NoError(t, err)

			if err := migration(tctx, db); err != nil {
				t.Fatal(err)
			}

			fn(tctx, t, db)
		})
	}
}

// Run runs tests against all configured databases.
func Run(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), flags ...interface{}) {
	var config metainfo.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &config,
		cfgstruct.UseTestDefaults(),
	)

	RunWithConfig(t, metabase.Config{
		ApplicationName:            "satellite-metabase-test",
		MinPartSize:                config.MinPartSize,
		MaxNumberOfParts:           config.MaxNumberOfParts,
		ServerSideCopy:             config.ServerSideCopy,
		ServerSideCopyDisabled:     config.ServerSideCopyDisabled,
		UseListObjectsIterator:     config.UseListObjectsIterator,
		TestingUniqueUnversioned:   true,
		TestingPrecommitDeleteMode: metabase.TestingPrecommitDeleteMode(config.TestingPrecommitDeleteMode),
	}, fn, flags...)
}

// Bench runs benchmark for all configured databases.
func Bench(b *testing.B, fn func(ctx *testcontext.Context, b *testing.B, db *metabase.DB)) {
	for _, dbinfo := range satellitedbtest.Databases() {
		dbinfo := dbinfo
		b.Run(dbinfo.Name, func(b *testing.B) {
			tctx := testcontext.New(b)
			defer tctx.Cleanup()
			db, err := satellitedbtest.CreateMetabaseDB(tctx, zaptest.NewLogger(b), b.Name(), "M", 0, dbinfo.MetabaseDB, metabase.Config{
				ApplicationName:            "satellite-bench",
				MinPartSize:                5 * memory.MiB,
				MaxNumberOfParts:           10000,
				TestingPrecommitDeleteMode: metabase.DefaultUnversionedPrecommitMode,
			})
			require.NoError(b, err)

			if err := db.TestMigrateToLatest(tctx); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			fn(tctx, b, db)

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

			TestingPrecommitDeleteMode: config.TestingPrecommitDeleteMode,
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
