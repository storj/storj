// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/lib/pq"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/satellite/gracefulexit"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type gracefulexitDB struct {
	db *satelliteDB
}

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
	if err == sql.ErrNoRows {
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

// Enqueue batch inserts graceful exit transfer queue entries it does not exist.
func (db *gracefulexitDB) Enqueue(ctx context.Context, items []gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)

	sort.Slice(items, func(i, k int) bool {
		compare := bytes.Compare(items[i].NodeID.Bytes(), items[k].NodeID.Bytes())
		if compare == 0 {
			return bytes.Compare(items[i].Path, items[k].Path) < 0
		}
		return compare < 0
	})

	var nodeIDs []storj.NodeID
	var paths [][]byte
	var pieceNums []int32
	var rootPieceIDs [][]byte
	var durabilities []float64
	for _, item := range items {
		nodeIDs = append(nodeIDs, item.NodeID)
		paths = append(paths, item.Path)
		pieceNums = append(pieceNums, item.PieceNum)
		rootPieceIDs = append(rootPieceIDs, item.RootPieceID.Bytes())
		durabilities = append(durabilities, item.DurabilityRatio)
	}

	_, err = db.db.ExecContext(ctx, db.db.Rebind(`
			INSERT INTO graceful_exit_transfer_queue(node_id, path, piece_num, root_piece_id, durability_ratio, queued_at)
			SELECT unnest($1::bytea[]), unnest($2::bytea[]), unnest($3::integer[]), unnest($4::bytea[]), unnest($5::float8[]), $6
			ON CONFLICT DO NOTHING;`), postgresNodeIDList(nodeIDs), pq.ByteaArray(paths), pq.Array(pieceNums), pq.ByteaArray(rootPieceIDs), pq.Array(durabilities), time.Now().UTC())

	return Error.Wrap(err)
}

// UpdateTransferQueueItem creates a graceful exit transfer queue entry.
func (db *gracefulexitDB) UpdateTransferQueueItem(ctx context.Context, item gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)
	update := dbx.GracefulExitTransferQueue_Update_Fields{
		DurabilityRatio: dbx.GracefulExitTransferQueue_DurabilityRatio(item.DurabilityRatio),
		LastFailedCode:  dbx.GracefulExitTransferQueue_LastFailedCode_Raw(item.LastFailedCode),
		FailedCount:     dbx.GracefulExitTransferQueue_FailedCount_Raw(item.FailedCount),
	}

	if item.RequestedAt != nil {
		update.RequestedAt = dbx.GracefulExitTransferQueue_RequestedAt_Raw(item.RequestedAt)
	}
	if item.LastFailedAt != nil {
		update.LastFailedAt = dbx.GracefulExitTransferQueue_LastFailedAt_Raw(item.LastFailedAt)
	}
	if item.FinishedAt != nil {
		update.FinishedAt = dbx.GracefulExitTransferQueue_FinishedAt_Raw(item.FinishedAt)
	}

	return db.db.UpdateNoReturn_GracefulExitTransferQueue_By_NodeId_And_Path_And_PieceNum(ctx,
		dbx.GracefulExitTransferQueue_NodeId(item.NodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(item.Path),
		dbx.GracefulExitTransferQueue_PieceNum(int(item.PieceNum)),
		update,
	)
}

// DeleteTransferQueueItem deletes a graceful exit transfer queue entry.
func (db *gracefulexitDB) DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte, pieceNum int32) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitTransferQueue_By_NodeId_And_Path_And_PieceNum(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()), dbx.GracefulExitTransferQueue_Path(path),
		dbx.GracefulExitTransferQueue_PieceNum(int(pieceNum)))
	return Error.Wrap(err)
}

// DeleteTransferQueueItem deletes a graceful exit transfer queue entries by nodeID.
func (db *gracefulexitDB) DeleteTransferQueueItems(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitTransferQueue_By_NodeId(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()))
	return Error.Wrap(err)
}

// DeleteFinishedTransferQueueItem deletes finiahed graceful exit transfer queue entries by nodeID.
func (db *gracefulexitDB) DeleteFinishedTransferQueueItems(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitTransferQueue_By_NodeId_And_FinishedAt_IsNot_Null(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()))
	return Error.Wrap(err)
}

// GetTransferQueueItem gets a graceful exit transfer queue entry.
func (db *gracefulexitDB) GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte, pieceNum int32) (_ *gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTransferQueue, err := db.db.Get_GracefulExitTransferQueue_By_NodeId_And_Path_And_PieceNum(ctx,
		dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(path),
		dbx.GracefulExitTransferQueue_PieceNum(int(pieceNum)))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	transferQueueItem, err := dbxToTransferQueueItem(dbxTransferQueue)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItem, Error.Wrap(err)
}

// GetIncomplete gets incomplete graceful exit transfer queue entries ordered by durability ratio and queued date ascending.
func (db *gracefulexitDB) GetIncomplete(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	sql := `SELECT node_id, path, piece_num, root_piece_id, durability_ratio, queued_at, requested_at, last_failed_at, last_failed_code, failed_count, finished_at, order_limit_send_count
			FROM graceful_exit_transfer_queue 
			WHERE node_id = ? 
			AND finished_at is NULL 
			ORDER BY durability_ratio asc, queued_at asc LIMIT ? OFFSET ?`
	rows, err := db.db.Query(db.db.Rebind(sql), nodeID.Bytes(), limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	transferQueueItemRows, err := scanRows(rows)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItemRows, nil
}

// GetIncompleteNotFailed gets incomplete graceful exit transfer queue entries that haven't failed, ordered by durability ratio and queued date ascending.
func (db *gracefulexitDB) GetIncompleteNotFailed(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	sql := `SELECT node_id, path, piece_num, root_piece_id, durability_ratio, queued_at, requested_at, last_failed_at, last_failed_code, failed_count, finished_at, order_limit_send_count
			FROM graceful_exit_transfer_queue 
			WHERE node_id = ? 
			AND finished_at is NULL
			AND last_failed_at is NULL
			ORDER BY durability_ratio asc, queued_at asc LIMIT ? OFFSET ?`
	rows, err := db.db.Query(db.db.Rebind(sql), nodeID.Bytes(), limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	transferQueueItemRows, err := scanRows(rows)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItemRows, nil
}

// GetIncompleteNotFailed gets incomplete graceful exit transfer queue entries that have failed <= maxFailures times, ordered by durability ratio and queued date ascending.
func (db *gracefulexitDB) GetIncompleteFailed(ctx context.Context, nodeID storj.NodeID, maxFailures int, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	sql := `SELECT node_id, path, piece_num, root_piece_id, durability_ratio, queued_at, requested_at, last_failed_at, last_failed_code, failed_count, finished_at, order_limit_send_count
			FROM graceful_exit_transfer_queue 
			WHERE node_id = ? 
			AND finished_at is NULL
			AND last_failed_at is not NULL
			AND failed_count < ?
			ORDER BY durability_ratio asc, queued_at asc LIMIT ? OFFSET ?`
	rows, err := db.db.Query(db.db.Rebind(sql), nodeID.Bytes(), maxFailures, limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	transferQueueItemRows, err := scanRows(rows)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItemRows, nil
}

// IncrementOrderLimitSendCount increments the number of times a node has been sent an order limit for transferring.
func (db *gracefulexitDB) IncrementOrderLimitSendCount(ctx context.Context, nodeID storj.NodeID, path []byte, pieceNum int32) (err error) {
	defer mon.Task()(&ctx)(&err)

	sql := db.db.Rebind(
		`UPDATE graceful_exit_transfer_queue SET order_limit_send_count = graceful_exit_transfer_queue.order_limit_send_count + 1
		WHERE node_id = ?
		AND path = ?
		AND piece_num = ?`,
	)
	_, err = db.db.ExecContext(ctx, sql, nodeID, path, pieceNum)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func scanRows(rows *sql.Rows) (transferQueueItemRows []*gracefulexit.TransferQueueItem, err error) {
	for rows.Next() {
		transferQueueItem := &gracefulexit.TransferQueueItem{}
		var pieceIDBytes []byte
		err = rows.Scan(&transferQueueItem.NodeID, &transferQueueItem.Path, &transferQueueItem.PieceNum, &pieceIDBytes,
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
	return transferQueueItemRows, nil
}

func dbxToTransferQueueItem(dbxTransferQueue *dbx.GracefulExitTransferQueue) (item *gracefulexit.TransferQueueItem, err error) {
	nID, err := storj.NodeIDFromBytes(dbxTransferQueue.NodeId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	item = &gracefulexit.TransferQueueItem{
		NodeID:              nID,
		Path:                dbxTransferQueue.Path,
		PieceNum:            int32(dbxTransferQueue.PieceNum),
		DurabilityRatio:     dbxTransferQueue.DurabilityRatio,
		QueuedAt:            dbxTransferQueue.QueuedAt,
		OrderLimitSendCount: dbxTransferQueue.OrderLimitSendCount,
	}
	if dbxTransferQueue.RootPieceId != nil {
		item.RootPieceID, err = storj.PieceIDFromBytes(dbxTransferQueue.RootPieceId)
		if err != nil {
			return nil, err
		}
	}
	if dbxTransferQueue.LastFailedCode != nil {
		item.LastFailedCode = dbxTransferQueue.LastFailedCode
	}
	if dbxTransferQueue.FailedCount != nil {
		item.FailedCount = dbxTransferQueue.FailedCount
	}
	if dbxTransferQueue.RequestedAt != nil && !dbxTransferQueue.RequestedAt.IsZero() {
		item.RequestedAt = dbxTransferQueue.RequestedAt
	}
	if dbxTransferQueue.LastFailedAt != nil && !dbxTransferQueue.LastFailedAt.IsZero() {
		item.LastFailedAt = dbxTransferQueue.LastFailedAt
	}
	if dbxTransferQueue.FinishedAt != nil && !dbxTransferQueue.FinishedAt.IsZero() {
		item.FinishedAt = dbxTransferQueue.FinishedAt
	}

	return item, nil
}
