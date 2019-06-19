// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"database/sql"
	"storj.io/storj/storagenode/console"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

type consoledb struct { *InfoDB }

// Console returns console.DB
func (db *InfoDB) Console() console.DB { return &consoledb{db}}

// Console returns console.DB
func (db *DB) Console() console.DB { return db.info.Console()}

func (db *consoledb) GetSatelliteIDs(ctx context.Context, from, to time.Time) (_ storj.NodeIDList, err error) {
	defer mon.Task()(&ctx)(&err)
	defer db.locked()()

	var satellites storj.NodeIDList

	rows, err := db.db.QueryContext(ctx, `
		SELECT DISTINCT satellite_id
		FROM bandwidth_usage
		WHERE ? <= created_at AND created_at <= ?`, from, to)

	if err != nil {
		if err == sql.ErrNoRows {
			return satellites, nil
		}
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	for rows.Next() {
		var satelliteID storj.NodeID
		if err = rows.Scan(&satelliteID); err != nil {
			return nil, err
		}

		satellites = append(satellites, satelliteID)
	}

	return satellites, nil
}
