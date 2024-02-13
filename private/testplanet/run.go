// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"runtime/pprof"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/common/dbutil"
	"storj.io/common/dbutil/pgtest"
	"storj.io/common/dbutil/pgutil"
	"storj.io/common/tagsql"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	databases := satellitedbtest.Databases()
	if len(databases) == 0 {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + pgtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + pgtest.DefaultCockroach)
	}

	for _, satelliteDB := range databases {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			parallel := !config.NonParallel
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

			testmonkit.Run(context.Background(), t, func(parent context.Context) {
				defer pprof.SetGoroutineLabels(parent)
				parent = pprof.WithLabels(parent, pprof.Labels("test", t.Name()))

				timeout := config.Timeout
				if timeout == 0 {
					timeout = testcontext.DefaultTimeout
				}
				ctx := testcontext.NewWithContextAndTimeout(parent, t, timeout)
				defer ctx.Cleanup()

				planetConfig.applicationName = "testplanet" + pgutil.CreateRandomTestingSchemaName(6)
				planet, err := NewCustom(ctx, log, planetConfig, satelliteDB)
				if err != nil {
					t.Fatalf("%+v", err)
				}
				defer ctx.Check(planet.Shutdown)

				planet.Start(ctx)

				var rawDB tagsql.DB
				var queriesBefore []string
				if len(planet.Satellites) > 0 && satelliteDB.Name == "Cockroach" {
					rawDB = planet.Satellites[0].DB.Testing().RawDB()

					var err error
					queriesBefore, err = satellitedbtest.FullTableScanQueries(ctx, rawDB, dbutil.Cockroach, planetConfig.applicationName)
					if err != nil {
						t.Fatalf("%+v", err)
					}
				}

				test(t, ctx, planet)

				if rawDB != nil {
					queriesAfter, err := satellitedbtest.FullTableScanQueries(context2.WithoutCancellation(ctx), rawDB, dbutil.Cockroach, planetConfig.applicationName)
					if err != nil {
						t.Fatalf("%+v", err)
					}

					diff := cmp.Diff(queriesBefore, queriesAfter)
					if diff != "" {
						log.Sugar().Warnf("FULL TABLE SCAN DETECTED\n%s", diff)
					}
				}
			})
		})
	}
}

// Bench makes benchmark with testplanet as easy as running unit tests with Run method.
func Bench(b *testing.B, config Config, bench func(b *testing.B, ctx *testcontext.Context, planet *Planet)) {
	databases := satellitedbtest.Databases()
	if len(databases) == 0 {
		b.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + pgtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + pgtest.DefaultCockroach)
	}

	for _, satelliteDB := range databases {
		satelliteDB := satelliteDB
		b.Run(satelliteDB.Name, func(b *testing.B) {
			if satelliteDB.MasterDB.URL == "" {
				b.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}

			log := zap.NewNop()

			planetConfig := config
			if planetConfig.Name == "" {
				planetConfig.Name = b.Name()
			}

			testmonkit.Run(context.Background(), b, func(parent context.Context) {
				defer pprof.SetGoroutineLabels(parent)
				parent = pprof.WithLabels(parent, pprof.Labels("test", b.Name()))

				timeout := config.Timeout
				if timeout == 0 {
					timeout = testcontext.DefaultTimeout
				}
				ctx := testcontext.NewWithContextAndTimeout(parent, b, timeout)
				defer ctx.Cleanup()

				planetConfig.applicationName = "testplanet-bench"
				planet, err := NewCustom(ctx, log, planetConfig, satelliteDB)
				if err != nil {
					b.Fatalf("%+v", err)
				}
				defer ctx.Check(planet.Shutdown)

				planet.Start(ctx)

				bench(b, ctx, planet)
			})
		})
	}
}
