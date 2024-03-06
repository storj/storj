// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
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

func (db *usedSpacePerPrefixDB) Store(ctx context.Context, usedSpace pieces.PrefixUsedSpace) (err error) {
	defer mon.Task()(&ctx)(&err)

	if usedSpace.LastUpdated.IsZero() {
		usedSpace.LastUpdated = time.Now()
	}

	_, err = db.ExecContext(ctx, `
		INSERT OR REPLACE INTO used_space_per_prefix(satellite_id, piece_prefix, total_bytes, last_updated)
		VALUES($1,$2,$3,$4)
		ON CONFLICT(satellite_id, piece_prefix) DO UPDATE SET total_bytes = $3, last_updated = $4`,
		usedSpace.SatelliteID, usedSpace.Prefix, usedSpace.TotalBytes, usedSpace.LastUpdated.UTC())

	return ErrUsedSpacePerPrefix.Wrap(err)
}

func (db *usedSpacePerPrefixDB) Get(ctx context.Context, satelliteID storj.NodeID) (usedSpaces []pieces.PrefixUsedSpace, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.QueryContext(ctx, `
		SELECT piece_prefix, total_bytes, last_updated
		FROM used_space_per_prefix
		WHERE satellite_id = ?
	`, satelliteID)
	if err != nil {
		return nil, ErrUsedSpacePerPrefix.Wrap(err)
	}

	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		usedSpace := pieces.PrefixUsedSpace{
			SatelliteID: satelliteID,
		}

		err = rows.Scan(&usedSpace.Prefix, &usedSpace.TotalBytes, &usedSpace.LastUpdated)
		if err != nil {
			return nil, ErrUsedSpacePerPrefix.Wrap(err)
		}
		usedSpaces = append(usedSpaces, usedSpace)
	}

	return usedSpaces, ErrUsedSpacePerPrefix.Wrap(rows.Err())
}
