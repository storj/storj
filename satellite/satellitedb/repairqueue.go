// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/pgutil"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// RepairQueueSelectLimit defines how many items can be selected at the same time.
const RepairQueueSelectLimit = 1000

// repairQueue implements storj.io/storj/satellite/repair/queue.RepairQueue.
type repairQueue struct {
	db *satelliteDB
}

func (r *repairQueue) Insert(ctx context.Context, seg *queue.InjuredSegment) (alreadyInserted bool, err error) {
	defer mon.Task()(&ctx)(&err)
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
			SET segment_health=EXCLUDED.segment_health, updated_at=current_timestamp
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

func (r *repairQueue) Select(ctx context.Context) (seg *queue.InjuredSegment, err error) {
	defer mon.Task()(&ctx)(&err)

	segment := queue.InjuredSegment{}
	switch r.db.impl {
	case dbutil.Cockroach:
		err = r.db.QueryRowContext(ctx, `
				UPDATE repair_queue SET attempted_at = now()
				WHERE attempted_at IS NULL OR attempted_at < now() - interval '6 hours'
				ORDER BY segment_health ASC, attempted_at NULLS FIRST
				LIMIT 1
				RETURNING stream_id, position, attempted_at, updated_at, inserted_at, segment_health, placement
		`).Scan(&segment.StreamID, &segment.Position, &segment.AttemptedAt,
			&segment.UpdatedAt, &segment.InsertedAt, &segment.SegmentHealth, &segment.Placement)
	case dbutil.Postgres:
		err = r.db.QueryRowContext(ctx, `
				UPDATE repair_queue SET attempted_at = now() WHERE (stream_id, position) = (
					SELECT stream_id, position FROM repair_queue
					WHERE attempted_at IS NULL OR attempted_at < now() - interval '6 hours'
					ORDER BY segment_health ASC, attempted_at NULLS FIRST FOR UPDATE SKIP LOCKED LIMIT 1
				) RETURNING stream_id, position, attempted_at, updated_at, inserted_at, segment_health, placement
		`).Scan(&segment.StreamID, &segment.Position, &segment.AttemptedAt,
			&segment.UpdatedAt, &segment.InsertedAt, &segment.SegmentHealth, &segment.Placement)
	default:
		return seg, errs.New("unhandled database: %v", r.db.impl)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, queue.ErrEmpty.New("")
		}
		return nil, err
	}
	return &segment, err
}

func (r *repairQueue) Delete(ctx context.Context, seg *queue.InjuredSegment) (err error) {
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
		r.db.Rebind(`SELECT stream_id, position, attempted_at, updated_at, segment_health, placement
					FROM repair_queue LIMIT ?`), limit,
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	for rows.Next() {
		var seg queue.InjuredSegment
		err = rows.Scan(&seg.StreamID, &seg.Position, &seg.AttemptedAt,
			&seg.UpdatedAt, &seg.SegmentHealth, &seg.Placement)
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
func (r *repairQueue) TestingSetAttemptedTime(ctx context.Context, streamID uuid.UUID,
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
