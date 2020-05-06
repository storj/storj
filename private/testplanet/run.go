// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	databases := satellitedbtest.Databases()
	hasDatabase := false
	for _, db := range databases {
		hasDatabase = hasDatabase || db.MasterDB.URL != ""
	}
	if !hasDatabase {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + pgtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + pgtest.DefaultCockroach)
	}

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			parallel := !config.NonParallel
			if parallel {
				t.Parallel()
			}

			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}

			planetConfig := config
			if planetConfig.Name == "" {
				planetConfig.Name = t.Name()
			}

			planet, err := NewCustom(zaptest.NewLogger(t), config, satelliteDB)
			if err != nil {
				t.Fatalf("%+v", err)
			}
			defer ctx.Check(planet.Shutdown)

			planet.Start(ctx)

			test(t, ctx, planet)
		})
	}
}
