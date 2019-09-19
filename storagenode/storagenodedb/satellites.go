// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/satellites"
)

// ErrSatellitesDB represents errors from the satellites database.
var ErrSatellitesDB = errs.Class("satellitesdb error")

// reputation works with node reputation DB
type satellitesDB struct {
	location string
	SQLDB
}

// newSatellitesDB returns a new instance of satellitesDB initialized with the specified database.
func newSatellitesDB(db SQLDB, location string) *satellitesDB {
	return &satellitesDB{
		location: location,
		SQLDB:    db,
	}
}

// initiate graceful exit
func (db *satellitesDB) InitiateGracefulExit(ctx context.Context, satelliteID storj.NodeID, intitiatedAt time.Time, startingDiskUsage int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	return ErrSatellitesDB.Wrap(withTx(ctx, db.SQLDB, func(tx *sql.Tx) error {
		query := `INSERT OR REPLACE INTO satellites (node_id, status, added_at) VALUES (?,?, COALESCE((SELECT added_at FROM satellites WHERE node_id = ?), ?))`
		_, err = tx.ExecContext(ctx, query, satelliteID, satellites.Exiting, satelliteID, intitiatedAt.UTC()) // assume intitiatedAt < time.Now()
		if err != nil {
			return err
		}
		query = `INSERT INTO satellite_exit_progress (satellite_id, initiated_at, starting_disk_usage, bytes_deleted) VALUES (?,?,?,0)`
		_, err = tx.ExecContext(ctx, query, satelliteID, intitiatedAt.UTC(), startingDiskUsage)
		return err
	}))
}

// increment graceful exit bytes deleted
func (db *satellitesDB) UpdateGracefulExit(ctx context.Context, satelliteID storj.NodeID, bytesDeleted int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	query := `UPDATE satellite_exit_progress SET bytes_deleted = bytes_deleted + ? WHERE satellite_id = ?`
	_, err = db.ExecContext(ctx, query, bytesDeleted, satelliteID)
	return ErrSatellitesDB.Wrap(err)
}

// complete graceful exit
func (db *satellitesDB) CompleteGracefulExit(ctx context.Context, satelliteID storj.NodeID, finishedAt time.Time, exitStatus satellites.Status, completionReceipt []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return ErrSatellitesDB.Wrap(withTx(ctx, db.SQLDB, func(tx *sql.Tx) error {
		query := `UPDATE satellites SET status = ? WHERE node_id = ?`
		_, err = tx.ExecContext(ctx, query, satelliteID, exitStatus)
		if err != nil {
			return err
		}
		query = `UPDATE satellite_exit_progress SET finished_at = ?, completion_receipt = ? WHERE satellite_id = ?`
		_, err = tx.ExecContext(ctx, query, finishedAt.UTC(), completionReceipt, satelliteID)
		return err
	}))
}
