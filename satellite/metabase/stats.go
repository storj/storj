// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"
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
	for _, adapter := range db.adapters {
		stats, err := adapter.GetTableStats(ctx, opts)
		if err != nil {
			return result, err
		}
		result.SegmentCount += stats.SegmentCount
	}
	return result, nil

}

// GetTableStats implements Adapter.
func (p *PostgresAdapter) GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error) {
	defer mon.Task()(&ctx)(&err)
	err = p.db.QueryRowContext(ctx, `SELECT count(1) FROM segments`).Scan(&result.SegmentCount)
	if err != nil {
		return TableStats{}, err
	}
	return result, nil
}

// GetTableStats implements Adapter.
func (c *CockroachAdapter) GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error) {
	// if it's cockroach and statistics are up to date we will use them to get segments count
	var created time.Time
	err = c.db.QueryRowContext(ctx, `WITH stats AS (SHOW STATISTICS FOR TABLE segments) SELECT row_count, created FROM stats ORDER BY created DESC LIMIT 1`).
		Scan(&result.SegmentCount, &created)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TableStats{}, err
	}

	if !created.IsZero() && statsUpToDateThreshold > time.Since(created) {
		return result, nil
	}
	err = c.db.QueryRowContext(ctx, `SELECT count(1) FROM segments `+c.impl.AsOfSystemInterval(opts.AsOfSystemInterval)).Scan(&result.SegmentCount)
	if err != nil {
		return TableStats{}, err
	}
	return result, nil
}

// GetTableStats (will) implement Adapter.
func (s *SpannerAdapter) GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error) {
	// TODO:spanner gather a total number of bytes stored instead of rows
	//
	// Unfortunately, https://cloud.google.com/spanner/docs/introspection/table-sizes-statistics
	// won't quite be able to get us a number of rows here. It can only tell us how many total
	// bytes are used to store a table, and not the number of rows. We could theoretically use
	// the average number of bytes per row to get a decent estimate of the number of rows, but
	// the sizes in TABLE_SIZES_STATS_1HOUR include all past versions of rows and deleted rows
	// for whatever the version_retention_period is.
	//
	// Some other problems are (1) the Spanner emulator does not support TABLE_SIZES_STATS_1HOUR
	// at all, and (2) there is no way to request or force an update to the statistics other than
	// waiting until the top of the next hour.
	//
	// Instead of trying to force spanner into a cockroach-shaped hole, we should probably just
	// report the table sizes in bytes. This will require storing some different metrics in
	// rangedloop/observerlivecount.go, but that shouldn't be too bad.

	return TableStats{
		SegmentCount: 0,
	}, nil
}
