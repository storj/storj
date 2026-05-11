// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"slices"
	"testing"

	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqtest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/dbutil/pgutil"
)

// DatabasesForConfig returns the databases configured for the test based on the given config.
func DatabasesForConfig[TB testing.TB](tb TB, config Config) []satellitedbtest.SatelliteDatabases {
	databases := satellitedbtest.Databases(tb)
	if len(databases) == 0 {
		return nil
	}
	databases = slices.DeleteFunc(databases, func(db satellitedbtest.SatelliteDatabases) bool {
		return (db.Name == "Spanner" && config.SkipSpanner) ||
			(db.Name == "TiDB" && !config.EnableTiDB)
	})
	if len(databases) == 0 {
		tb.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + dbtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + dbtest.DefaultCockroach + "\n" +
			"-spanner-test-db=" + dbtest.DefaultSpanner + "\n" +
			"-tidb-test-db=" + dbtest.DefaultTiDB)
	}
	return databases
}

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	parallel := !config.NonParallel
	if parallel {
		t.Parallel()
	}

	for _, satelliteDB := range DatabasesForConfig(t, config) {
		t.Run(satelliteDB.Name, func(t *testing.T) {
			if parallel {
				t.Parallel()
			}

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}
			planetConfig := config
			if planetConfig.Name == "" {
				planetConfig.Name = t.Name()
			}

			log := NewLogger(t)

			jobqtest.WithServer(t, &jobqtest.ServerOptions{
				Host:    planetConfig.Host,
				Timeout: config.Timeout,
			}, func(ctx *testcontext.Context, srv *jobqtest.TestServer) {
				reconfig := func(log *zap.Logger, index int, config *satellite.Config) {
					config.JobQueue = jobq.Config{
						ServerNodeURL: srv.NodeURL,
						TLS:           srv.TLSOpts.Config,
					}
				}
				planetConfig.applicationName = "testplanet" + pgutil.CreateRandomTestingSchemaName(6)
				if planetConfig.Reconfigure.Satellite == nil {
					planetConfig.Reconfigure.Satellite = reconfig
				} else {
					planetConfig.Reconfigure.Satellite = Combine(planetConfig.Reconfigure.Satellite, reconfig)
				}
				planet, err := NewCustom(ctx, log, planetConfig, satelliteDB)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(planet.Shutdown)

				if err := planet.Start(ctx); err != nil {
					t.Fatalf("planet failed to start: %+v", err)
				}

				test(t, ctx, planet)
			})
		})

	}
}

// Bench makes benchmark with testplanet as easy as running unit tests with Run method.
func Bench(b *testing.B, config Config, bench func(b *testing.B, ctx *testcontext.Context, planet *Planet)) {
	for _, satelliteDB := range DatabasesForConfig(b, config) {
		b.Run(satelliteDB.Name, func(b *testing.B) {
			if satelliteDB.MasterDB.URL == "" {
				b.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}

			log := zap.NewNop()

			planetConfig := config
			if planetConfig.Name == "" {
				planetConfig.Name = b.Name()
			}

			jobqtest.WithServer(b, &jobqtest.ServerOptions{
				Host:    planetConfig.Host,
				Timeout: config.Timeout,
			}, func(ctx *testcontext.Context, srv *jobqtest.TestServer) {
				reconfig := func(log *zap.Logger, index int, config *satellite.Config) {
					config.JobQueue = jobq.Config{
						ServerNodeURL: srv.NodeURL,
						TLS:           srv.TLSOpts.Config,
					}
				}
				planetConfig.applicationName = "testplanet-bench"
				if planetConfig.Reconfigure.Satellite == nil {
					planetConfig.Reconfigure.Satellite = reconfig
				} else {
					planetConfig.Reconfigure.Satellite = Combine(planetConfig.Reconfigure.Satellite, reconfig)
				}
				planet, err := NewCustom(ctx, log, planetConfig, satelliteDB)
				if err != nil {
					b.Fatalf("%+v", err)
				}
				defer ctx.Check(planet.Shutdown)

				if err := planet.Start(ctx); err != nil {
					b.Fatalf("planet failed to start: %+v", err)
				}

				bench(b, ctx, planet)
			})
		})
	}
}
