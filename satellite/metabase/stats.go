// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"storj.io/storj/shared/dbutil"
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

// GetTableStats implements Adapter.
func (t *TiDBAdapter) GetTableStats(ctx context.Context, opts GetTableStats) (result TableStats, err error) {
	defer mon.Task()(&ctx)(&err)
	// Read TiDB's persisted statistics from mysql.stats_meta directly, rather
	// than INFORMATION_SCHEMA.TABLES.TABLE_ROWS/UPDATE_TIME. The latter is
	// derived from the in-memory stats handle and reports UPDATE_TIME as NULL
	// for a large, never-fully-analyzed table (pseudo stats), which would make
	// the freshness check below never pass and force an exact COUNT(1) on
	// billions of rows every time. mysql.stats_meta.count is the row-count
	// estimate TiDB keeps continuously updated via background delta-dumping,
	// and version is a TSO whose physical time tells us how fresh it is. Trust
	// the estimate only if it is within statsUpToDateThreshold, otherwise fall
	// back to an exact COUNT(1) to match the Postgres/CockroachDB contract.
	//
	// Note: reading mysql.stats_meta requires SELECT on the mysql schema for
	// the connecting user.
	var (
		count      sql.NullInt64
		ageSeconds sql.NullInt64
	)
	err = t.db.QueryRowContext(ctx, `
		SELECT sm.count,
		       TIMESTAMPDIFF(SECOND, TIDB_PARSE_TSO(sm.version), NOW())
		FROM mysql.stats_meta sm
		JOIN INFORMATION_SCHEMA.TABLES it ON it.tidb_table_id = sm.table_id
		WHERE it.table_schema = DATABASE() AND it.table_name = 'segments'
	`).Scan(&count, &ageSeconds)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return TableStats{}, err
	}
	if count.Valid && count.Int64 > 0 &&
		ageSeconds.Valid && time.Duration(ageSeconds.Int64)*time.Second <= statsUpToDateThreshold {
		result.SegmentCount = count.Int64
		return result, nil
	}
	err = t.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM segments`).Scan(&result.SegmentCount)
	if err != nil {
		return TableStats{}, err
	}
	return result, nil
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
func (t *TiDBAdapter) UpdateTableStats(ctx context.Context) error {
	_, err := t.db.ExecContext(ctx, "ANALYZE TABLE segments")
	return Error.Wrap(err)
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
// Postgres has no AS OF SYSTEM TIME, so it can only count live and refuses a
// checkTimestamp rather than silently describing a different snapshot than
// the caller asked for. CockroachDB inherits this and counts AS OF SYSTEM
// TIME, which is a full scan of the segments table at that timestamp;
// CockroachDB is a legacy metabase backend with limited support, and that
// cost is accepted there.
func (p *PostgresAdapter) CountSegments(ctx context.Context, checkTimestamp time.Time) (result int64, err error) {
	defer mon.Task()(&ctx)(&err)

	asOf := p.Implementation().AsOfSystemTime(checkTimestamp)
	if !checkTimestamp.IsZero() && asOf == "" {
		return 0, ErrInvalidRequest.New("checkTimestamp is not supported on %v", p.Implementation())
	}

	err = p.db.QueryRowContext(ctx, `SELECT count(1) FROM segments`+asOf).Scan(&result)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	return result, nil
}

// CountSegments returns the number of segments in the segments table.
func (t *TiDBAdapter) CountSegments(ctx context.Context, checkTimestamp time.Time) (result int64, err error) {
	defer mon.Task()(&ctx)(&err)

	asOf := ""
	if !checkTimestamp.IsZero() {
		asOf = dbutil.TiDB.AsOfSystemTime(checkTimestamp)
	}

	err = t.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM segments`+asOf).Scan(&result)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	return result, nil
}
