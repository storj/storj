// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
)

// ErrUsedSpacePerPrefix represents errors from the used space per prefix database.
var ErrUsedSpacePerPrefix = errs.Class("used_space_per_prefix_db")

// UsedSpacePerPrefixDBName represents the database name.
const UsedSpacePerPrefixDBName = "used_space_per_prefix"

type usedSpacePerPrefixDB struct {
	dbContainerImpl
}

// Store stores the used space for a prefix.
func (db *usedSpacePerPrefixDB) Store(ctx context.Context, usedSpace pieces.PrefixUsedSpace) (err error) {
	defer mon.Task()(&ctx)(&err)

	if usedSpace.LastUpdated.IsZero() {
		usedSpace.LastUpdated = time.Now().UTC()
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO used_space_per_prefix(satellite_id, piece_prefix, total_bytes, total_content_size, piece_counts, last_updated)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(satellite_id, piece_prefix) DO UPDATE SET total_bytes = $3, total_content_size = $4, piece_counts = $5, last_updated = $6`,
		usedSpace.SatelliteID, usedSpace.Prefix, usedSpace.TotalBytes, usedSpace.TotalContentSize, usedSpace.PieceCounts, usedSpace.LastUpdated.UTC())

	return ErrUsedSpacePerPrefix.Wrap(err)
}

var monStoreBatch = mon.Task()

// StoreBatch stores the batch of used space per prefix.
func (db *usedSpacePerPrefixDB) StoreBatch(ctx context.Context, usedSpaces []pieces.PrefixUsedSpace) (err error) {
	defer monStoreBatch(&ctx)(&err)

	var args []any
	for _, u := range usedSpaces {
		args = append(args, u.SatelliteID, u.Prefix, u.TotalBytes, u.TotalContentSize, u.PieceCounts, u.LastUpdated.UTC())
	}

	values := strings.TrimRight(strings.Repeat("(?,?,?,?,?,?),", len(args)/6), ",")

	_, err = db.ExecContext(ctx, `
       INSERT INTO used_space_per_prefix(satellite_id, piece_prefix, total_bytes, total_content_size, piece_counts, last_updated)
       VALUES `+values+` ON CONFLICT(satellite_id, piece_prefix) DO UPDATE SET total_bytes = excluded.total_bytes, total_content_size = excluded.total_content_size, piece_counts = excluded.piece_counts, last_updated = excluded.last_updated`,
		args...)

	return ErrUsedSpacePerPrefix.Wrap(err)
}

// Get returns the used space per prefix for the satellite, for prefixes that were updated after lastUpdated.
func (db *usedSpacePerPrefixDB) Get(ctx context.Context, satelliteID storj.NodeID, lastUpdated *time.Time) (usedSpaces []pieces.PrefixUsedSpace, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT piece_prefix, total_bytes, total_content_size, piece_counts, last_updated
		FROM used_space_per_prefix
		WHERE satellite_id = ?`

	params := []any{satelliteID}
	if lastUpdated != nil {
		query += ` AND last_updated > ?`
		params = append(params, lastUpdated.UTC())
	}

	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, ErrUsedSpacePerPrefix.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		usedSpace := pieces.PrefixUsedSpace{
			SatelliteID: satelliteID,
		}

		err = rows.Scan(&usedSpace.Prefix, &usedSpace.TotalBytes, &usedSpace.TotalContentSize, &usedSpace.PieceCounts, &usedSpace.LastUpdated)
		if err != nil {
			return nil, ErrUsedSpacePerPrefix.Wrap(err)
		}
		usedSpaces = append(usedSpaces, usedSpace)
	}

	return usedSpaces, ErrUsedSpacePerPrefix.Wrap(rows.Err())
}

// GetSatelliteUsedSpace returns the total used space for the satellite.
func (db *usedSpacePerPrefixDB) GetSatelliteUsedSpace(ctx context.Context, satelliteID storj.NodeID) (piecesTotal, piecesContentSize, piecesCount int64, err error) {
	defer mon.Task()(&ctx)(&err)

	err = db.QueryRowContext(ctx, `
		SELECT SUM(total_bytes), SUM(total_content_size), SUM(piece_counts)
		FROM used_space_per_prefix
		WHERE satellite_id = ?
	`, satelliteID).Scan(&piecesTotal, &piecesContentSize, &piecesCount)

	return piecesTotal, piecesContentSize, piecesCount, ErrUsedSpacePerPrefix.Wrap(err)
}

// Delete deletes the used space for the satellite.
func (db *usedSpacePerPrefixDB) Delete(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.ExecContext(ctx, `DELETE FROM used_space_per_prefix WHERE satellite_id = ?`, satelliteID)
	return ErrUsedSpacePerPrefix.Wrap(err)
}
