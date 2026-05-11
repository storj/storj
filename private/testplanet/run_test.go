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
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/jobqtest"
)

func TestRun(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1, MultinodeCount: 1, NonParallel: true,
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
	for _, satelliteDB := range testplanet.DatabasesForConfig(b, config) {
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

					jobqtest.WithServer(b, &jobqtest.ServerOptions{
						Host:    planetConfig.Host,
						Timeout: planetConfig.Timeout,
					}, func(ctx *testcontext.Context, srv *jobqtest.TestServer) {
						reconfig := func(log *zap.Logger, index int, config *satellite.Config) {
							config.JobQueue = jobq.Config{
								ServerNodeURL: srv.NodeURL,
								TLS:           srv.TLSOpts.Config,
							}
						}
						if planetConfig.Reconfigure.Satellite == nil {
							planetConfig.Reconfigure.Satellite = reconfig
						} else {
							planetConfig.Reconfigure.Satellite = testplanet.Combine(planetConfig.Reconfigure.Satellite, reconfig)
						}

						testmonkit.Run(ctx, b, func(parent context.Context) {
							defer pprof.SetGoroutineLabels(parent)
							parent = pprof.WithLabels(parent, pprof.Labels("test", b.Name()))

							innerCtx := testcontext.NewWithContextAndTimeout(parent, b, testcontext.DefaultTimeout)
							defer innerCtx.Cleanup()

							planet, err := testplanet.NewCustom(innerCtx, log, planetConfig, satelliteDB)
							if err != nil {
								b.Fatalf("%+v", err)
							}
							defer innerCtx.Check(planet.Shutdown)

							if err := planet.Start(innerCtx); err != nil {
								b.Fatalf("planet failed to start: %+v", err)
							}
						})
					})
				}()
			}
		})
	}
}
