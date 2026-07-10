// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"context"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/mud"
)

// ConfigVariation is a function that modifies metabase configuration.
//
// This is a type alias so that ordinary function declarations (e.g.
// WithTimestampVersioning) carry an interface-compatible dynamic type when
// passed through the RunFlag (any) variadic in RunWithConfigAndMigration.
type ConfigVariation = func(config *metabase.Config) (name string)

// RunFlag is a flag that can be used to run tests with specific flags.
type RunFlag any

// WithTimestampVersioning modifies metabase configuration to use timestamp versioning.
func WithTimestampVersioning(config *metabase.Config) (name string) {
	config.TestingTimestampVersioning = true
	return "tsver"
}

// RunWithConfig runs tests with specific metabase configuration.
func RunWithConfig(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), flags ...RunFlag) {
	migration := func(ctx context.Context, db *metabase.DB) error {
		return db.TestMigrateToLatest(ctx)
	}
	RunWithConfigAndMigration(t, config, fn, migration, flags...)
}

// RunWithConfigAndMigration runs tests with specific metabase configuration and migration type.
func RunWithConfigAndMigration(t *testing.T, config metabase.Config, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), migration func(ctx context.Context, db *metabase.DB) error, flags ...RunFlag) {
	t.Parallel()

	for _, dbinfo := range satellitedbtest.Databases(t) {
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			testmonkit.Run(t.Context(), t, func(ctx context.Context) {
				tctx := testcontext.NewWithContext(ctx, t)
				defer tctx.Cleanup()

				db, err := satellitedbtest.CreateMetabaseDB(tctx, zaptest.NewLogger(t), t.Name(), "M", 0, dbinfo.MetabaseDB, config)
				require.NoError(t, err)
				defer tctx.Check(db.Close)

				if err := migration(tctx, db); err != nil {
					t.Fatal(err)
				}

				fn(tctx, t, db)
			})
		})

		for _, flag := range flags {
			variation, ok := flag.(ConfigVariation)
			if !ok {
				continue
			}

			varConfig := config
			name := variation(&varConfig)
			t.Run(dbinfo.Name+"-"+name, func(t *testing.T) {
				t.Parallel()

				testmonkit.Run(t.Context(), t, func(ctx context.Context) {
					tctx := testcontext.NewWithContext(ctx, t)
					defer tctx.Cleanup()

					db, err := satellitedbtest.CreateMetabaseDB(tctx, zaptest.NewLogger(t), t.Name(), "M", 0, dbinfo.MetabaseDB, varConfig)
					require.NoError(t, err)
					defer tctx.Check(db.Close)

					if err := migration(tctx, db); err != nil {
						t.Fatal(err)
					}

					fn(tctx, t, db)
				})
			})
		}
	}
}

// Run runs tests against all configured databases.
func Run(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), flags ...RunFlag) {
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
		TestingUniqueUnversioned:   true,
		TestingTimestampVersioning: config.TestingTimestampVersioning,
		DefaultListMode:            metabase.ListMode(config.DefaultListMode),
	}, fn, flags...)
}

// RunWithMigration runs test with specific migration.
func RunWithMigration(t *testing.T, fn func(ctx *testcontext.Context, t *testing.T, db *metabase.DB), migration func(ctx context.Context, db *metabase.DB) error, flags ...RunFlag) {
	var config metainfo.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &config,
		cfgstruct.UseTestDefaults(),
	)

	RunWithConfigAndMigration(t, metabase.Config{
		ApplicationName:            "satellite-metabase-test",
		MinPartSize:                config.MinPartSize,
		MaxNumberOfParts:           config.MaxNumberOfParts,
		ServerSideCopy:             config.ServerSideCopy,
		ServerSideCopyDisabled:     config.ServerSideCopyDisabled,
		TestingUniqueUnversioned:   true,
		TestingTimestampVersioning: config.TestingTimestampVersioning,
		DefaultListMode:            metabase.ListMode(config.DefaultListMode),
	}, fn, migration, flags...)
}

// TransitionFunc is the test body run by RunTransition. projectID is routed
// through the transition adapter; primary and secondary are the underlying
// backends (adapter 0 and 1) for seeding/inspecting a specific side directly.
type TransitionFunc = func(ctx *testcontext.Context, t *testing.T, db *metabase.DB, projectID uuid.UUID, primary, secondary metabase.Adapter)

// RunTransition runs tests against a metabase.DB configured with two backends of
// the same database engine, with projectID routed through the transition
// adapter (primary = adapter 0, secondary = adapter 1). New writes for that
// project land in the primary backend, while existing data is read from
// whichever backend holds it.
func RunTransition(t *testing.T, fn TransitionFunc) {
	// Note: this intentionally does not call t.Parallel() at the function level.
	// Each transition test opens two databases per backend; running the many
	// transition test functions concurrently exhausts backend connection limits
	// (e.g. Postgres "too many clients"). The per-backend subtests below still
	// run in parallel, and each backend is a separate server, so a single
	// transition test only ever holds two connections-worth per backend.
	var miCfg metainfo.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &miCfg, cfgstruct.UseTestDefaults())

	for _, dbinfo := range satellitedbtest.Databases(t) {
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			testmonkit.Run(t.Context(), t, func(ctx context.Context) {
				tctx := testcontext.NewWithContext(ctx, t)
				defer tctx.Cleanup()

				log := zaptest.NewLogger(t)

				primaryDB, err := satellitedbtest.CreateTempDB(tctx, log, satellitedbtest.TempDBSchemaConfig{
					Name:     "transition",
					Category: "M",
					Index:    0,
				}, dbinfo.MetabaseDB)
				require.NoError(t, err)

				secondaryDB, err := satellitedbtest.CreateTempDB(tctx, log, satellitedbtest.TempDBSchemaConfig{
					Name:     "transition",
					Category: "M",
					Index:    1,
				}, dbinfo.MetabaseDB)
				require.NoError(t, err)

				projectID := testrand.UUID()
				config := metabase.Config{
					ApplicationName:          "satellite-metabase-transition-test",
					MinPartSize:              miCfg.MinPartSize,
					MaxNumberOfParts:         miCfg.MaxNumberOfParts,
					ServerSideCopy:           miCfg.ServerSideCopy,
					ServerSideCopyDisabled:   miCfg.ServerSideCopyDisabled,
					TestingUniqueUnversioned: true,
					DefaultListMode:          metabase.ListMode(miCfg.DefaultListMode),
					ProjectTransition: map[uuid.UUID]metabase.TransitionRoute{
						projectID: {Primary: 0, Secondary: 1},
					},
					// Each transition test opens two databases; bound the pool so
					// running the suite across backends doesn't exhaust connections.
					ConnParams: &dbutil.ConnParams{MaxIdleConns: 1, MaxOpenConns: 3},
				}

				db, err := metabase.Open(tctx, log.Named("metabase"), primaryDB.ConnStr+";"+secondaryDB.ConnStr, config)
				require.NoError(t, err)
				db.TestingSetCleanup(func() error {
					return errs.Combine(primaryDB.Close(), secondaryDB.Close())
				})
				defer tctx.Check(db.Close)

				require.NoError(t, db.TestMigrateToLatest(tctx))

				adapters := db.TestingAdapters()
				require.Len(t, adapters, 2)

				fn(tctx, t, db, projectID, adapters[0], adapters[1])
			})
		})
	}
}

// RunMirror runs tests against a metabase.DB configured with two backends of
// the same database engine, with projectID routed through the mirror adapter
// (primary = adapter 0, secondary = adapter 1). All reads and writes for that
// project are served by the primary; writes are additionally mirrored onto the
// secondary in the background.
func RunMirror(t *testing.T, fn TransitionFunc) {
	// See RunTransition for why this does not call t.Parallel() at the top level.
	var miCfg metainfo.Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &miCfg, cfgstruct.UseTestDefaults())

	for _, dbinfo := range satellitedbtest.Databases(t) {
		t.Run(dbinfo.Name, func(t *testing.T) {
			t.Parallel()

			testmonkit.Run(t.Context(), t, func(ctx context.Context) {
				tctx := testcontext.NewWithContext(ctx, t)
				defer tctx.Cleanup()

				log := zaptest.NewLogger(t)

				primaryDB, err := satellitedbtest.CreateTempDB(tctx, log, satellitedbtest.TempDBSchemaConfig{
					Name:     "mirror",
					Category: "M",
					Index:    0,
				}, dbinfo.MetabaseDB)
				require.NoError(t, err)

				secondaryDB, err := satellitedbtest.CreateTempDB(tctx, log, satellitedbtest.TempDBSchemaConfig{
					Name:     "mirror",
					Category: "M",
					Index:    1,
				}, dbinfo.MetabaseDB)
				require.NoError(t, err)

				projectID := testrand.UUID()
				config := metabase.Config{
					ApplicationName:          "satellite-metabase-mirror-test",
					MinPartSize:              miCfg.MinPartSize,
					MaxNumberOfParts:         miCfg.MaxNumberOfParts,
					ServerSideCopy:           miCfg.ServerSideCopy,
					ServerSideCopyDisabled:   miCfg.ServerSideCopyDisabled,
					TestingUniqueUnversioned: true,
					ProjectMirror: map[uuid.UUID]metabase.TransitionRoute{
						projectID: {Primary: 0, Secondary: 1},
					},
					ConnParams: &dbutil.ConnParams{MaxIdleConns: 1, MaxOpenConns: 3},
				}

				db, err := metabase.Open(tctx, log.Named("metabase"), primaryDB.ConnStr+";"+secondaryDB.ConnStr, config)
				require.NoError(t, err)
				db.TestingSetCleanup(func() error {
					return errs.Combine(primaryDB.Close(), secondaryDB.Close())
				})
				defer tctx.Check(db.Close)

				require.NoError(t, db.TestMigrateToLatest(tctx))

				adapters := db.TestingAdapters()
				require.Len(t, adapters, 2)

				fn(tctx, t, db, projectID, adapters[0], adapters[1])
			})
		})
	}
}

// Bench runs benchmark for all configured databases.
func Bench(b *testing.B, fn func(ctx *testcontext.Context, b *testing.B, db *metabase.DB)) {
	for _, dbinfo := range satellitedbtest.Databases(b) {
		dbinfo := dbinfo
		b.Run(dbinfo.Name, func(b *testing.B) {
			tctx := testcontext.New(b)
			defer tctx.Cleanup()

			db, err := satellitedbtest.CreateMetabaseDB(tctx, zaptest.NewLogger(b), b.Name(), "M", 0, dbinfo.MetabaseDB, metabase.Config{
				ApplicationName:  "satellite-bench",
				MinPartSize:      5 * memory.MiB,
				MaxNumberOfParts: 10000,
			})
			require.NoError(b, err)
			defer tctx.Check(db.Close)

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
