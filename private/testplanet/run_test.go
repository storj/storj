// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"context"
	"runtime/pprof"
	"testing"

	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testmonkit"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/pgtest"
)

func TestRun(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1, MultinodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		t.Log("running test")
	})
}

func BenchmarkRun_Satellite(b *testing.B) {
	benchmarkRunConfig(b, testplanet.Config{SatelliteCount: 1})
}

func BenchmarkRun_StorageNode(b *testing.B) {
	benchmarkRunConfig(b, testplanet.Config{StorageNodeCount: 4})
}

func BenchmarkRun_Planet(b *testing.B) {
	benchmarkRunConfig(b, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	})
}
func benchmarkRunConfig(b *testing.B, config testplanet.Config) {
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

			for i := 0; i < b.N; i++ {
				func() {
					planetConfig := config
					if planetConfig.Name == "" {
						planetConfig.Name = b.Name()
					}

					log := zap.NewNop()

					testmonkit.Run(context.Background(), b, func(parent context.Context) {
						defer pprof.SetGoroutineLabels(parent)
						parent = pprof.WithLabels(parent, pprof.Labels("test", b.Name()))

						ctx := testcontext.NewWithContextAndTimeout(parent, b, testcontext.DefaultTimeout)
						defer ctx.Cleanup()

						planet, err := testplanet.NewCustom(ctx, log, planetConfig, satelliteDB)
						if err != nil {
							b.Fatalf("%+v", err)
						}
						defer ctx.Check(planet.Shutdown)

						planet.Start(ctx)
					})
				}()
			}
		})
	}
}
