// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cloud.google.com/go/spanner"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/zeebo/errs"
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
	var asOf *time.Time
	err = p.db.QueryRowContext(ctx, `
		WITH schema_names AS (
			SELECT btrim(p) AS schema, ord
			FROM UNNEST(string_to_array((
				SELECT setting FROM pg_settings WHERE name='search_path'
			), ',')) WITH ORDINALITY AS x(p, ord)
		)
		SELECT ut.n_live_tup, GREATEST(ut.last_vacuum, ut.last_analyze, ut.last_autovacuum, ut.last_autoanalyze) AS as_of
		FROM pg_stat_user_tables ut, schema_names sn
		WHERE
			(ut.schemaname = sn.schema OR '"' || ut.schemaname  || '"' = sn.schema)
			AND ut.relname = 'segments'
		ORDER BY sn.ord LIMIT 1
	`).Scan(&result.SegmentCount, &asOf)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TableStats{}, err
	}
	if asOf == nil || time.Since(*asOf) > statsUpToDateThreshold {
		// Can't identify table (complicated search_path situation?), or table
		// has not been VACUUMed or ANALYZEd within the threshold
		err = p.db.QueryRowContext(ctx, `SELECT count(1) FROM segments`).Scan(&result.SegmentCount)
		if err != nil {
			return TableStats{}, err
		}
		return result, nil
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

// UpdateTableStats forces an update of table statistics. Probably useful mostly in test scenarios.
func (db *DB) UpdateTableStats(ctx context.Context) (err error) {
	for _, adapter := range db.adapters {
		err := adapter.UpdateTableStats(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateTableStats forces an update of table statistics. Probably useful mostly in test scenarios.
func (p *PostgresAdapter) UpdateTableStats(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, "VACUUM segments")
	return Error.Wrap(err)
}

// UpdateTableStats forces an update of table statistics. Probably useful mostly in test scenarios.
func (c *CockroachAdapter) UpdateTableStats(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, "CREATE STATISTICS test FROM segments")
	return Error.Wrap(err)
}

// UpdateTableStats forces an update of table statistics. Probably useful mostly in test scenarios.
func (s *SpannerAdapter) UpdateTableStats(ctx context.Context) error {
	return nil
}

// SegmentsStats contains information about the segments table.
type SegmentsStats struct {
	SegmentCount           int64
	PerAdapterSegmentCount []int64
}

// CountSegments returns the number of segments in the segments table.
func (db *DB) CountSegments(ctx context.Context, checkTimestamp time.Time) (result SegmentsStats, err error) {
	defer mon.Task()(&ctx)(&err)

	for _, adapter := range db.adapters {
		count, err := adapter.CountSegments(ctx, checkTimestamp)
		if err != nil {
			return SegmentsStats{}, Error.Wrap(err)
		}
		result.SegmentCount += count
		result.PerAdapterSegmentCount = append(result.PerAdapterSegmentCount, count)
	}
	return result, nil
}

// CountSegments returns the number of segments in the segments table.
func (s *SpannerAdapter) CountSegments(ctx context.Context, checkTimestamp time.Time) (result int64, err error) {
	defer mon.Task()(&ctx)(&err)

	stmt := spanner.Statement{
		SQL: `SELECT COUNT(1) FROM segments`,
	}

	iterator := s.client.Single().WithTimestampBound(spanner.ReadTimestamp(checkTimestamp)).QueryWithOptions(ctx, stmt, spanner.QueryOptions{
		Priority: spannerpb.RequestOptions_PRIORITY_LOW,
	})
	defer iterator.Stop()

	row, err := iterator.Next()
	if err != nil {
	}

	if err := row.Columns(&result); err != nil {
		return 0, Error.Wrap(err)
	}
	return result, nil
}

// CountSegments returns the number of segments in the segments table.
func (p *PostgresAdapter) CountSegments(ctx context.Context, checkTimestamp time.Time) (result int64, err error) {
	return 0, errs.New("not implemented")
}
