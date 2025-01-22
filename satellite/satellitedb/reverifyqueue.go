// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
)

// reverifyQueue implements storj.io/storj/satellite/audit.ReverifyQueue.
type reverifyQueue struct {
	db *satelliteDB
}

var _ audit.ReverifyQueue = (*reverifyQueue)(nil)

// Insert adds a reverification job to the queue. If there is already
// a matching job in the queue, nothing happens. (reverify_count is only
// incremented when a job is selected by GetNextJob.)
func (rq *reverifyQueue) Insert(ctx context.Context, piece *audit.PieceLocator) (err error) {
	defer mon.Task()(&ctx)(&err)

	var insertQuery string
	switch rq.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		insertQuery = `
			INSERT INTO reverification_audits ("node_id", "stream_id", "position", "piece_num")
				VALUES ($1, $2, $3, $4)
			ON CONFLICT ("node_id", "stream_id", "position") DO NOTHING
		`
	case dbutil.Spanner:
		insertQuery = `
			INSERT OR IGNORE INTO reverification_audits (node_id, stream_id, position, piece_num)
			VALUES (?, ?, ?, ?)
		`
	default:
		return audit.Error.New("unsupported database dialect: %s", rq.db.impl)
	}

	_, err = rq.db.DB.ExecContext(ctx, insertQuery, piece.NodeID[:], piece.StreamID[:], piece.Position.Encode(), piece.PieceNum)
	if err == nil {
		mon.Counter("audit_reverify_queue_piece_inserted").Inc(1)
	}

	return audit.Error.Wrap(err)
}

// GetNextJob retrieves a job from the queue. The job will be the
// job which has been in the queue the longest, except those which
// have already been claimed by another worker within the last
// retryInterval. If there are no such jobs, an error wrapped by
// audit.ErrEmptyQueue will be returned.
//
// retryInterval is expected to be held to the same value for every
// call to GetNextJob() within a given satellite cluster.
func (rq *reverifyQueue) GetNextJob(ctx context.Context, retryInterval time.Duration) (job *audit.ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)

	scanJobFunc := func(row *sql.Row) (*audit.ReverificationJob, error) {
		scannedJob := &audit.ReverificationJob{}
		err = row.Scan(
			&scannedJob.Locator.NodeID,
			&scannedJob.Locator.StreamID,
			&scannedJob.Locator.Position,
			&scannedJob.Locator.PieceNum,
			&scannedJob.InsertedAt,
			&scannedJob.ReverifyCount,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, audit.ErrEmptyQueue.Wrap(err)
		}
		if err != nil {
			return nil, audit.Error.Wrap(err)
		}

		return scannedJob, nil
	}
	switch rq.db.impl {
	case dbutil.Postgres, dbutil.Cockroach:
		selectQuery := `
			WITH next_entry AS (
				SELECT *
				FROM reverification_audits
				WHERE COALESCE(last_attempt, inserted_at) < (now() - '1 microsecond'::interval * $1::bigint)
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
		`
		row := rq.db.QueryRowContext(ctx, selectQuery, retryInterval.Microseconds())
		job, err = scanJobFunc(row)
		return job, err
	case dbutil.Spanner:
		selectQuery := `
			UPDATE reverification_audits
			SET last_attempt = CURRENT_TIMESTAMP(),
				reverify_count = reverify_count + 1
			WHERE (node_id, stream_id, position) IN (
				SELECT (node_id, stream_id, position)
				FROM reverification_audits
				WHERE COALESCE(last_attempt, inserted_at) < TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL ? nanosecond)
				ORDER BY inserted_at
				LIMIT 1
			)
			THEN RETURN node_id, stream_id, position, piece_num, inserted_at, reverify_count`
		row := rq.db.QueryRowContext(ctx, selectQuery, retryInterval.Nanoseconds())
		job, err = scanJobFunc(row)
		return job, err
	default:
		return nil, audit.Error.New("unsupported database dialect: %s", rq.db.impl)
	}
}

// Remove removes a job from the reverification queue, whether because the job
// was successful or because the job is no longer necessary. The wasDeleted
// return value indicates whether the indicated job was actually deleted (if
// not, there was no such job in the queue).
func (rq *reverifyQueue) Remove(ctx context.Context, piece *audit.PieceLocator) (wasDeleted bool, err error) {
	defer mon.Task()(&ctx)(&err)

	wasDeleted, err = rq.db.Delete_ReverificationAudits_By_NodeId_And_StreamId_And_Position(
		ctx,
		dbx.ReverificationAudits_NodeId(piece.NodeID[:]),
		dbx.ReverificationAudits_StreamId(piece.StreamID[:]),
		dbx.ReverificationAudits_Position(piece.Position.Encode()),
	)

	if wasDeleted {
		mon.Counter("audit_reverify_queue_piece_deleted").Dec(1)
	}

	return wasDeleted, err
}

// TestingFudgeUpdateTime (used only for testing) changes the last_update
// timestamp for an entry in the reverification queue to a specific value.
func (rq *reverifyQueue) TestingFudgeUpdateTime(ctx context.Context, piece *audit.PieceLocator, updateTime time.Time) error {
	query := rq.db.Rebind(`
		UPDATE reverification_audits
		SET last_attempt = ?
		WHERE node_id = ?
			AND stream_id = ?
			AND position = ?`)
	result, err := rq.db.ExecContext(ctx, query, updateTime, piece.NodeID[:], piece.StreamID[:], piece.Position)
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

func (rq *reverifyQueue) GetByNodeID(ctx context.Context, nodeID storj.NodeID) (pendingJob *audit.ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)

	pending, err := rq.db.First_ReverificationAudits_By_NodeId_OrderBy_Asc_StreamId_Asc_Position(ctx, dbx.ReverificationAudits_NodeId(nodeID.Bytes()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// it looks like dbx will not return this, but this is here just
			// in case dbx learns how.
			return nil, audit.ErrContainedNotFound.New("%v", nodeID)
		}
		return nil, audit.ContainError.Wrap(err)
	}
	if pending == nil {
		return nil, audit.ErrContainedNotFound.New("%v", nodeID)
	}

	return convertDBJob(ctx, pending)
}

func (rq *reverifyQueue) GetAllContainedNodes(ctx context.Context) (nodes []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := rq.db.QueryContext(ctx, `SELECT DISTINCT node_id FROM reverification_audits`)
	if err != nil {
		return nil, audit.ContainError.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, audit.ContainError.Wrap(result.Close()))
	}()

	for result.Next() {
		var nodeIDBytes []byte
		if err := result.Scan(&nodeIDBytes); err != nil {
			return nil, audit.ContainError.Wrap(err)
		}
		nodeID, err := storj.NodeIDFromBytes(nodeIDBytes)
		if err != nil {
			return nil, audit.ContainError.Wrap(err)
		}
		nodes = append(nodes, nodeID)
	}

	return nodes, audit.ContainError.Wrap(result.Err())
}

func convertDBJob(ctx context.Context, info *dbx.ReverificationAudits) (pendingJob *audit.ReverificationJob, err error) {
	defer mon.Task()(&ctx)(&err)
	if info == nil {
		return nil, Error.New("missing info")
	}

	pendingJob = &audit.ReverificationJob{
		Locator: audit.PieceLocator{
			Position: metabase.SegmentPositionFromEncoded(info.Position),
			PieceNum: info.PieceNum,
		},
		InsertedAt:    info.InsertedAt,
		ReverifyCount: int(info.ReverifyCount),
	}

	pendingJob.Locator.NodeID, err = storj.NodeIDFromBytes(info.NodeId)
	if err != nil {
		return nil, audit.ContainError.Wrap(err)
	}
	pendingJob.Locator.StreamID, err = uuid.FromBytes(info.StreamId)
	if err != nil {
		return nil, audit.ContainError.Wrap(err)
	}
	if info.LastAttempt != nil {
		pendingJob.LastAttempt = info.LastAttempt
	}

	return pendingJob, nil
}
