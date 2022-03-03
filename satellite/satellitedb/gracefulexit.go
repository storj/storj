// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/pgutil"
	"storj.io/private/tagsql"
	"storj.io/storj/satellite/gracefulexit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type gracefulexitDB struct {
	db *satelliteDB
}

const (
	deleteExitProgressBatchSize = 1000
)

// IncrementProgress increments transfer stats for a node.
func (db *gracefulexitDB) IncrementProgress(ctx context.Context, nodeID storj.NodeID, bytes int64, successfulTransfers int64, failedTransfers int64) (err error) {
	defer mon.Task()(&ctx)(&err)

	statement := db.db.Rebind(
		`INSERT INTO graceful_exit_progress (node_id, bytes_transferred, pieces_transferred, pieces_failed, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(node_id)
		 DO UPDATE SET bytes_transferred = graceful_exit_progress.bytes_transferred + excluded.bytes_transferred,
		 	pieces_transferred = graceful_exit_progress.pieces_transferred + excluded.pieces_transferred,
		 	pieces_failed = graceful_exit_progress.pieces_failed + excluded.pieces_failed,
		 	updated_at = excluded.updated_at;`,
	)
	now := time.Now().UTC()
	_, err = db.db.ExecContext(ctx, statement, nodeID, bytes, successfulTransfers, failedTransfers, now)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetProgress gets a graceful exit progress entry.
func (db *gracefulexitDB) GetProgress(ctx context.Context, nodeID storj.NodeID) (_ *gracefulexit.Progress, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxProgress, err := db.db.Get_GracefulExitProgress_By_NodeId(ctx, dbx.GracefulExitProgress_NodeId(nodeID.Bytes()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, gracefulexit.ErrNodeNotFound.Wrap(err)
	} else if err != nil {
		return nil, Error.Wrap(err)
	}
	nID, err := storj.NodeIDFromBytes(dbxProgress.NodeId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	progress := &gracefulexit.Progress{
		NodeID:            nID,
		BytesTransferred:  dbxProgress.BytesTransferred,
		PiecesTransferred: dbxProgress.PiecesTransferred,
		PiecesFailed:      dbxProgress.PiecesFailed,
		UpdatedAt:         dbxProgress.UpdatedAt,
	}

	return progress, Error.Wrap(err)
}

// Enqueue batch inserts graceful exit transfer queue entries if it does not exist.
func (db *gracefulexitDB) Enqueue(ctx context.Context, items []gracefulexit.TransferQueueItem, batchSize int) (err error) {
	defer mon.Task()(&ctx)(&err)

	sort.Slice(items, func(i, k int) bool {
		compare := bytes.Compare(items[i].NodeID.Bytes(), items[k].NodeID.Bytes())
		if compare == 0 {
			compare = bytes.Compare(items[i].StreamID[:], items[k].StreamID[:])
			if compare == 0 {
				return items[i].Position.Encode() < items[k].Position.Encode()
			}
			return compare < 0
		}
		return compare < 0
	})

	for i := 0; i < len(items); i += batchSize {
		lowerBound := i
		upperBound := lowerBound + batchSize

		if upperBound > len(items) {
			upperBound = len(items)
		}

		var nodeIDs []storj.NodeID
		var streamIds [][]byte
		var positions []int64
		var pieceNums []int32
		var rootPieceIDs [][]byte
		var durabilities []float64

		for _, item := range items[lowerBound:upperBound] {
			item := item
			nodeIDs = append(nodeIDs, item.NodeID)
			streamIds = append(streamIds, item.StreamID[:])
			positions = append(positions, int64(item.Position.Encode()))
			pieceNums = append(pieceNums, item.PieceNum)
			rootPieceIDs = append(rootPieceIDs, item.RootPieceID.Bytes())
			durabilities = append(durabilities, item.DurabilityRatio)
		}

		_, err = db.db.ExecContext(ctx, db.db.Rebind(`
			INSERT INTO graceful_exit_segment_transfer_queue (
				node_id, stream_id, position, piece_num,
				root_piece_id, durability_ratio, queued_at
			) SELECT
				unnest($1::bytea[]), unnest($2::bytea[]), unnest($3::int8[]),
				unnest($4::int4[]), unnest($5::bytea[]), unnest($6::float8[]),
				$7
			ON CONFLICT DO NOTHING;`), pgutil.NodeIDArray(nodeIDs), pgutil.ByteaArray(streamIds), pgutil.Int8Array(positions),
			pgutil.Int4Array(pieceNums), pgutil.ByteaArray(rootPieceIDs), pgutil.Float8Array(durabilities),
			time.Now().UTC())

		if err != nil {
			return Error.Wrap(err)
		}

	}
	return nil
}

// UpdateTransferQueueItem creates a graceful exit transfer queue entry.
func (db *gracefulexitDB) UpdateTransferQueueItem(ctx context.Context, item gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)

	update := dbx.GracefulExitSegmentTransfer_Update_Fields{
		DurabilityRatio: dbx.GracefulExitSegmentTransfer_DurabilityRatio(item.DurabilityRatio),
		LastFailedCode:  dbx.GracefulExitSegmentTransfer_LastFailedCode_Raw(item.LastFailedCode),
		FailedCount:     dbx.GracefulExitSegmentTransfer_FailedCount_Raw(item.FailedCount),
	}

	if item.RequestedAt != nil {
		update.RequestedAt = dbx.GracefulExitSegmentTransfer_RequestedAt_Raw(item.RequestedAt)
	}
	if item.LastFailedAt != nil {
		update.LastFailedAt = dbx.GracefulExitSegmentTransfer_LastFailedAt_Raw(item.LastFailedAt)
	}
	if item.FinishedAt != nil {
		update.FinishedAt = dbx.GracefulExitSegmentTransfer_FinishedAt_Raw(item.FinishedAt)
	}

	return db.db.UpdateNoReturn_GracefulExitSegmentTransfer_By_NodeId_And_StreamId_And_Position_And_PieceNum(ctx,
		dbx.GracefulExitSegmentTransfer_NodeId(item.NodeID.Bytes()),
		dbx.GracefulExitSegmentTransfer_StreamId(item.StreamID[:]),
		dbx.GracefulExitSegmentTransfer_Position(item.Position.Encode()),
		dbx.GracefulExitSegmentTransfer_PieceNum(int(item.PieceNum)),
		update,
	)
}

// DeleteTransferQueueItem deletes a graceful exit transfer queue entry.
func (db *gracefulexitDB) DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, streamID uuid.UUID, position metabase.SegmentPosition, pieceNum int32) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Delete_GracefulExitSegmentTransfer_By_NodeId_And_StreamId_And_Position_And_PieceNum(ctx,
		dbx.GracefulExitSegmentTransfer_NodeId(nodeID.Bytes()),
		dbx.GracefulExitSegmentTransfer_StreamId(streamID[:]),
		dbx.GracefulExitSegmentTransfer_Position(position.Encode()), dbx.GracefulExitSegmentTransfer_PieceNum(int(pieceNum)))

	return Error.Wrap(err)
}

// DeleteTransferQueueItem deletes a graceful exit transfer queue entries by nodeID.
func (db *gracefulexitDB) DeleteTransferQueueItems(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Delete_GracefulExitSegmentTransfer_By_NodeId(ctx, dbx.GracefulExitSegmentTransfer_NodeId(nodeID.Bytes()))
	return Error.Wrap(err)

}

// DeleteFinishedTransferQueueItem deletes finished graceful exit transfer queue entries by nodeID.
func (db *gracefulexitDB) DeleteFinishedTransferQueueItems(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = db.db.Delete_GracefulExitSegmentTransfer_By_NodeId_And_FinishedAt_IsNot_Null(ctx, dbx.GracefulExitSegmentTransfer_NodeId(nodeID.Bytes()))
	return Error.Wrap(err)
}

// DeleteAllFinishedTransferQueueItems deletes all graceful exit transfer
// queue items whose nodes have finished the exit before the indicated time
// returning the total number of deleted items.
func (db *gracefulexitDB) DeleteAllFinishedTransferQueueItems(
	ctx context.Context, before time.Time, asOfSystemInterval time.Duration, batchSize int) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	switch db.db.impl {
	case dbutil.Postgres:
		statement := `
			DELETE FROM graceful_exit_segment_transfer_queue
			WHERE node_id IN (
				SELECT node_id FROM graceful_exit_segment_transfer_queue INNER JOIN nodes
					ON graceful_exit_segment_transfer_queue.node_id = nodes.id
				WHERE nodes.exit_finished_at IS NOT NULL
				AND nodes.exit_finished_at < $1
			)`
		res, err := db.db.ExecContext(ctx, statement, before)
		if err != nil {
			return 0, Error.Wrap(err)
		}

		count, err := res.RowsAffected()
		if err != nil {
			return 0, Error.Wrap(err)
		}

		return count, nil

	case dbutil.Cockroach:
		nodesQuery := `
			SELECT id
			FROM nodes
		` + db.db.impl.AsOfSystemInterval(asOfSystemInterval) + `
			WHERE exit_finished_at IS NOT NULL
				AND exit_finished_at < $1
			LIMIT $2 OFFSET $3
		`
		deleteStmt := `
			DELETE FROM graceful_exit_segment_transfer_queue
			WHERE node_id = $1
			LIMIT $2
		`

		var (
			deleteCount int64
			offset      int
		)
		for {
			var nodeIDs storj.NodeIDList
			deleteItems := func() (int64, error) {
				// Select exited nodes
				rows, err := db.db.QueryContext(ctx, nodesQuery, before, batchSize, offset)
				if err != nil {
					return deleteCount, Error.Wrap(err)
				}
				defer func() { err = errs.Combine(err, rows.Close()) }()

				count := 0
				for rows.Next() {
					var id storj.NodeID
					if err = rows.Scan(&id); err != nil {
						return deleteCount, Error.Wrap(err)
					}
					nodeIDs = append(nodeIDs, id)
					count++
				}

				if count == batchSize {
					offset += count
				} else {
					offset = -1 // indicates that there aren't more nodes to query
				}

				for _, id := range nodeIDs {
					for {
						res, err := db.db.ExecContext(ctx, deleteStmt, id.Bytes(), batchSize)
						if err != nil {
							return deleteCount, Error.Wrap(err)
						}
						count, err := res.RowsAffected()
						if err != nil {
							return deleteCount, Error.Wrap(err)
						}
						deleteCount += count
						if count < int64(batchSize) {
							break
						}
					}
				}
				return deleteCount, nil
			}
			deleteCount, err = deleteItems()
			if err != nil {
				return deleteCount, err
			}
			// when offset is negative means that we have get already all the nodes
			// which have exited
			if offset < 0 {
				break
			}
		}
		return deleteCount, nil
	}

	return 0, Error.New("unsupported implementation: %s", db.db.impl)
}

// DeleteFinishedExitProgress deletes exit progress entries for nodes that
// finished exiting before the indicated time, returns number of deleted entries.
func (db *gracefulexitDB) DeleteFinishedExitProgress(
	ctx context.Context, before time.Time, asOfSystemInterval time.Duration) (_ int64, err error) {
	defer mon.Task()(&ctx)(&err)

	finishedNodes, err := db.GetFinishedExitNodes(ctx, before, asOfSystemInterval)
	if err != nil {
		return 0, err
	}
	return db.DeleteBatchExitProgress(ctx, finishedNodes)
}

// GetFinishedExitNodes gets nodes that are marked having finished graceful exit before a given time.
func (db *gracefulexitDB) GetFinishedExitNodes(ctx context.Context, before time.Time, asOfSystemInterval time.Duration) (finishedNodes []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)
	stmt := `
		SELECT id
		FROM nodes
		` + db.db.impl.AsOfSystemInterval(asOfSystemInterval) + `
        WHERE exit_finished_at IS NOT NULL
	    AND exit_finished_at < ?
		`
	rows, err := db.db.Query(ctx, db.db.Rebind(stmt), before.UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = Error.Wrap(errs.Combine(err, rows.Close()))
	}()

	for rows.Next() {
		var id storj.NodeID
		err = rows.Scan(&id)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		finishedNodes = append(finishedNodes, id)
	}
	return finishedNodes, Error.Wrap(rows.Err())
}

// DeleteBatchExitProgress batch deletes from exit progress. This is separate from
// getting the node IDs because the combined query is slow in CRDB. It's safe to do
// separately because if nodes are deleted between the get and delete, it doesn't
// affect correctness.
func (db *gracefulexitDB) DeleteBatchExitProgress(ctx context.Context, nodeIDs []storj.NodeID) (deleted int64, err error) {
	defer mon.Task()(&ctx)(&err)
	stmt := `DELETE FROM graceful_exit_progress
			WHERE node_id = ANY($1)`
	for len(nodeIDs) > 0 {
		numToSubmit := len(nodeIDs)
		if numToSubmit > deleteExitProgressBatchSize {
			numToSubmit = deleteExitProgressBatchSize
		}
		nodesToSubmit := nodeIDs[:numToSubmit]
		res, err := db.db.ExecContext(ctx, stmt, pgutil.NodeIDArray(nodesToSubmit))
		if err != nil {
			return deleted, Error.Wrap(err)
		}
		count, err := res.RowsAffected()
		if err != nil {
			return deleted, Error.Wrap(err)
		}
		deleted += count
		nodeIDs = nodeIDs[numToSubmit:]
	}
	return deleted, Error.Wrap(err)
}

// GetTransferQueueItem gets a graceful exit transfer queue entry.
func (db *gracefulexitDB) GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, streamID uuid.UUID, position metabase.SegmentPosition, pieceNum int32) (_ *gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxTransferQueue, err := db.db.Get_GracefulExitSegmentTransfer_By_NodeId_And_StreamId_And_Position_And_PieceNum(ctx,
		dbx.GracefulExitSegmentTransfer_NodeId(nodeID.Bytes()),
		dbx.GracefulExitSegmentTransfer_StreamId(streamID[:]),
		dbx.GracefulExitSegmentTransfer_Position(position.Encode()),
		dbx.GracefulExitSegmentTransfer_PieceNum(int(pieceNum)))

	if err != nil {
		return nil, Error.Wrap(err)
	}
	transferQueueItem, err := dbxSegmentTransferToTransferQueueItem(dbxTransferQueue)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItem, Error.Wrap(err)

}

// GetIncomplete gets incomplete graceful exit transfer queue entries ordered by durability ratio and queued date ascending.
func (db *gracefulexitDB) GetIncomplete(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)

	sql := `
			SELECT
				node_id, stream_id, position,
				piece_num, root_piece_id, durability_ratio,
				queued_at, requested_at, last_failed_at,
				last_failed_code, failed_count, finished_at,
				order_limit_send_count
			FROM graceful_exit_segment_transfer_queue
			WHERE node_id = ?
			AND finished_at is NULL
			ORDER BY durability_ratio asc, queued_at asc LIMIT ? OFFSET ?`
	rows, err := db.db.Query(ctx, db.db.Rebind(sql), nodeID.Bytes(), limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	transferQueueItemRows, err := scanRows(rows)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItemRows, nil
}

// GetIncompleteNotFailed gets incomplete graceful exit transfer queue entries that haven't failed, ordered by durability ratio and queued date ascending.
func (db *gracefulexitDB) GetIncompleteNotFailed(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)

	sql := `
			SELECT
				node_id, stream_id, position,
				piece_num, root_piece_id, durability_ratio,
				queued_at, requested_at, last_failed_at,
				last_failed_code, failed_count, finished_at,
				order_limit_send_count
			FROM graceful_exit_segment_transfer_queue
			WHERE node_id = ?
			AND finished_at is NULL
			AND last_failed_at is NULL
			ORDER BY durability_ratio asc, queued_at asc LIMIT ? OFFSET ?`
	rows, err := db.db.Query(ctx, db.db.Rebind(sql), nodeID.Bytes(), limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	transferQueueItemRows, err := scanRows(rows)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItemRows, nil
}

// GetIncompleteNotFailed gets incomplete graceful exit transfer queue entries that have failed <= maxFailures times, ordered by durability ratio and queued date ascending.
func (db *gracefulexitDB) GetIncompleteFailed(ctx context.Context, nodeID storj.NodeID, maxFailures int, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)

	sql := `
			SELECT
				node_id, stream_id, position,
				piece_num, root_piece_id, durability_ratio,
				queued_at, requested_at, last_failed_at,
				last_failed_code, failed_count, finished_at,
				order_limit_send_count
			FROM graceful_exit_segment_transfer_queue
			WHERE node_id = ?
				AND finished_at IS NULL
				AND last_failed_at IS NOT NULL
				AND failed_count < ?
			ORDER BY durability_ratio asc, queued_at asc LIMIT ? OFFSET ?`
	rows, err := db.db.Query(ctx, db.db.Rebind(sql), nodeID.Bytes(), maxFailures, limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, rows.Close()) }()

	transferQueueItemRows, err := scanRows(rows)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItemRows, nil
}

// IncrementOrderLimitSendCount increments the number of times a node has been sent an order limit for transferring.
func (db *gracefulexitDB) IncrementOrderLimitSendCount(ctx context.Context, nodeID storj.NodeID, streamID uuid.UUID, position metabase.SegmentPosition, pieceNum int32) (err error) {
	defer mon.Task()(&ctx)(&err)

	sql := `UPDATE graceful_exit_segment_transfer_queue SET order_limit_send_count = graceful_exit_segment_transfer_queue.order_limit_send_count + 1
			WHERE node_id = ?
			AND stream_id = ?
			AND position = ?
			AND piece_num = ?`
	_, err = db.db.ExecContext(ctx, db.db.Rebind(sql), nodeID, streamID, position.Encode(), pieceNum)

	return Error.Wrap(err)
}

// CountFinishedTransferQueueItemsByNode return a map of the nodes which has
// finished the exit before the indicated time but there are at least one item
// left in the transfer queue.
func (db *gracefulexitDB) CountFinishedTransferQueueItemsByNode(ctx context.Context, before time.Time, asOfSystemInterval time.Duration) (_ map[storj.NodeID]int64, err error) {
	defer mon.Task()(&ctx)(&err)

	query := `SELECT n.id, count(getq.node_id)
		FROM nodes as n INNER JOIN graceful_exit_segment_transfer_queue as getq
			ON n.id = getq.node_id
		` + db.db.impl.AsOfSystemInterval(asOfSystemInterval) + `
		WHERE n.exit_finished_at IS NOT NULL
			AND n.exit_finished_at < ?
		GROUP BY n.id`

	statement := db.db.Rebind(query)

	rows, err := db.db.QueryContext(ctx, statement, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, Error.Wrap(rows.Close())) }()

	nodesItemsCount := make(map[storj.NodeID]int64)
	for rows.Next() {
		var (
			nodeID storj.NodeID
			n      int64
		)
		err := rows.Scan(&nodeID, &n)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		nodesItemsCount[nodeID] = n
	}

	return nodesItemsCount, Error.Wrap(rows.Err())
}

func scanRows(rows tagsql.Rows) (transferQueueItemRows []*gracefulexit.TransferQueueItem, err error) {
	for rows.Next() {
		transferQueueItem := &gracefulexit.TransferQueueItem{}
		var pieceIDBytes []byte
		err = rows.Scan(&transferQueueItem.NodeID, &transferQueueItem.StreamID, &transferQueueItem.Position, &transferQueueItem.PieceNum, &pieceIDBytes,
			&transferQueueItem.DurabilityRatio, &transferQueueItem.QueuedAt, &transferQueueItem.RequestedAt, &transferQueueItem.LastFailedAt,
			&transferQueueItem.LastFailedCode, &transferQueueItem.FailedCount, &transferQueueItem.FinishedAt, &transferQueueItem.OrderLimitSendCount)

		if err != nil {
			return nil, Error.Wrap(err)
		}
		if pieceIDBytes != nil {
			transferQueueItem.RootPieceID, err = storj.PieceIDFromBytes(pieceIDBytes)
			if err != nil {
				return nil, Error.Wrap(err)
			}
		}

		transferQueueItemRows = append(transferQueueItemRows, transferQueueItem)
	}
	return transferQueueItemRows, Error.Wrap(rows.Err())
}

func dbxSegmentTransferToTransferQueueItem(dbxSegmentTransfer *dbx.GracefulExitSegmentTransfer) (item *gracefulexit.TransferQueueItem, err error) {
	nID, err := storj.NodeIDFromBytes(dbxSegmentTransfer.NodeId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	streamID, err := uuid.FromBytes(dbxSegmentTransfer.StreamId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	position := metabase.SegmentPositionFromEncoded(dbxSegmentTransfer.Position)

	item = &gracefulexit.TransferQueueItem{
		NodeID:              nID,
		StreamID:            streamID,
		Position:            position,
		PieceNum:            int32(dbxSegmentTransfer.PieceNum),
		DurabilityRatio:     dbxSegmentTransfer.DurabilityRatio,
		QueuedAt:            dbxSegmentTransfer.QueuedAt,
		OrderLimitSendCount: dbxSegmentTransfer.OrderLimitSendCount,
	}
	if dbxSegmentTransfer.RootPieceId != nil {
		item.RootPieceID, err = storj.PieceIDFromBytes(dbxSegmentTransfer.RootPieceId)
		if err != nil {
			return nil, err
		}
	}
	if dbxSegmentTransfer.LastFailedCode != nil {
		item.LastFailedCode = dbxSegmentTransfer.LastFailedCode
	}
	if dbxSegmentTransfer.FailedCount != nil {
		item.FailedCount = dbxSegmentTransfer.FailedCount
	}
	if dbxSegmentTransfer.RequestedAt != nil && !dbxSegmentTransfer.RequestedAt.IsZero() {
		item.RequestedAt = dbxSegmentTransfer.RequestedAt
	}
	if dbxSegmentTransfer.LastFailedAt != nil && !dbxSegmentTransfer.LastFailedAt.IsZero() {
		item.LastFailedAt = dbxSegmentTransfer.LastFailedAt
	}
	if dbxSegmentTransfer.FinishedAt != nil && !dbxSegmentTransfer.FinishedAt.IsZero() {
		item.FinishedAt = dbxSegmentTransfer.FinishedAt
	}

	return item, nil
}
