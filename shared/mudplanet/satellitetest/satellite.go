// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitetest

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting/live"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	trustmud "storj.io/storj/satellite/trust/mud"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mudplanet"
)

// Satellite is a configuration. For db support, Wrap it with WithDB.
var Satellite = mudplanet.Customization{
	Modules: mudplanet.Modules{
		dbModule,
		satellite.Module,
		trustmud.Module,
	},
	PreInit: []any{
		func(options *live.Config) {
			options.StorageBackend = "noop://"
		},
		func(options *orders.Config) error {
			key, err := orders.NewEncryptionKeys(orders.EncryptionKey{
				ID:  orders.EncryptionKeyID{1},
				Key: storj.Key{1},
			})
			if err != nil {
				return err
			}
			options.EncryptionKeys = *key
			return nil
		},
	},
}

// WithoutDB is a configuration for running satellite without database support, but the SatelliteDatabases is required by the dependency graph.
func WithoutDB(ball *mud.Ball) {
	mud.Supply[satellitedbtest.SatelliteDatabases](ball, satellitedbtest.SatelliteDatabases{})
}

// WithDB is a configuration for running satellite with database support.
func WithDB(components ...mudplanet.Component) mudplanet.Config {
	return mudplanet.Config{
		Components: components,
		RunWrapper: runWithDatabases,
	}
}

func dbModule(ball *mud.Ball) {
	mud.Provide[satellite.DB](ball, func(ctx context.Context, log *zap.Logger, database satellitedbtest.SatelliteDatabases) (satellite.DB, error) {
		db, err := satellitedbtest.CreateMasterDB(ctx, log.Named("db"), "satellite", "S", 1, database.MasterDB, satellitedb.Options{
			ApplicationName: "mudplanet",
		})
		if err != nil {
			return nil, err
		}
		err = satellitedb.MigrateSatelliteDB(ctx, log, db, "snapshot,testdata")
		return db, err
	})
	mud.Provide[*metabase.DB](ball, func(ctx context.Context, log *zap.Logger, database satellitedbtest.SatelliteDatabases) (*metabase.DB, error) {
		db, err := satellitedbtest.CreateMetabaseDB(ctx, log.Named("metabase"), "metabase", "M", 1, database.MetabaseDB, metabase.Config{
			ApplicationName:  "mudplanet",
			MaxNumberOfParts: 100,
		})
		if err != nil {
			return nil, err
		}
		err = db.TestMigrateToLatest(ctx)
		return db, err
	})
}

func runWithDatabases(t *testing.T, fn func(t *testing.T, module func(*mud.Ball))) {
	databases := satellitedbtest.Databases(t)
	if len(databases) == 0 {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + dbtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + dbtest.DefaultCockroach + "\n" +
			"-spanner-test-db=" + dbtest.DefaultSpanner)
	}

	for _, satelliteDB := range databases {
		t.Run(satelliteDB.Name, func(t *testing.T) {
			fn(t, func(ball *mud.Ball) {
				mud.Supply[satellitedbtest.SatelliteDatabases](ball, satelliteDB)
			})
		})
	}
}
