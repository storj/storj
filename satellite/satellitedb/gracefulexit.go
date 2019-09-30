// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"sort"
	"time"

	"github.com/lib/pq"
	sqlite3 "github.com/mattn/go-sqlite3"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/gracefulexit"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type gracefulexitDB struct {
	db *dbx.DB
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
	if err != nil {
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

	switch t := db.db.Driver().(type) {
	case *sqlite3.SQLiteDriver:
		statement := db.db.Rebind(
			`INSERT INTO graceful_exit_transfer_queue(node_id, path, piece_num, durability_ratio, queued_at)
			 VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING;`,
		)
		for _, item := range items {
			_, err = db.db.ExecContext(ctx, statement,
				item.NodeID.Bytes(), item.Path, item.PieceNum, item.DurabilityRatio, time.Now().UTC())
			if err != nil {
				return Error.Wrap(err)
			}
		}
	case *pq.Driver:
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
		var durabilities []float64
		for _, item := range items {
			nodeIDs = append(nodeIDs, item.NodeID)
			paths = append(paths, item.Path)
			pieceNums = append(pieceNums, item.PieceNum)
			durabilities = append(durabilities, item.DurabilityRatio)
		}

		_, err := db.db.ExecContext(ctx, `
			INSERT INTO graceful_exit_transfer_queue(node_id, path, piece_num, durability_ratio, queued_at)
			SELECT unnest($1::bytea[]), unnest($2::bytea[]), unnest($3::integer[]), unnest($4::float8[]), $5
			ON CONFLICT DO NOTHING;`, postgresNodeIDList(nodeIDs), pq.ByteaArray(paths), pq.Array(pieceNums), pq.Array(durabilities), time.Now().UTC())
		if err != nil {
			return Error.Wrap(err)
		}
	default:
		return Error.New("Unsupported database %t", t)
	}

	return nil
}

// UpdateTransferQueueItem creates a graceful exit transfer queue entry.
func (db *gracefulexitDB) UpdateTransferQueueItem(ctx context.Context, item gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)
	update := dbx.GracefulExitTransferQueue_Update_Fields{
		DurabilityRatio: dbx.GracefulExitTransferQueue_DurabilityRatio(item.DurabilityRatio),
		LastFailedCode:  dbx.GracefulExitTransferQueue_LastFailedCode_Raw(&item.LastFailedCode),
		FailedCount:     dbx.GracefulExitTransferQueue_FailedCount_Raw(&item.FailedCount),
	}

	if !item.RequestedAt.IsZero() {
		update.RequestedAt = dbx.GracefulExitTransferQueue_RequestedAt_Raw(&item.RequestedAt)
	}
	if !item.LastFailedAt.IsZero() {
		update.LastFailedAt = dbx.GracefulExitTransferQueue_LastFailedAt_Raw(&item.LastFailedAt)
	}
	if !item.FinishedAt.IsZero() {
		update.FinishedAt = dbx.GracefulExitTransferQueue_FinishedAt_Raw(&item.FinishedAt)
	}

	return db.db.UpdateNoReturn_GracefulExitTransferQueue_By_NodeId_And_Path(ctx,
		dbx.GracefulExitTransferQueue_NodeId(item.NodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(item.Path),
		update,
	)
}

// DeleteTransferQueueItem deletes a graceful exit transfer queue entry.
func (db *gracefulexitDB) DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitTransferQueue_By_NodeId_And_Path(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()), dbx.GracefulExitTransferQueue_Path(path))
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
func (db *gracefulexitDB) GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (_ *gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTransferQueue, err := db.db.Get_GracefulExitTransferQueue_By_NodeId_And_Path(ctx,
		dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(path))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	transferQueueItem, err := dbxToTransferQueueItem(dbxTransferQueue)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return transferQueueItem, Error.Wrap(err)
}

// GetIncomplete gets incomplete graceful exit transfer queue entries in the database ordered by the queued date ascending.
func (db *gracefulexitDB) GetIncomplete(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTransferQueueItemRows, err := db.db.Limited_GracefulExitTransferQueue_By_NodeId_And_FinishedAt_Is_Null_OrderBy_Asc_QueuedAt(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()), limit, offset)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var transferQueueItemRows = make([]*gracefulexit.TransferQueueItem, len(dbxTransferQueueItemRows))
	for i, dbxTransferQueue := range dbxTransferQueueItemRows {
		transferQueueItem, err := dbxToTransferQueueItem(dbxTransferQueue)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		transferQueueItemRows[i] = transferQueueItem
	}

	return transferQueueItemRows, nil
}

func dbxToTransferQueueItem(dbxTransferQueue *dbx.GracefulExitTransferQueue) (item *gracefulexit.TransferQueueItem, err error) {
	nID, err := storj.NodeIDFromBytes(dbxTransferQueue.NodeId)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	item = &gracefulexit.TransferQueueItem{
		NodeID:          nID,
		Path:            dbxTransferQueue.Path,
		PieceNum:        int32(dbxTransferQueue.PieceNum),
		DurabilityRatio: dbxTransferQueue.DurabilityRatio,
		QueuedAt:        dbxTransferQueue.QueuedAt,
	}
	if dbxTransferQueue.LastFailedCode != nil {
		item.LastFailedCode = *dbxTransferQueue.LastFailedCode
	}
	if dbxTransferQueue.FailedCount != nil {
		item.FailedCount = *dbxTransferQueue.FailedCount
	}
	if dbxTransferQueue.RequestedAt != nil && !dbxTransferQueue.RequestedAt.IsZero() {
		item.RequestedAt = *dbxTransferQueue.RequestedAt
	}
	if dbxTransferQueue.LastFailedAt != nil && !dbxTransferQueue.LastFailedAt.IsZero() {
		item.LastFailedAt = *dbxTransferQueue.LastFailedAt
	}
	if dbxTransferQueue.FinishedAt != nil && !dbxTransferQueue.FinishedAt.IsZero() {
		item.FinishedAt = *dbxTransferQueue.FinishedAt
	}

	return item, nil
}
