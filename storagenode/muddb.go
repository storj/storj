// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode

import (
	"context"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/storagenodedb"
)

// DBModule is a mud Module, defining how to get DB components.
func DBModule(ball *mud.Ball) {
	mud.Provide[storagenodedb.Config](ball, newDbConfig)
	mud.Provide[DB](ball, NewDB)
	mud.View(ball, DB.Notifications)
	mud.View(ball, DB.Reputation)
	mud.View(ball, DB.Satellites)
	mud.View[DB, *storagenodedb.BandwidthDB](ball, func(db DB) *storagenodedb.BandwidthDB {
		return db.Bandwidth().(*storagenodedb.BandwidthDB)
	})
	mud.RegisterInterfaceImplementation[bandwidth.Writer, *storagenodedb.BandwidthDB](ball)
	mud.RegisterInterfaceImplementation[bandwidth.DB, *storagenodedb.BandwidthDB](ball)
	mud.View(ball, DB.Orders)
	mud.View(ball, DB.Payout)
	mud.View(ball, DB.Pricing)
	mud.View(ball, DB.GCFilewalkerProgress)
	mud.View(ball, DB.V0PieceInfo)
	mud.View(ball, DB.PieceSpaceUsedDB)
	mud.View(ball, DB.StorageUsage)
	mud.View(ball, DB.UsedSpacePerPrefix)
}

// NewDB will create (+ test + migrate) new DB instance for storagenodes.
func NewDB(ctx context.Context, log *zap.Logger, config storagenodedb.Config) (DB, error) {
	db, err := storagenodedb.OpenExisting(ctx, log, config)
	if err != nil {
		return nil, errs.New("Error opening database on storagenode: %+v", err)
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return nil, errs.New("Error migrating tables for database on storagenode: %+v", err)
	}

	err = db.CheckVersion(ctx)
	if err != nil {
		return nil, errs.New("Error checking version for storagenode database: %+v", err)
	}

	err = db.Preflight(ctx)
	if err != nil {
		return nil, errs.New("Error during preflight check for storagenode databases: %+v", err)
	}
	return db, nil
}

func newDbConfig(old piecestore.OldConfig, pss piecestore.Config, ps pieces.Config, fs filestore.Config) storagenodedb.Config {
	dbdir := pss.DatabaseDir
	if dbdir == "" {
		dbdir = old.Path
	}
	return storagenodedb.Config{
		Storage:   old.Path,
		Cache:     ps.FileStatCache,
		Info:      filepath.Join(dbdir, "piecestore.db"),
		Info2:     filepath.Join(dbdir, "info.db"),
		Pieces:    old.Path,
		Filestore: fs,
	}
}
