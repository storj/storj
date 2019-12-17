// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage/redis/redisserver"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.MasterDB.Name, func(t *testing.T) {
			t.Parallel()

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}

			addr, cleanup, err := redisserver.Mini()
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()

			planetConfig := config
			reconfigSat := planetConfig.Reconfigure.Satellite

			planetConfig.Reconfigure.Satellite = func(log *zap.Logger, index int, config *satellite.Config) {
				config.LiveAccounting.StorageBackend = "redis://" + addr + "?db=0"
				if reconfigSat != nil {
					reconfigSat(log, index, config)
				}
			}

			planetConfig.Reconfigure.NewSatelliteDB = func(log *zap.Logger, index int) (satellite.DB, error) {
				return satellitedbtest.CreateMasterDB(t, "S", index, satelliteDB.MasterDB)
			}

			if satelliteDB.PointerDB.URL != "" {
				planetConfig.Reconfigure.NewSatellitePointerDB = func(log *zap.Logger, index int) (metainfo.PointerDB, error) {
					return satellitedbtest.CreatePointerDB(t, "P", index, satelliteDB.PointerDB)
				}
			}

			planet, err := NewCustom(zaptest.NewLogger(t), planetConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer ctx.Check(planet.Shutdown)

			planet.Start(ctx)

			test(t, ctx, planet)
		})
	}
}
