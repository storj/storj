// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
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
// it cannot conflict with real satellite_id names
const trashTotalRowName = "trashtotal"

type pieceSpaceUsedDB struct {
	dbContainerImpl
}

// Init creates the total pieces and total trash records if they don't already exist
func (db *pieceSpaceUsedDB) Init(ctx context.Context) (err error) {
	totalPiecesRow := db.QueryRow(`
		SELECT total
		FROM piece_space_used
		WHERE satellite_id IS NULL
			AND satellite_id IS NOT ?;
	`, trashTotalRowName)

	var piecesTotal int64
	err = totalPiecesRow.Scan(&piecesTotal)
	if err != nil {
		if err == sql.ErrNoRows {
			err = db.createInitTotalPieces(ctx)
			if err != nil {
				return ErrPieceSpaceUsed.Wrap(err)
			}
		}
	}

	totalTrashRow := db.QueryRow(`
		SELECT total
		FROM piece_space_used
		WHERE satellite_id = ?;
	`, trashTotalRowName)

	var trashTotal int64
	err = totalTrashRow.Scan(&trashTotal)
	if err != nil {
		if err == sql.ErrNoRows {
			err = db.createInitTotalTrash(ctx)
			if err != nil {
				return ErrPieceSpaceUsed.Wrap(err)
			}
		}
	}

	return ErrPieceSpaceUsed.Wrap(err)
}

func (db *pieceSpaceUsedDB) createInitTotalPieces(ctx context.Context) (err error) {
	_, err = db.Exec(`
		INSERT INTO piece_space_used (total) VALUES (0)
	`)
	return ErrPieceSpaceUsed.Wrap(err)
}

func (db *pieceSpaceUsedDB) createInitTotalTrash(ctx context.Context) (err error) {
	_, err = db.Exec(`
		INSERT INTO piece_space_used (total, satellite_id) VALUES (0, ?)
	`, trashTotalRowName)
	return ErrPieceSpaceUsed.Wrap(err)
}

// GetPieceTotal returns the total space used by all pieces stored on disk
func (db *pieceSpaceUsedDB) GetPieceTotal(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row := db.QueryRow(`
		SELECT total
		FROM piece_space_used
		WHERE satellite_id IS NULL;
	`)

	var total int64
	err = row.Scan(&total)
	if err != nil {
		if err == sql.ErrNoRows {
			return total, nil
		}
		return total, ErrPieceSpaceUsed.Wrap(err)
	}
	return total, nil
}

// GetTrashTotal returns the total space used by all trash
func (db *pieceSpaceUsedDB) GetTrashTotal(ctx context.Context) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	row := db.QueryRow(`
		SELECT total
		FROM piece_space_used
		WHERE satellite_id = ?
	`, trashTotalRowName)

	var total int64
	err = row.Scan(&total)
	if err != nil {
		if err == sql.ErrNoRows {
			return total, nil
		}
		return total, ErrPieceSpaceUsed.Wrap(err)
	}
	return total, nil
}

// GetPieceTotalsForAllSatellites returns how much total space used by pieces stored on disk for each satelliteID
func (db *pieceSpaceUsedDB) GetPieceTotalsForAllSatellites(ctx context.Context) (_ map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT total, satellite_id
		FROM piece_space_used
		WHERE satellite_id IS NOT NULL
			AND satellite_id IS NOT ?
	`, trashTotalRowName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, ErrPieceSpaceUsed.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	totalBySatellite := map[storj.NodeID]int64{}
	for rows.Next() {
		var total int64
		var satelliteID storj.NodeID

		err = rows.Scan(&total, &satelliteID)
		if err != nil {
			return nil, ErrPieceSpaceUsed.Wrap(err)
		}
		totalBySatellite[satelliteID] = total
	}
	return totalBySatellite, nil
}

// UpdatePieceTotal updates the record for total spaced used with a new value
func (db *pieceSpaceUsedDB) UpdatePieceTotal(ctx context.Context, newTotal int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_space_used
		SET total = ?
		WHERE satellite_id IS NULL
	`, newTotal)

	return ErrPieceSpaceUsed.Wrap(err)
}

// UpdateTrashTotal updates the record for total spaced used with a new value
func (db *pieceSpaceUsedDB) UpdateTrashTotal(ctx context.Context, newTotal int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `
		UPDATE piece_space_used
		SET total = ?
		WHERE satellite_id = ?
	`, newTotal, trashTotalRowName)

	return ErrPieceSpaceUsed.Wrap(err)
}

// UpdatePieceTotalsForAllSatellites updates each record for total spaced used with a new value for each satelliteID
func (db *pieceSpaceUsedDB) UpdatePieceTotalsForAllSatellites(ctx context.Context, newTotalsBySatellites map[storj.NodeID]int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	for satelliteID, newTotal := range newTotalsBySatellites {
		if newTotal == 0 {
			if err := db.deleteTotalBySatellite(ctx, satelliteID); err != nil {
				return ErrPieceSpaceUsed.Wrap(err)
			}
			continue
		}

		_, err = db.ExecContext(ctx, `
			INSERT INTO piece_space_used (total, satellite_id)
			VALUES (?, ?)
			ON CONFLICT (satellite_id)
			DO UPDATE SET total = ?
			WHERE satellite_id = ?
		`, newTotal, satelliteID, newTotal, satelliteID)

		if err != nil {
			if err == sql.ErrNoRows {
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
