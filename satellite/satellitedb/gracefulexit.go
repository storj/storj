// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/gracefulexit"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type gracefulexitDB struct {
	db *dbx.DB
}

// CreateProgress creates a graceful exit progress entry in the database.
func (db *gracefulexitDB) CreateProgress(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return db.db.CreateNoReturn_GracefulExitProgress(
		ctx,
		dbx.GracefulExitProgress_NodeId(nodeID.Bytes()),
		dbx.GracefulExitProgress_BytesTransferred(0),
	)
}

// UpdateProgress updates a graceful exit progress entry in the database.
func (db *gracefulexitDB) UpdateProgress(ctx context.Context, nodeID storj.NodeID, bytesTransferred int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	return db.db.UpdateNoReturn_GracefulExitProgress_By_NodeId(
		ctx,
		dbx.GracefulExitProgress_NodeId(nodeID.Bytes()),
		dbx.GracefulExitProgress_Update_Fields{
			BytesTransferred: dbx.GracefulExitProgress_BytesTransferred(bytesTransferred),
		},
	)
}

// DeleteProgress deletes a graceful exit progress entry in the database.
func (db *gracefulexitDB) DeleteProgress(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitProgress_By_NodeId(ctx, dbx.GracefulExitProgress_NodeId(nodeID.Bytes()))
	return err
}

// IncrementProgressBytesTransferred increments bytes transferred value.
func (db *gracefulexitDB) IncrementProgressBytesTransferred(ctx context.Context, nodeID storj.NodeID, bytesTransferred int64) (err error) {
	defer mon.Task()(&ctx)(&err)
	statement := db.db.Rebind(
		`UPDATE graceful_exit_progress SET bytes_transferred = bytes_transferred + ? WHERE node_id = ?`,
	)
	_, err = db.db.ExecContext(ctx, statement, uint64(bytesTransferred), nodeID)
	if err != nil {
		return err
	}

	return nil
}

// GetProgress gets a graceful exit progress entry in the database.
func (db *gracefulexitDB) GetProgress(ctx context.Context, nodeID storj.NodeID) (_ *gracefulexit.Progress, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxProgress, err := db.db.Get_GracefulExitProgress_By_NodeId(ctx, dbx.GracefulExitProgress_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}
	nID, err := storj.NodeIDFromBytes(dbxProgress.NodeId)
	if err != nil {
		return nil, err
	}

	progress := &gracefulexit.Progress{
		NodeID:           nID,
		BytesTransferred: dbxProgress.BytesTransferred,
		UpdatedAt:        dbxProgress.UpdatedAt,
	}

	return progress, err
}

// GetAllProgress gets all graceful exit progress entries in the database.
func (db *gracefulexitDB) GetAllProgress(ctx context.Context) (_ []*gracefulexit.Progress, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxProgressRows, err := db.db.All_GracefulExitProgress(ctx)
	if err != nil {
		return nil, err
	}

	var progressRows = make([]*gracefulexit.Progress, len(dbxProgressRows))
	for i, dbxProgress := range dbxProgressRows {
		nID, err := storj.NodeIDFromBytes(dbxProgress.NodeId)
		if err != nil {
			return nil, err
		}

		progress := &gracefulexit.Progress{
			NodeID:           nID,
			BytesTransferred: dbxProgress.BytesTransferred,
			UpdatedAt:        dbxProgress.UpdatedAt,
		}
		progressRows[i] = progress
	}
	return progressRows, err
}

// CreateTransferQueueItem creates a graceful exit transfer queue entry in the database.
func (db *gracefulexitDB) CreateTransferQueueItem(ctx context.Context, item gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)

	optional := dbx.GracefulExitTransferQueue_Create_Fields{
		LastFailedCode: dbx.GracefulExitTransferQueue_LastFailedCode_Raw(&item.LastFailedCode),
		FailedCount:    dbx.GracefulExitTransferQueue_FailedCount_Raw(&item.FailedCount),
	}
	if !item.RequestedAt.IsZero() {
		optional.RequestedAt = dbx.GracefulExitTransferQueue_RequestedAt_Raw(&item.RequestedAt)
	}
	if !item.LastFailedAt.IsZero() {
		optional.LastFailedAt = dbx.GracefulExitTransferQueue_LastFailedAt_Raw(&item.LastFailedAt)
	}
	if !item.FinishedAt.IsZero() {
		optional.FinishedAt = dbx.GracefulExitTransferQueue_FinishedAt_Raw(&item.FinishedAt)
	}

	return db.db.CreateNoReturn_GracefulExitTransferQueue(ctx,
		dbx.GracefulExitTransferQueue_NodeId(item.NodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(item.Path),
		dbx.GracefulExitTransferQueue_PieceNum(item.PieceNum),
		dbx.GracefulExitTransferQueue_DurabilityRatio(item.DurabilityRatio),
		optional,
	)
}

// UpdateTransferQueueItem creates a graceful exit transfer queue entry in the database.
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

// DeleteTransferQueueItem deletes a graceful exit transfer queue entry in the database.
func (db *gracefulexitDB) DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitTransferQueue_By_NodeId_And_Path(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()), dbx.GracefulExitTransferQueue_Path(path))
	return err
}

// GetTransferQueueItem gets a graceful exit transfer queue entry in the database.
func (db *gracefulexitDB) GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (_ *gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTransferQueue, err := db.db.Get_GracefulExitTransferQueue_By_NodeId_And_Path(ctx,
		dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(path))
	if err != nil {
		return nil, err
	}

	transferQueueItem, err := dbxToTransferQueueItem(dbxTransferQueue)
	if err != nil {
		return nil, err
	}

	return transferQueueItem, err
}

// GetIncompleteTransferQueueItemsByNodeIDWithLimits gets incomplete graceful exit transfer queue entries in the database ordered by the queued date ascending.
func (db *gracefulexitDB) GetIncompleteTransferQueueItemsByNodeIDWithLimits(ctx context.Context, nodeID storj.NodeID, limit int, offset int64) (_ []*gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTransferQueueItemRows, err := db.db.Limited_GracefulExitTransferQueue_By_NodeId_And_FinishedAt_Is_Null_OrderBy_Asc_QueuedAt(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()), limit, offset)
	if err != nil {
		return nil, err
	}

	var transferQueueItemRows = make([]*gracefulexit.TransferQueueItem, len(dbxTransferQueueItemRows))
	for i, dbxTransferQueue := range dbxTransferQueueItemRows {
		transferQueueItem, err := dbxToTransferQueueItem(dbxTransferQueue)
		if err != nil {
			return nil, err
		}
		transferQueueItemRows[i] = transferQueueItem
	}

	return transferQueueItemRows, err
}

func dbxToTransferQueueItem(dbxTransferQueue *dbx.GracefulExitTransferQueue) (item *gracefulexit.TransferQueueItem, err error) {
	nID, err := storj.NodeIDFromBytes(dbxTransferQueue.NodeId)
	if err != nil {
		return nil, err
	}

	item = &gracefulexit.TransferQueueItem{
		NodeID:          nID,
		Path:            dbxTransferQueue.Path,
		PieceNum:        dbxTransferQueue.PieceNum,
		DurabilityRatio: dbxTransferQueue.DurabilityRatio,
		QueuedAt:        dbxTransferQueue.QueuedAt,
		LastFailedCode:  *dbxTransferQueue.LastFailedCode,
		FailedCount:     *dbxTransferQueue.FailedCount,
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

	return item, err
}
