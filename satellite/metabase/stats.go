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
	//TODO:spanner use https://cloud.google.com/spanner/docs/introspection/table-sizes-statistics
	return TableStats{
		SegmentCount: 0,
	}, nil
}
