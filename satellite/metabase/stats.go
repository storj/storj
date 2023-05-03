// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/private/dbutil"
)

const statsUpToDateThreshold = 8 * time.Hour

// GetTableStats contains arguments necessary for getting table statistics.
type GetTableStats struct {
	AsOfSystemInterval time.Duration
}

// TableStats contains information about the metabase status.
type TableStats struct {
	SegmentCount int64
}

// GetTableStats gathers information about the metabase tables, currently only "segments" table.
func (db *DB) GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error) {
	defer mon.Task()(&ctx)(&err)

	// if it's cockroach and statistics are up to date we will use them to get segments count
	if db.impl == dbutil.Cockroach {
		var created time.Time
		err := db.db.QueryRowContext(ctx, `WITH stats AS (SHOW STATISTICS FOR TABLE segments) SELECT row_count, created FROM stats ORDER BY created DESC LIMIT 1`).
			Scan(&result.SegmentCount, &created)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return TableStats{}, err
		}

		if !created.IsZero() && statsUpToDateThreshold > time.Since(created) {
			return result, nil
		}
	}
	err = db.db.QueryRowContext(ctx, `SELECT count(*) FROM segments `+db.impl.AsOfSystemInterval(opts.AsOfSystemInterval)).Scan(&result.SegmentCount)
	if err != nil {
		return TableStats{}, err
	}
	return result, nil
}
