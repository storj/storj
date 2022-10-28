// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"time"

	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/satellitedb/dbx"
)

const (
	// ReverifyRetryInterval defines a limit on how frequently we retry
	// reverification audits. At least this long should elapse between
	// attempts.
	ReverifyRetryInterval = 4 * time.Hour
)

// reverifyQueue implements storj.io/storj/satellite/audit.ReverifyQueue.
type reverifyQueue struct {
	db *satelliteDB
}

var _ audit.ReverifyQueue = (*reverifyQueue)(nil)

// Insert adds a reverification job to the queue.
func (rq *reverifyQueue) Insert(ctx context.Context, piece audit.PieceLocator) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = rq.db.Create_ReverificationAudits(
		ctx,
		dbx.ReverificationAudits_NodeId(piece.NodeID[:]),
		dbx.ReverificationAudits_StreamId(piece.StreamID[:]),
		dbx.ReverificationAudits_Position(piece.Position.Encode()),
		dbx.ReverificationAudits_PieceNum(piece.PieceNum),
		dbx.ReverificationAudits_Create_Fields{},
	)
	return err
}

// GetNextJob retrieves a job from the queue. The job will be the
// job which has been in the queue the longest, except those which
// have already been claimed by another worker within the last
// ReverifyRetryInterval. If there are no such jobs, sql.ErrNoRows
// will be returned.
func (rq *reverifyQueue) GetNextJob(ctx context.Context) (job audit.ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)

	err = rq.db.QueryRowContext(ctx, `
		WITH next_entry AS (
			SELECT *
			FROM reverification_audits
			WHERE last_attempt IS NULL OR last_attempt < (now() - '1 microsecond'::interval * $1::bigint)
			ORDER BY inserted_at
			LIMIT 1
		)
		UPDATE reverification_audits ra
		SET last_attempt = now(),
			reverify_count = ra.reverify_count + 1
		FROM next_entry
		WHERE ra.node_id = next_entry.node_id
			AND ra.stream_id = next_entry.stream_id
			AND ra.position = next_entry.position
		RETURNING ra.node_id, ra.stream_id, ra.position, ra.piece_num, ra.inserted_at, ra.reverify_count
	`, ReverifyRetryInterval.Microseconds()).Scan(
		&job.Locator.NodeID,
		&job.Locator.StreamID,
		&job.Locator.Position,
		&job.Locator.PieceNum,
		&job.InsertedAt,
		&job.ReverifyCount,
	)
	return job, err
}

// Remove removes a job from the reverification queue, whether because the job
// was successful or because the job is no longer necessary. The wasDeleted
// return value indicates whether the indicated job was actually deleted (if
// not, there was no such job in the queue).
func (rq *reverifyQueue) Remove(ctx context.Context, piece audit.PieceLocator) (wasDeleted bool, err error) {
	defer mon.Task()(&ctx)(&err)

	return rq.db.Delete_ReverificationAudits_By_NodeId_And_StreamId_And_Position(
		ctx,
		dbx.ReverificationAudits_NodeId(piece.NodeID[:]),
		dbx.ReverificationAudits_StreamId(piece.StreamID[:]),
		dbx.ReverificationAudits_Position(piece.Position.Encode()),
	)
}

// TestingFudgeUpdateTime (used only for testing) changes the last_update
// timestamp for an entry in the reverification queue to a specific value.
func (rq *reverifyQueue) TestingFudgeUpdateTime(ctx context.Context, piece audit.PieceLocator, updateTime time.Time) error {
	result, err := rq.db.ExecContext(ctx, `
		UPDATE reverification_audits
		SET last_attempt = $4
		WHERE node_id = $1
			AND stream_id = $2
			AND position = $3
	`, piece.NodeID[:], piece.StreamID[:], piece.Position, updateTime)
	if err != nil {
		return err
	}
	numRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if numRows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
