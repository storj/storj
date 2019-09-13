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

// CreateProgress creates a graceful exit progress entry in the database
func (db *gracefulexitDB) CreateProgress(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	return db.db.CreateNoReturn_GracefulExitProgress(
		ctx,
		dbx.GracefulExitProgress_NodeId(nodeID.Bytes()),
		dbx.GracefulExitProgress_BytesTransferred(0),
	)
}

// UpdateProgress updates a graceful exit progress entry in the database
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

// DeleteProgress deletes a graceful exit progress entry in the database
func (db *gracefulexitDB) DeleteProgress(ctx context.Context, nodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitProgress_By_NodeId(ctx, dbx.GracefulExitProgress_NodeId(nodeID.Bytes()))
	return err
}

// IncrementProgressBytesTransferred increments bytes transferred value
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

// GetProgress gets a graceful exit progress entry in the database
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

// CreateProgress creates a graceful exit transfer queue entry in the database
func (db *gracefulexitDB) CreateTransferQueueItem(ctx context.Context, item gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)

	optional := dbx.GracefulExitTransferQueue_Create_Fields{
		RequestedAt:    dbx.GracefulExitTransferQueue_RequestedAt_Raw(&item.RequestedAt),
		LastFailedAt:   dbx.GracefulExitTransferQueue_LastFailedAt_Raw(&item.LastFailedAt),
		LastFailedCode: dbx.GracefulExitTransferQueue_LastFailedCode_Raw(&item.LastFailedCode),
		FailedCount:    dbx.GracefulExitTransferQueue_FailedCount_Raw(&item.FailedCount),
		FinishedAt:     dbx.GracefulExitTransferQueue_FinishedAt_Raw(&item.FinishedAt),
	}

	return db.db.CreateNoReturn_GracefulExitTransferQueue(ctx,
		dbx.GracefulExitTransferQueue_NodeId(item.NodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(item.Path),
		dbx.GracefulExitTransferQueue_PieceNum(item.PieceNum),
		dbx.GracefulExitTransferQueue_DurabilityRatio(item.DurabilityRatio),
		optional,
	)
}

// UpdateTransferQueueItem creates a graceful exit transfer queue entry in the database
func (db *gracefulexitDB) UpdateTransferQueueItem(ctx context.Context, item gracefulexit.TransferQueueItem) (err error) {
	defer mon.Task()(&ctx)(&err)
	update := dbx.GracefulExitTransferQueue_Update_Fields{
		DurabilityRatio: dbx.GracefulExitTransferQueue_DurabilityRatio(item.DurabilityRatio),
		RequestedAt:     dbx.GracefulExitTransferQueue_RequestedAt_Raw(&item.RequestedAt),
		LastFailedAt:    dbx.GracefulExitTransferQueue_LastFailedAt_Raw(&item.LastFailedAt),
		LastFailedCode:  dbx.GracefulExitTransferQueue_LastFailedCode_Raw(&item.LastFailedCode),
		FailedCount:     dbx.GracefulExitTransferQueue_FailedCount_Raw(&item.FailedCount),
		FinishedAt:      dbx.GracefulExitTransferQueue_FinishedAt_Raw(&item.FinishedAt),
	}
	return db.db.UpdateNoReturn_GracefulExitTransferQueue_By_NodeId_And_Path(ctx,
		dbx.GracefulExitTransferQueue_NodeId(item.NodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(item.Path),
		update,
	)
}

// DeleteTransferQueueItem deletes a graceful exit transfer queue entry in the database
func (db *gracefulexitDB) DeleteTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = db.db.Delete_GracefulExitTransferQueue_By_NodeId_And_Path(ctx, dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()), dbx.GracefulExitTransferQueue_Path(path))
	return err
}

// GetTransferQueueItem gets a graceful exit transfer queue entry in the database
func (db *gracefulexitDB) GetTransferQueueItem(ctx context.Context, nodeID storj.NodeID, path []byte) (_ *gracefulexit.TransferQueueItem, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxTransferQueue, err := db.db.Get_GracefulExitTransferQueue_By_NodeId_And_Path(ctx,
		dbx.GracefulExitTransferQueue_NodeId(nodeID.Bytes()),
		dbx.GracefulExitTransferQueue_Path(path))
	if err != nil {
		return nil, err
	}
	nID, err := storj.NodeIDFromBytes(dbxTransferQueue.NodeId)
	if err != nil {
		return nil, err
	}

	transferQueueItem := &gracefulexit.TransferQueueItem{
		NodeID:          nID,
		Path:            dbxTransferQueue.Path,
		PieceNum:        dbxTransferQueue.PieceNum,
		DurabilityRatio: dbxTransferQueue.DurabilityRatio,
		QueuedAt:        dbxTransferQueue.QueuedAt,
		RequestedAt:     *dbxTransferQueue.RequestedAt,
		LastFailedAt:    *dbxTransferQueue.LastFailedAt,
		LastFailedCode:  *dbxTransferQueue.LastFailedCode,
		FailedCount:     *dbxTransferQueue.FailedCount,
		FinishedAt:      *dbxTransferQueue.FinishedAt,
	}

	return transferQueueItem, err
}
