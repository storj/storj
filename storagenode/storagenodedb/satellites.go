// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/satellites"
)

// ErrSatellitesDB represents errors from the satellites database.
var ErrSatellitesDB = errs.Class("satellitesdb error")

// SatellitesDBName represents the database name.
const SatellitesDBName = "satellites"

// reputation works with node reputation DB
type satellitesDB struct {
	dbContainerImpl
}

// GetSatellite retrieves that satellite by ID
func (db *satellitesDB) GetSatellite(ctx context.Context, satelliteID storj.NodeID) (satellite satellites.Satellite, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, "SELECT node_id, added_at, status from satellites where node_id = ?", satelliteID)
	if err != nil {
		return satellite, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	if rows.Next() {
		err := rows.Scan(&satellite.SatelliteID, &satellite.AddedAt, &satellite.Status)
		if err != nil {
			return satellite, err
		}
	}
	return satellite, nil
}

// InitiateGracefulExit updates the database to reflect the beginning of a graceful exit
func (db *satellitesDB) InitiateGracefulExit(ctx context.Context, satelliteID storj.NodeID, intitiatedAt time.Time, startingDiskUsage int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	return ErrSatellitesDB.Wrap(withTx(ctx, db.GetDB(), func(tx *sql.Tx) error {
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

// UpdateGracefulExit increments the total bytes deleted during a graceful exit
func (db *satellitesDB) UpdateGracefulExit(ctx context.Context, satelliteID storj.NodeID, addToBytesDeleted int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	query := `UPDATE satellite_exit_progress SET bytes_deleted = bytes_deleted + ? WHERE satellite_id = ?`
	_, err = db.ExecContext(ctx, query, addToBytesDeleted, satelliteID)
	return ErrSatellitesDB.Wrap(err)
}

// CompleteGracefulExit updates the database when a graceful exit is completed or failed
func (db *satellitesDB) CompleteGracefulExit(ctx context.Context, satelliteID storj.NodeID, finishedAt time.Time, exitStatus satellites.Status, completionReceipt []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return ErrSatellitesDB.Wrap(withTx(ctx, db.GetDB(), func(tx *sql.Tx) error {
		query := `UPDATE satellites SET status = ? WHERE node_id = ?`
		_, err = tx.ExecContext(ctx, query, exitStatus, satelliteID)
		if err != nil {
			return err
		}
		query = `UPDATE satellite_exit_progress SET finished_at = ?, completion_receipt = ? WHERE satellite_id = ?`
		_, err = tx.ExecContext(ctx, query, finishedAt.UTC(), completionReceipt, satelliteID)
		return err
	}))
}

// ListGracefulExits lists all graceful exit records
func (db *satellitesDB) ListGracefulExits(ctx context.Context) (exitList []satellites.ExitProgress, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT satellite_id, initiated_at, finished_at, starting_disk_usage, bytes_deleted, completion_receipt FROM satellite_exit_progress`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, ErrSatellitesDB.Wrap(err)
	}
	defer func() {
		err = ErrSatellitesDB.Wrap(errs.Combine(err, rows.Close()))
	}()

	for rows.Next() {
		var exit satellites.ExitProgress
		err := rows.Scan(&exit.SatelliteID, &exit.InitiatedAt, &exit.FinishedAt, &exit.StartingDiskUsage, &exit.BytesDeleted, &exit.CompletionReceipt)
		if err != nil {
			return nil, err
		}
		exitList = append(exitList, exit)
	}

	return exitList, nil
}
