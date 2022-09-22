// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"time"
)

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

	err = db.db.QueryRowContext(ctx, `SELECT count(*) FROM segments `+db.impl.AsOfSystemInterval(opts.AsOfSystemInterval)).Scan(&result.SegmentCount)
	if err != nil {
		return TableStats{}, err
	}
	return result, nil
}
