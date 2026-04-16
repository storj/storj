// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/modular"
)

// Migrate is a subcommand to run database migrations.
type Migrate struct {
	Log        *zap.Logger
	DB         satellite.DB
	MetabaseDB *metabase.DB
	Stop       *modular.StopTrigger
}

// NewMigrate creates a new Migrate command.
func NewMigrate(log *zap.Logger, db satellite.DB, metabaseDB *metabase.DB, stop *modular.StopTrigger) *Migrate {
	return &Migrate{
		Log:        log,
		DB:         db,
		MetabaseDB: metabaseDB,
		Stop:       stop,
	}
}

// Run executes the satellite database migration.
func (m *Migrate) Run(ctx context.Context) error {
	m.Log.Info("Running satellite database migration")
	err := m.DB.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating tables for master database on satellite: %+v", err)
	}

	m.Log.Info("Running metabase database migration")
	err = m.MetabaseDB.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating metabase tables: %+v", err)
	}

	m.Stop.Cancel()
	return nil
}
