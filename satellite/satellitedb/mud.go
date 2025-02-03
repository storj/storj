// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite"
	"storj.io/storj/shared/lrucache"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[satellite.DB](ball, OpenDBWithMigration)
	config.RegisterConfig[DatabaseOptions](ball, "database-options")

}

// DatabaseOptions are the configurations for satellitedb.
type DatabaseOptions struct {
	URL          string `help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`
	APIKeysCache struct {
		Expiration time.Duration `help:"satellite database api key expiration" default:"60s"`
		Capacity   int           `help:"satellite database api key lru capacity" default:"10000"`
	}
	RevocationsCache struct {
		Expiration time.Duration `help:"macaroon revocation cache expiration" default:"5m"`
		Capacity   int           `help:"macaroon revocation cache capacity" default:"10000"`
	}
	MigrationUnsafe string `help:"comma separated migration types to run during every startup (none: no migration, snapshot: creating db from latest test snapshot (for testing only), testdata: create testuser in addition to a migration, full: do the normal migration (equals to 'satellite run migration'" default:"none" hidden:"true"`
}

// OpenDBWithMigration is a wrapper for opening database and do optional migration.
func OpenDBWithMigration(ctx context.Context, logger *zap.Logger, cfg DatabaseOptions) (satellite.DB, error) {
	db, err := Open(ctx, logger, cfg.URL, Options{
		// TODO: use correct application name
		ApplicationName: "satellite",
		APIKeysLRUOptions: lrucache.Options{
			Expiration: cfg.APIKeysCache.Expiration,
			Capacity:   cfg.APIKeysCache.Capacity,
		},
		RevocationLRUOptions: lrucache.Options{
			Expiration: cfg.RevocationsCache.Expiration,
			Capacity:   cfg.RevocationsCache.Capacity,
		},
	})

	if err != nil {
		return nil, errs.New("Error starting master database on satellite api: %+v", err)
	}
	err = MigrateSatelliteDB(ctx, logger, db, cfg.MigrationUnsafe)
	if err != nil {
		return nil, err
	}
	return db, err
}
