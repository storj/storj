// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

// ErrPieceExpiration represents errors from the piece expiration database.
var ErrPieceExpiration = errs.Class("piece expiration error")

// PieceExpirationDBName represents the database filename.
const PieceExpirationDBName = "piece_expiration"

type pieceExpirationDB struct {
	dbContainerImpl
}

// GetExpired gets piece IDs that expire or have expired before the given time
func (db *pieceExpirationDB) GetExpired(ctx context.Context, expiresBefore time.Time, limit int64) (expiredPieceIDs []pieces.ExpiredInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT satellite_id, piece_id
			FROM piece_expirations
			WHERE piece_expiration < ?
				AND ((deletion_failed_at IS NULL) OR deletion_failed_at <> ?)
				AND trash = 0
			LIMIT ?
	`, expiresBefore.UTC(), expiresBefore.UTC(), limit)
	if err != nil {
		return nil, ErrPieceExpiration.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var satelliteID storj.NodeID
		var pieceID storj.PieceID
		err = rows.Scan(&satelliteID, &pieceID)
		if err != nil {
			return nil, ErrPieceExpiration.Wrap(err)
		}
		expiredPieceIDs = append(expiredPieceIDs, pieces.ExpiredInfo{
			SatelliteID: satelliteID,
			PieceID:     pieceID,
			InPieceInfo: false,
		})
	}
	return expiredPieceIDs, nil
}

// SetExpiration sets an expiration time for the given piece ID on the given satellite
func (db *pieceExpirationDB) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO piece_expirations(satellite_id, piece_id, piece_expiration)
			VALUES (?,?,?)
	`, satellite, pieceID, expiresAt.UTC())
	return ErrPieceExpiration.Wrap(err)
}

// DeleteExpiration removes an expiration record for the given piece ID on the given satellite
func (db *pieceExpirationDB) DeleteExpiration(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (found bool, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := db.ExecContext(ctx, `
		DELETE FROM piece_expirations
			WHERE satellite_id = ? AND piece_id = ?
	`, satelliteID, pieceID)
	if err != nil {
		return false, err
	}
	numRows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return numRows > 0, nil
}

// DeleteFailed marks an expiration record as having experienced a failure in deleting the piece
// from the disk
func (db *pieceExpirationDB) DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, when time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_expirations
			SET deletion_failed_at = ?
			WHERE satellite_id = ?
				AND piece_id = ?
	`, when.UTC(), satelliteID, pieceID)
	return ErrPieceExpiration.Wrap(err)
}

// Trash marks a piece expiration as "trashed"
func (db *pieceExpirationDB) Trash(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_expirations
			SET trash = 1
			WHERE satellite_id = ?
				AND piece_id = ?
	`, satelliteID, pieceID)
	return ErrPieceExpiration.Wrap(err)
}

// Restore restores all trashed pieces
func (db *pieceExpirationDB) RestoreTrash(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_expirations
			SET trash = 0
			WHERE satellite_id = ?
				AND trash = 1
	`, satelliteID)
	return ErrPieceExpiration.Wrap(err)
}
