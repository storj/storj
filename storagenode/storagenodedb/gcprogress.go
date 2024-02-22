// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

// ErrGCProgress represents errors from the filewalker database.
var ErrGCProgress = errs.Class("gc_filewalker_progress_db")

// GCFilewalkerProgressDBName represents the database name.
const GCFilewalkerProgressDBName = "garbage_collection_filewalker_progress"

type gcFilewalkerProgressDB struct {
	dbContainerImpl
}

func (db *gcFilewalkerProgressDB) Store(ctx context.Context, progress pieces.GCFilewalkerProgress) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		INSERT OR REPLACE INTO progress(satellite_id, bloomfilter_created_before, last_checked_prefix)
		VALUES(?,?,?)
	`, progress.SatelliteID, progress.BloomfilterCreatedBefore.UTC(), progress.Prefix)

	return ErrGCProgress.Wrap(err)
}

func (db *gcFilewalkerProgressDB) Get(ctx context.Context, satelliteID storj.NodeID) (progress pieces.GCFilewalkerProgress, err error) {
	defer mon.Task()(&ctx)(&err)

	err = db.QueryRowContext(ctx, `
		SELECT last_checked_prefix, bloomfilter_created_before
		FROM progress
		WHERE satellite_id = ?
	`, satelliteID).Scan(&progress.Prefix, &progress.BloomfilterCreatedBefore)

	progress.SatelliteID = satelliteID

	return progress, ErrGCProgress.Wrap(err)
}

func (db *gcFilewalkerProgressDB) Reset(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		DELETE FROM progress
		WHERE satellite_id = ?
	`, satelliteID)

	return ErrGCProgress.Wrap(err)
}
