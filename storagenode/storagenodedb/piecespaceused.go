// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

// ErrPieceSpaceUsed represents errors from the piece spaced used database.
var ErrPieceSpaceUsed = errs.Class("piece space used error")

// PieceSpaceUsedDBName represents the database name.
const PieceSpaceUsedDBName = "piece_spaced_used"

// trashTotalRowName is the special "satellite_id" used in the db to represent
// the total stored for trash. Similar to how we use NULL as a special value
// for satellite_id to represent the total for pieces, this value is used to
// identify the row storing the total for trash.
//
// It is intentionally an otherwise-invalid satellite_id (not 32 bytes) so that
// it cannot conflict with real satellite_id names.
const trashTotalRowName = "trashtotal"

type pieceSpaceUsedDB struct {
	dbContainerImpl
}

// Init creates the total pieces and total trash records if they don't already exist.
func (db *pieceSpaceUsedDB) Init(ctx context.Context) (err error) {
	totalPiecesRow := db.QueryRowContext(ctx, `
		SELECT total
		FROM piece_space_used
		WHERE satellite_id IS NULL
			AND satellite_id IS NOT ?;
	`, trashTotalRowName)

	var piecesTotal int64
	err = totalPiecesRow.Scan(&piecesTotal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = db.createInitTotalPieces(ctx)
			if err != nil {
				return ErrPieceSpaceUsed.Wrap(err)
			}
		}
	}

	totalTrashRow := db.QueryRowContext(ctx, `
		SELECT total
		FROM piece_space_used
		WHERE satellite_id = ?;
	`, trashTotalRowName)

	var trashTotal int64
	err = totalTrashRow.Scan(&trashTotal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = db.createInitTotalTrash(ctx)
			if err != nil {
				return ErrPieceSpaceUsed.Wrap(err)
			}
		}
	}

	return ErrPieceSpaceUsed.Wrap(err)
}

func (db *pieceSpaceUsedDB) createInitTotalPieces(ctx context.Context) (err error) {
	_, err = db.ExecContext(ctx, `
		INSERT INTO piece_space_used (total, content_size) VALUES (0, 0)
	`)
	return ErrPieceSpaceUsed.Wrap(err)
}

func (db *pieceSpaceUsedDB) createInitTotalTrash(ctx context.Context) (err error) {
	_, err = db.ExecContext(ctx, `
		INSERT INTO piece_space_used (total, content_size, satellite_id) VALUES (0, 0, ?)
	`, trashTotalRowName)
	return ErrPieceSpaceUsed.Wrap(err)
}

// GetPieceTotal returns the total space used (total and contentSize) for all pieces stored.
func (db *pieceSpaceUsedDB) GetPieceTotals(ctx context.Context) (total int64, contentSize int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row := db.QueryRowContext(ctx, `
		SELECT total, content_size
		FROM piece_space_used
		WHERE satellite_id IS NULL;
	`)

	err = row.Scan(&total, &contentSize)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, ErrPieceSpaceUsed.Wrap(err)
	}
	return total, contentSize, nil
}

// GetTrashTotal returns the total space used by all trash.
func (db *pieceSpaceUsedDB) GetTrashTotal(ctx context.Context) (total int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row := db.QueryRowContext(ctx, `
		SELECT total
		FROM piece_space_used
		WHERE satellite_id = ?
	`, trashTotalRowName)

	err = row.Scan(&total)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return total, nil
		}
		return total, ErrPieceSpaceUsed.Wrap(err)
	}
	return total, nil
}

// GetPieceTotalsForAllSatellites returns how much space used by pieces stored for each satelliteID.
func (db *pieceSpaceUsedDB) GetPieceTotalsForAllSatellites(ctx context.Context) (_ map[storj.NodeID]pieces.SatelliteUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT total, content_size, satellite_id
		FROM piece_space_used
		WHERE satellite_id IS NOT NULL
			AND satellite_id IS NOT ?
	`, trashTotalRowName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, ErrPieceSpaceUsed.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totalBySatellite := map[storj.NodeID]pieces.SatelliteUsage{}
	for rows.Next() {
		var total, contentSize int64
		var satelliteID storj.NodeID

		err = rows.Scan(&total, &contentSize, &satelliteID)
		if err != nil {
			return nil, ErrPieceSpaceUsed.Wrap(err)
		}
		totalBySatellite[satelliteID] = pieces.SatelliteUsage{
			Total:       total,
			ContentSize: contentSize,
		}
	}
	return totalBySatellite, rows.Err()
}

// UpdatePieceTotals updates the record for total spaced used with new total and contentSize values.
func (db *pieceSpaceUsedDB) UpdatePieceTotals(ctx context.Context, newTotal, newContentSize int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_space_used
		SET total = ?, content_size = ?
		WHERE satellite_id IS NULL
	`, newTotal, newContentSize)

	return ErrPieceSpaceUsed.Wrap(err)
}

// UpdateTrashTotal updates the record for total spaced used with a new value.
func (db *pieceSpaceUsedDB) UpdateTrashTotal(ctx context.Context, newTotal int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_space_used
		SET total = ?
		WHERE satellite_id = ?
	`, newTotal, trashTotalRowName)

	return ErrPieceSpaceUsed.Wrap(err)
}

// UpdatePieceTotalsForAllSatellites updates each record with new values for each satelliteID.
func (db *pieceSpaceUsedDB) UpdatePieceTotalsForAllSatellites(ctx context.Context, newTotalsBySatellites map[storj.NodeID]pieces.SatelliteUsage) (err error) {
	defer mon.Task()(&ctx)(&err)

	for satelliteID, vals := range newTotalsBySatellites {
		if vals.ContentSize == 0 && vals.Total == 0 {
			if err := db.deleteTotalBySatellite(ctx, satelliteID); err != nil {
				return ErrPieceSpaceUsed.Wrap(err)
			}
			continue
		}

		_, err = db.ExecContext(ctx, `
			INSERT INTO piece_space_used (total, content_size, satellite_id)
			VALUES (?, ?, ?)
			ON CONFLICT (satellite_id)
			DO UPDATE SET total = ?, content_size = ?
			WHERE satellite_id = ?
		`, vals.Total, vals.ContentSize, satelliteID, vals.Total, vals.ContentSize, satelliteID)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return ErrPieceSpaceUsed.Wrap(err)
		}
	}
	return nil
}

func (db *pieceSpaceUsedDB) deleteTotalBySatellite(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		DELETE FROM piece_space_used
		WHERE satellite_id = ?
	`, satelliteID)
	if err != nil {
		return err
	}

	return nil
}
