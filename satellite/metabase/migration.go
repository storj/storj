// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/migration"
)

// MigrateMetainfoDB migrates metabase database.
func MigrateMetainfoDB(ctx context.Context, log *zap.Logger, db *DB, migrationType string) (err error) {
	for _, migrationType := range strings.Split(migrationType, ",") {
		migrationType = strings.TrimSpace(migrationType)
		if migrationType == "" {
			continue
		}
		switch migrationType {
		case migration.FullMigration:
			err = db.MigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case migration.SnapshotMigration:
			log.Info("MigrationUnsafe using latest snapshot. It's not for production", zap.String("db", "master"))
			err = db.TestMigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case migration.NoMigration, migration.TestDataCreation:
		// noop
		default:
			return errs.New("unsupported migration type: %s, please try one of the: %s", migrationType, strings.Join(migration.MigrationTypes, ","))
		}
	}
	return err
}
