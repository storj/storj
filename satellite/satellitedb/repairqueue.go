// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/tagsql"
)

// RepairQueueSelectLimit defines how many items can be selected at the same time.
const RepairQueueSelectLimit = 1000

// repairQueue implements storj.io/storj/satellite/repair/queue.RepairQueue.
type repairQueue struct {
	db *satelliteDB
}

// Stat returns stat of the current queue state.
func (r *repairQueue) Stat(ctx context.Context) ([]queue.Stat, error) {
	query := `
        select placement,
            count(1),
            max(inserted_at)  as max_inserted_at,
            min(inserted_at)  as min_inserted_at,
            max(attempted_at) as max_attempted_at,
            min(attempted_at) as min_attempted_at,
            max(segment_health) as max_health,
            min(segment_health) as min_health
        from repair_queue
        group by placement, attempted_at is null`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	var res []queue.Stat
	for rows.Next() {
		var stat queue.Stat
		err = rows.Scan(
			&stat.Placement,
			&stat.Count,
			&stat.MaxInsertedAt,
			&stat.MinInsertedAt,
			&stat.MaxAttemptedAt,
			&stat.MinAttemptedAt,
			&stat.MaxSegmentHealth,
			&stat.MinSegmentHealth,
		)
		if err != nil {
			return res, err
		}
		res = append(res, stat)
	}
	return res, rows.Err()
}

func (r *repairQueue) Insert(ctx context.Context, seg *queue.InjuredSegment) (alreadyInserted bool, err error) {
	defer mon.Task()(&ctx)(&err)
	// insert if not exists, or update healthy count if does exist
	var query string

	mon.Counter("repair_queue_inserted").Inc(1)

	// we want to insert the segment if it is not in the queue, but update the segment health if it already is in the queue
	// we also want to know if the result was an insert or an update - this is the reasoning for the xmax section of the postgres query
	// and the separate cockroach query (which the xmax trick does not work for)
	switch r.db.impl {
	case dbutil.Postgres:
		query = `
			INSERT INTO repair_queue
			(
				stream_id, position, segment_health, placement
			)
			VALUES (
				$1, $2, $3, $4
			)
			ON CONFLICT (stream_id, position)
			DO UPDATE
			SET segment_health=$3, updated_at=current_timestamp, placement=$4
			RETURNING (xmax != 0) AS alreadyInserted
		`
	case dbutil.Cockroach:
		// TODO it's not optimal solution but crdb is not used in prod for repair queue
		query = `
			WITH inserted AS (
				SELECT count(*) as alreadyInserted FROM repair_queue
				WHERE stream_id = $1 AND position = $2
			)
			INSERT INTO repair_queue
			(
				stream_id, position, segment_health, placement
			)
			VALUES (
				$1, $2, $3, $4
			)
			ON CONFLICT (stream_id, position)
			DO UPDATE
			SET segment_health=$3, updated_at=current_timestamp, placement=$4
			RETURNING (SELECT alreadyInserted FROM inserted)
		`
	case dbutil.Spanner:
		query = `UPDATE repair_queue SET segment_health = ?, updated_at = current_timestamp(), placement=? WHERE stream_id=? AND position=?`
		err = r.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			result, txErr := tx.Tx.ExecContext(ctx, query, seg.SegmentHealth, seg.Placement, seg.StreamID, seg.Position)
			if txErr != nil {
				return Error.Wrap(txErr)
			}
			if rows, txErr := result.RowsAffected(); txErr != nil {
				return Error.Wrap(txErr)
			} else {
				alreadyInserted = rows > 0
			}
			if !alreadyInserted {
				query = `INSERT OR IGNORE INTO repair_queue (stream_id, position, segment_health, placement) VALUES (?, ?, ?, ?)`
				if _, txErr = tx.Tx.ExecContext(ctx, query, seg.StreamID, seg.Position.Encode(), seg.SegmentHealth, seg.Placement); txErr != nil {
					return Error.Wrap(err)
				}
			}
			return nil
		})
		return alreadyInserted, err
	default:
		return false, Error.New("unsupported database: %v", r.db.impl)
	}

	rows, err := r.db.QueryContext(ctx, query, seg.StreamID, seg.Position.Encode(), seg.SegmentHealth, seg.Placement)
	if err != nil {
		return false, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	if !rows.Next() {
		// cockroach query does not return anything if the segment is already in the queue
		alreadyInserted = true
	} else {
		err = rows.Scan(&alreadyInserted)
		if err != nil {
			return false, err
		}
	}
	return alreadyInserted, rows.Err()
}

func (r *repairQueue) InsertBatch(
	ctx context.Context,
	segments []*queue.InjuredSegment,
) (newlyInsertedSegments []*queue.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(segments) == 0 {
		return nil, nil
	}

	mon.Counter("repair_queue_inserted").Inc(int64(len(segments)))

	// insert if not exists, or update healthy count if does exist
	var query string

	// we want to insert the segment if it is not in the queue, but update the segment health if it already is in the queue
	// we also want to know if the result was an insert or an update - this is the reasoning for the xmax section of the postgres query
	// and the separate cockroach query (which the xmax trick does not work for)
	switch r.db.impl {
	case dbutil.Postgres:
		query = `
			INSERT INTO repair_queue
			(
				stream_id, position, segment_health, placement
			)
			VALUES (
				UNNEST($1::BYTEA[]),
				UNNEST($2::INT8[]),
				UNNEST($3::double precision[]),
				UNNEST($4::INT2[])
			)
			ON CONFLICT (stream_id, position)
			DO UPDATE
			SET segment_health=EXCLUDED.segment_health, updated_at=current_timestamp, placement=EXCLUDED.placement
			RETURNING NOT(xmax != 0) AS newlyInserted
		`
	case dbutil.Cockroach:
		// TODO it's not optimal solution but crdb is not used in prod for repair queue
		query = `
			WITH to_insert AS (
				SELECT
					UNNEST($1::BYTEA[]) AS stream_id,
					UNNEST($2::INT8[]) AS position,
					UNNEST($3::double precision[]) AS segment_health,
					UNNEST($4::INT2[]) AS placement
			),
			do_insert AS (
				INSERT INTO repair_queue (
					stream_id, position, segment_health, placement
				)
				SELECT stream_id, position, segment_health, placement
				FROM to_insert
				ON CONFLICT (stream_id, position)
				DO UPDATE
				SET
					segment_health=EXCLUDED.segment_health,
					updated_at=current_timestamp,
					placement=EXCLUDED.placement
				RETURNING false
			)
			SELECT
				(repair_queue.stream_id IS NULL) AS newlyInserted
			FROM to_insert
			LEFT JOIN repair_queue
				ON to_insert.stream_id = repair_queue.stream_id
				AND to_insert.position = repair_queue.position
		`

	case dbutil.Spanner:
		err := r.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
			for _, s := range segments {
				res, err := tx.ExecContext(ctx, "UPDATE repair_queue SET segment_health = ?, placement = ?, updated_at = current_timestamp() where stream_id = ? AND position = ?", s.SegmentHealth, s.Placement, s.StreamID, s.Position.Encode())
				if err != nil {
					return errs.Wrap(err)
				}

				affected, err := res.RowsAffected()
				if err != nil {
					return errs.Wrap(err)
				}

				if affected == 0 {
					_, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO repair_queue (stream_id, position, segment_health, placement, updated_at) VALUES (?, ?, ?, ?, current_timestamp())", s.StreamID, s.Position.Encode(), s.SegmentHealth, s.Placement)
					if err != nil {
						return errs.Wrap(err)
					}

					newlyInsertedSegments = append(newlyInsertedSegments, s)
				}
			}
			return nil
		})
		return newlyInsertedSegments, err

	default:
		return nil, Error.New("unsupported database: %v", r.db.impl)
	}

	var insertData struct {
		StreamIDs      []uuid.UUID
		Positions      []int64
		SegmentHealths []float64
		placements     []int16
	}

	for _, segment := range segments {
		insertData.StreamIDs = append(insertData.StreamIDs, segment.StreamID)
		insertData.Positions = append(insertData.Positions, int64(segment.Position.Encode()))
		insertData.SegmentHealths = append(insertData.SegmentHealths, segment.SegmentHealth)
		insertData.placements = append(insertData.placements, int16(segment.Placement))
	}

	rows, err := r.db.QueryContext(
		ctx, query,
		pgutil.UUIDArray(insertData.StreamIDs),
		pgutil.Int8Array(insertData.Positions),
		pgutil.Float8Array(insertData.SegmentHealths),
		pgutil.Int2Array(insertData.placements),
	)

	if err != nil {
		return newlyInsertedSegments, err
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	i := 0
	for rows.Next() {
		var isNewlyInserted bool
		err = rows.Scan(&isNewlyInserted)
		if err != nil {
			return newlyInsertedSegments, err
		}

		if isNewlyInserted {
			newlyInsertedSegments = append(newlyInsertedSegments, segments[i])
		}

		i++
	}

	return newlyInsertedSegments, rows.Err()

}

func (r *repairQueue) Select(ctx context.Context, n int, includedPlacements []storj.PlacementConstraint, excludedPlacements []storj.PlacementConstraint) (segments []queue.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	restriction := ""

	placementsToString := func(placements []storj.PlacementConstraint) string {
		var ps []string
		for _, p := range placements {
			ps = append(ps, fmt.Sprintf("%d", p))
		}
		return strings.Join(ps, ",")
	}
	if len(includedPlacements) > 0 {
		restriction += fmt.Sprintf(" AND placement IN (%s)", placementsToString(includedPlacements))
	}

	if len(excludedPlacements) > 0 {
		restriction += fmt.Sprintf(" AND placement NOT IN (%s)", placementsToString(excludedPlacements))
	}

	scanSegments := func(rows tagsql.Rows) error {
		return withRows(rows, err)(func(rows tagsql.Rows) error {
			for rows.Next() {
				var segment queue.InjuredSegment
				err := rows.Scan(&segment.StreamID, &segment.Position, &segment.AttemptedAt,
					&segment.UpdatedAt, &segment.InsertedAt, &segment.SegmentHealth, &segment.Placement)
				if err != nil {
					return errs.Wrap(err)
				}
				segments = append(segments, segment)
			}
			return nil
		})
	}

	var rows tagsql.Rows
	switch r.db.impl {
	case dbutil.Postgres:
		rows, err = r.db.QueryContext(ctx, r.db.Rebind(`
			UPDATE repair_queue SET attempted_at = now() WHERE (stream_id, position) IN (
				SELECT stream_id, position FROM repair_queue
				WHERE (attempted_at IS NULL OR attempted_at < now() - interval '6 hours') `+restriction+`
				ORDER BY segment_health ASC, attempted_at NULLS FIRST FOR UPDATE SKIP LOCKED
				LIMIT ?
			) RETURNING stream_id, position, attempted_at, updated_at, inserted_at, segment_health, placement

		`), n)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		err = scanSegments(rows)
	case dbutil.Cockroach:
		rows, err = r.db.QueryContext(ctx, r.db.Rebind(`
			UPDATE repair_queue SET attempted_at = now()
			WHERE (attempted_at IS NULL OR attempted_at < now() - interval '6 hours') `+restriction+`
			ORDER BY segment_health ASC, attempted_at NULLS FIRST
			LIMIT ?
			RETURNING stream_id, position, attempted_at, updated_at, inserted_at, segment_health, placement

		`), n)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		err = scanSegments(rows)
	case dbutil.Spanner:
		// Note: Spanner sorts nulls first by default.
		rows, err = r.db.QueryContext(ctx, `UPDATE repair_queue
			SET attempted_at = CURRENT_TIMESTAMP()
			WHERE (stream_id, position) IN (
				SELECT (stream_id, position)
				FROM (
					SELECT stream_id, position
					FROM repair_queue
					WHERE (attempted_at IS NULL OR attempted_at < TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 6 HOUR)) `+restriction+`
					ORDER BY segment_health ASC, attempted_at ASC
					LIMIT ?
				) AS target_row
			)
			THEN RETURN stream_id, position, attempted_at, updated_at, inserted_at, segment_health, placement`, n)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		err = scanSegments(rows)
	default:
		return segments, errs.New("unhandled database: %v", r.db.impl)
	}
	if err != nil {
		return nil, err
	}

	if len(segments) == 0 {
		return nil, queue.ErrEmpty.New("")
	}

	return segments, nil
}

func (r *repairQueue) Release(ctx context.Context, seg queue.InjuredSegment, repaired bool) (err error) {
	defer mon.Task()(&ctx)(&err)
	if repaired {
		return Error.Wrap(r.Delete(ctx, seg))
	}
	return nil
}

func (r *repairQueue) Delete(ctx context.Context, seg queue.InjuredSegment) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM repair_queue WHERE stream_id = ? AND position = ?`), seg.StreamID, seg.Position.Encode())
	return Error.Wrap(err)
}

func (r *repairQueue) Clean(ctx context.Context, before time.Time) (deleted int64, err error) {
	defer mon.Task()(&ctx)(&err)
	n, err := r.db.Delete_RepairQueue_By_UpdatedAt_Less(ctx, dbx.RepairQueue_UpdatedAt(before))
	return n, Error.Wrap(err)
}

func (r *repairQueue) SelectN(ctx context.Context, limit int) (segs []queue.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)
	if limit <= 0 || limit > RepairQueueSelectLimit {
		limit = RepairQueueSelectLimit
	}
	// TODO: strictly enforce order-by or change tests
	rows, err := r.db.QueryContext(ctx,
		r.db.Rebind(`SELECT stream_id, position, attempted_at, updated_at, inserted_at, segment_health, placement
					FROM repair_queue LIMIT ?`), limit,
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var seg queue.InjuredSegment
		err = rows.Scan(&seg.StreamID, &seg.Position, &seg.AttemptedAt,
			&seg.UpdatedAt, &seg.InsertedAt, &seg.SegmentHealth, &seg.Placement)
		if err != nil {
			return segs, Error.Wrap(err)
		}
		segs = append(segs, seg)
	}

	return segs, Error.Wrap(rows.Err())
}

func (r *repairQueue) Count(ctx context.Context) (count int, err error) {
	defer mon.Task()(&ctx)(&err)

	// Count every segment regardless of how recently repair was last attempted
	err = r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) as count FROM repair_queue`)).Scan(&count)

	return count, Error.Wrap(err)
}

// TestingSetAttemptedTime sets attempted time for a segment.
func (r *repairQueue) TestingSetAttemptedTime(ctx context.Context, _ storj.PlacementConstraint, streamID uuid.UUID,
	position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error) {

	defer mon.Task()(&ctx)(&err)
	res, err := r.db.ExecContext(ctx,
		r.db.Rebind(`UPDATE repair_queue SET attempted_at = ? WHERE stream_id = ? AND position = ?`),
		t, streamID, position.Encode(),
	)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	count, err := res.RowsAffected()
	return count, Error.Wrap(err)
}

// TestingSetUpdatedTime sets updated time for a segment.
func (r *repairQueue) TestingSetUpdatedTime(ctx context.Context, _ storj.PlacementConstraint, streamID uuid.UUID,
	position metabase.SegmentPosition, t time.Time) (rowsAffected int64, err error) {

	defer mon.Task()(&ctx)(&err)
	res, err := r.db.ExecContext(ctx,
		r.db.Rebind(`UPDATE repair_queue SET updated_at = ? WHERE stream_id = ? AND position = ?`),
		t, streamID, position.Encode(),
	)
	if err != nil {
		return 0, Error.Wrap(err)
	}
	count, err := res.RowsAffected()
	return count, Error.Wrap(err)
}

// Implementation returns the database implementation.
func (r *repairQueue) Implementation() dbutil.Implementation {
	return r.db.impl
}
