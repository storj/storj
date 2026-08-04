// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.View[*DB, DB](ball, mud.Dereference[DB])
	// TODO: there are cases when we need all the adapters (like changefeed). We need a better way to configure which should be used for these usescases.
	mud.View[*DB, Adapter](ball, func(db *DB) Adapter {
		for _, v := range db.adapters {
			return v
		}
		panic("no adapters found")
	})

}

// DatabaseConfig is the minimum required configuration for metabase.
type DatabaseConfig struct {
	MigrationUnsafe string `help:"comma separated migration types to run during every startup (none: no migration, snapshot: creating db from latest test snapshot (for testing only), testdata: create testuser in addition to a migration, full: do the normal migration (equals to 'satellite run migration'" default:"none" hidden:"true"`
	URL             string
	Config
}

// OpenDatabaseWithMigration will open the database (and update schema, if required).
func OpenDatabaseWithMigration(ctx context.Context, logger *zap.Logger, cfg DatabaseConfig) (*DB, error) {
	metabaseDB, err := Open(ctx, logger, cfg.URL, cfg.Config)
	if err != nil {
		return nil, errs.New("Error creating metabase connection on satellite api: %+v", err)
	}

	err = MigrateMetainfoDB(ctx, logger, metabaseDB, cfg.MigrationUnsafe)
	if err != nil {
		return nil, err
	}
	return metabaseDB, err
}
