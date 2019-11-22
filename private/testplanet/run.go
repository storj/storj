// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/dbutil/pgtest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/uplink"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	databases := satellitedbtest.Databases()
	hasDatabase := false
	for _, db := range databases {
		hasDatabase = hasDatabase || (db.MasterDB.URL != "" && db.MasterDB.URL != "omit")
	}
	if !hasDatabase {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + pgtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + pgtest.DefaultCockroach)
	}

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		if strings.EqualFold(satelliteDB.MasterDB.URL, "omit") {
			continue
		}
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
			planetConfig.Reconfigure.NewSatelliteDB = func(log *zap.Logger, index int) (satellite.DB, error) {
				var schema string
				db, err := satellitedb.New(log, satelliteDB.MasterDB.URL)

				if satelliteDB.MasterDB.Name == "Postgres" {
					schema = satellitedbtest.SchemaName(t.Name(), "S", index, schemaSuffix)
					db, err = satellitedb.New(log, pgutil.ConnstrWithSchema(satelliteDB.MasterDB.URL, schema))
					if err != nil {
						t.Fatal(err)
					}
					return &satellitedbtest.SchemaDB{
						DB:       db,
						Schema:   schema,
						AutoDrop: true,
					}, nil
				}

				return db, err
			}

			planet, err := NewCustom(zaptest.NewLogger(t), planetConfig, satelliteDB)
			if err != nil {
				t.Fatalf("%+v", err)
			}
			defer ctx.Check(planet.Shutdown)

			planet.Start(ctx)

			provisionUplinks(ctx, t, planet)

			test(t, ctx, planet)
		})
	}
}

func provisionUplinks(ctx context.Context, t *testing.T, planet *Planet) {
	for _, planetUplink := range planet.Uplinks {
		for _, satellite := range planet.Satellites {
			apiKey := planetUplink.APIKey[satellite.ID()]
			access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.URL(), apiKey.Serialize(), "")
			if err != nil {
				t.Fatalf("%+v", err)
			}
			planetUplink.Access[satellite.ID()] = access
		}
	}
}
