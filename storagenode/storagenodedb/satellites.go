// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/shared/dbutil/sqliteutil"
	"storj.io/storj/shared/tagsql"
	"storj.io/storj/storagenode/satellites"
)

// ErrSatellitesDB represents errors from the satellites database.
var ErrSatellitesDB = errs.Class("satellitesdb")

// SatellitesDBName represents the database name.
const SatellitesDBName = "satellites"

// reputation works with node reputation DB.
type satellitesDB struct {
	dbContainerImpl
}

// SetAddress inserts into satellite's db id, address, added time.
func (db *satellitesDB) SetAddress(ctx context.Context, satelliteID storj.NodeID, address string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO satellites (node_id, address, added_at, status) VALUES(?,?,?,?) ON CONFLICT (node_id) DO UPDATE SET address = EXCLUDED.address`,
		satelliteID,
		address,
		time.Now().UTC(),
		satellites.Normal,
	)

	return ErrSatellitesDB.Wrap(err)
}

// SetAddressAndStatus inserts into satellite's db id, address, added time and status.
func (db *satellitesDB) SetAddressAndStatus(ctx context.Context, satelliteID storj.NodeID, address string, status satellites.Status) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO satellites (node_id, address, added_at, status) VALUES(?,?,?,?) ON CONFLICT (node_id) DO UPDATE SET address = EXCLUDED.address, status = EXCLUDED.status`,
		satelliteID,
		address,
		time.Now().UTC(),
		status,
	)

	return ErrSatellitesDB.Wrap(err)
}

// GetSatellite retrieves that satellite by ID.
func (db *satellitesDB) GetSatellite(ctx context.Context, satelliteID storj.NodeID) (satellite satellites.Satellite, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, "SELECT node_id, address, added_at, status FROM satellites WHERE node_id = ?", satelliteID)
	if err != nil {
		return satellite, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var address sql.NullString
	if rows.Next() {
		err := rows.Scan(&satellite.SatelliteID, &address, &satellite.AddedAt, &satellite.Status)
		if err != nil {
			return satellite, err
		}
		satellite.Address = address.String
	}
	return satellite, rows.Err()
}

// GetSatellites retrieves all satellites.
func (db *satellitesDB) GetSatellites(ctx context.Context) (sats []satellites.Satellite, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, "SELECT node_id, address, added_at, status FROM satellites")
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var satellite satellites.Satellite
		var address sql.NullString
		err := rows.Scan(&satellite.SatelliteID, &address, &satellite.AddedAt, &satellite.Status)
		if err != nil {
			return nil, err
		}
		satellite.Address = address.String
		sats = append(sats, satellite)
	}
	return sats, rows.Err()
}

// GetSatellitesUrls retrieves all satellite's id and urls.
func (db *satellitesDB) GetSatellitesUrls(ctx context.Context) (satelliteURLs []storj.NodeURL, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT
			node_id,
			address
		FROM satellites`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	var urls []storj.NodeURL
	for rows.Next() {
		var url storj.NodeURL

		err := rows.Scan(&url.ID, &url.Address)
		if err != nil {
			return nil, ErrPayout.Wrap(err)
		}

		urls = append(urls, url)
	}
	if err = rows.Err(); err != nil {
		return nil, ErrPayout.Wrap(err)
	}

	return urls, nil
}

// UpdateSatelliteStatus updates satellite status.
func (db *satellitesDB) UpdateSatelliteStatus(ctx context.Context, satelliteID storj.NodeID, status satellites.Status) (err error) {
	return db.updateSatelliteStatus(ctx, db.DB, satelliteID, status)
}

// updateSatelliteStatus updates satellite status.
func (db *satellitesDB) updateSatelliteStatus(ctx context.Context, tx tagsql.ExecQueryer, satelliteID storj.NodeID, status satellites.Status) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = tx.ExecContext(ctx, "UPDATE satellites SET status = ? WHERE node_id = ?", status, satelliteID)
	return ErrSatellitesDB.Wrap(err)
}

// InitiateGracefulExit updates the database to reflect the beginning of a graceful exit.
func (db *satellitesDB) InitiateGracefulExit(ctx context.Context, satelliteID storj.NodeID, intitiatedAt time.Time, startingDiskUsage int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	return ErrSatellitesDB.Wrap(sqliteutil.WithTx(ctx, db.GetDB(), func(ctx context.Context, tx tagsql.Tx) error {
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

// CancelGracefulExit delete an entry by satellite ID.
func (db *satellitesDB) CancelGracefulExit(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, "DELETE FROM satellite_exit_progress WHERE satellite_id = ?", satelliteID)
	return ErrSatellitesDB.Wrap(err)
}

// UpdateGracefulExit increments the total bytes deleted during a graceful exit.
func (db *satellitesDB) UpdateGracefulExit(ctx context.Context, satelliteID storj.NodeID, addToBytesDeleted int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	query := `UPDATE satellite_exit_progress SET bytes_deleted = bytes_deleted + ? WHERE satellite_id = ?`
	_, err = db.ExecContext(ctx, query, addToBytesDeleted, satelliteID)
	return ErrSatellitesDB.Wrap(err)
}

// CompleteGracefulExit updates the database when a graceful exit is completed or failed.
func (db *satellitesDB) CompleteGracefulExit(ctx context.Context, satelliteID storj.NodeID, finishedAt time.Time, exitStatus satellites.Status, completionReceipt []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return ErrSatellitesDB.Wrap(sqliteutil.WithTx(ctx, db.GetDB(), func(ctx context.Context, tx tagsql.Tx) error {
		err := db.updateSatelliteStatus(ctx, tx, satelliteID, exitStatus)
		if err != nil {
			return err
		}
		query := `UPDATE satellite_exit_progress SET finished_at = ?, completion_receipt = ? WHERE satellite_id = ?`
		_, err = tx.ExecContext(ctx, query, finishedAt.UTC(), completionReceipt, satelliteID)
		return err
	}))
}

// ListGracefulExits lists all graceful exit records.
func (db *satellitesDB) ListGracefulExits(ctx context.Context) (exitList []satellites.ExitProgress, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT satellite_id, initiated_at, finished_at, starting_disk_usage, bytes_deleted, completion_receipt, status FROM satellite_exit_progress INNER JOIN satellites ON satellite_exit_progress.satellite_id = satellites.node_id`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, ErrSatellitesDB.Wrap(err)
	}
	defer func() {
		err = ErrSatellitesDB.Wrap(errs.Combine(err, rows.Close()))
	}()

	for rows.Next() {
		var exit satellites.ExitProgress
		err := rows.Scan(&exit.SatelliteID, &exit.InitiatedAt, &exit.FinishedAt, &exit.StartingDiskUsage, &exit.BytesDeleted, &exit.CompletionReceipt, &exit.Status)
		if err != nil {
			return nil, err
		}
		exitList = append(exitList, exit)
	}

	return exitList, rows.Err()
}

// DeleteSatellite deletes the satellite from the database.
func (db *satellitesDB) DeleteSatellite(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, "DELETE FROM satellites WHERE node_id = ?", satelliteID)
	return ErrSatellitesDB.Wrap(err)
}
