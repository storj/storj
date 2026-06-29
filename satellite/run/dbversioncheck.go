// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/migration"
	"storj.io/storj/satellite/satellitedb"
)

// DBVersionCheck validates that the satellite and metabase databases are at the
// schema version expected by this binary. It is wired into the long-running
// satellite subcommands (api, core, ...) but intentionally not into the migrate
// subcommand, which is responsible for bringing the schema up to date.
//
// The check runs while the component is being initialized (before any service
// starts), so a schema mismatch fails the process fast, mirroring the
// checkDBVersions step of the non-modular satellite.
type DBVersionCheck struct {
}

// NewDBVersionCheck validates the satellite and metabase database versions.
//
// Each database is gated by its own migration flag (the satellite and metabase
// MigrationUnsafe settings are independent): validation is skipped when a full
// or snapshot migration was just applied to that database (see
// migration.ShouldValidateVersion), since those bring the schema to a known
// state. For the default no-migration case (production, where migrations are
// applied separately via the migrate subcommand) the version is validated.
func NewDBVersionCheck(ctx context.Context, log *zap.Logger, db satellite.DB, metabaseDB *metabase.DB, dbOpts *satellitedb.DatabaseOptions, metabaseCfg metabase.DatabaseConfig) (*DBVersionCheck, error) {
	if migration.ShouldValidateVersion(metabaseCfg.MigrationUnsafe) {
		if err := metabaseDB.CheckVersion(ctx); err != nil {
			log.Error("Failed metabase database version check.", zap.Error(err))
			return nil, errs.New("failed metabase version check: %+v", err)
		}
	}

	if migration.ShouldValidateVersion(dbOpts.MigrationUnsafe) {
		if err := db.CheckVersion(ctx); err != nil {
			log.Error("Failed satellite database version check.", zap.Error(err))
			return nil, errs.New("failed satellite version check: %+v", err)
		}
	}

	return &DBVersionCheck{}, nil
}
