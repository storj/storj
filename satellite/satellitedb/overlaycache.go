// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var _ overlay.CacheDB = (*overlaycache)(nil)

type overlaycache struct {
	db *dbx.DB
}

func (cache *overlaycache) Get(ctx context.Context, id storj.NodeID) (*pb.Node, error) {
	if id.IsZero() {
		return nil, overlay.ErrEmptyNode
	}

	node, err := cache.db.Get_OverlayCacheNode_By_NodeId(ctx,
		dbx.OverlayCacheNode_NodeId(id.Bytes()),
	)
	if err == sql.ErrNoRows {
		return nil, overlay.ErrNodeNotFound
	}
	return convertOverlayNode(node), err
}

func (cache *overlaycache) GetAll(ctx context.Context, ids storj.NodeIDList) ([]*pb.Node, error) {
	infos := make([]*pb.Node, len(ids))
	for i, id := range ids {
		// TODO: abort on canceled context
		info, err := cache.Get(ctx, id)
		if err != nil {
			continue
		}
		infos[i] = info
	}
	return infos, nil
}

func (cache *overlaycache) List(ctx context.Context, cursor storj.NodeID, limit int) ([]*pb.Node, error) {
	dbxInfos, err := cache.db.Limited_OverlayCacheNode_By_NodeId_GreaterOrEqual(ctx,
		dbx.OverlayCacheNode_NodeId(cursor.Bytes()),
		limit, 0,
	)
	if err != nil {
		return nil, err
	}

	infos := make([]*pb.Node, len(dbxInfos))
	for i, dbxInfo := range dbxInfos {
		infos[i] = convertOverlayNode(dbxInfo)
	}
	return infos, nil
}

func (cache *overlaycache) Update(ctx context.Context, info *pb.Node) (err error) {
	if info.Id.IsZero() {
		return overlay.ErrEmptyNode
	}

	tx, err := cache.db.Open(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	// TODO: use upsert
	_, err = tx.Get_OverlayCacheNode_By_NodeId(ctx,
		dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),
	)
	if err != nil {
		_, err = tx.Create_OverlayCacheNode(
			ctx,
			dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),

			dbx.OverlayCacheNode_NodeType(int(info.Type)),
			dbx.OverlayCacheNode_Address(info.Address.Address),
			dbx.OverlayCacheNode_Protocol(int(info.Address.Transport)),

			dbx.OverlayCacheNode_OperatorEmail(info.Metadata.Email),
			dbx.OverlayCacheNode_OperatorWallet(info.Metadata.Wallet),

			dbx.OverlayCacheNode_FreeBandwidth(info.Restrictions.FreeBandwidth),
			dbx.OverlayCacheNode_FreeDisk(info.Restrictions.FreeDisk),

			dbx.OverlayCacheNode_Latency90(info.Reputation.Latency_90),
			dbx.OverlayCacheNode_AuditSuccessRatio(info.Reputation.AuditSuccessRatio),
			dbx.OverlayCacheNode_AuditUptimeRatio(info.Reputation.UptimeRatio),
			dbx.OverlayCacheNode_AuditCount(info.Reputation.AuditCount),
			dbx.OverlayCacheNode_AuditSuccessCount(info.Reputation.AuditSuccessCount),

			dbx.OverlayCacheNode_UptimeCount(info.Reputation.UptimeCount),
			dbx.OverlayCacheNode_UptimeSuccessCount(info.Reputation.UptimeSuccessCount),
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		_, err := tx.Update_OverlayCacheNode_By_NodeId(
			ctx,
			dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),
			dbx.OverlayCacheNode_Update_Fields{
				dbx.OverlayCacheNode_Address(info.Address.Address),
				dbx.OverlayCacheNode_Protocol(int(info.Address.Transport)),

				dbx.OverlayCacheNode_OperatorEmail(info.Metadata.Email),
				dbx.OverlayCacheNode_OperatorWallet(info.Metadata.Wallet),

				dbx.OverlayCacheNode_FreeBandwidth(info.Restrictions.FreeBandwidth),
				dbx.OverlayCacheNode_FreeDisk(info.Restrictions.FreeDisk),

				dbx.OverlayCacheNode_Latency90(info.Reputation.Latency_90),
				dbx.OverlayCacheNode_AuditSuccessRatio(info.Reputation.AuditSuccessRatio),
				dbx.OverlayCacheNode_AuditUptimeRatio(info.Reputation.UptimeRatio),
				dbx.OverlayCacheNode_AuditCount(info.Reputation.AuditCount),
				dbx.OverlayCacheNode_AuditSuccessCount(info.Reputation.AuditSuccessCount),
				dbx.OverlayCacheNode_UptimeCount(info.Reputation.UptimeCount),
				dbx.OverlayCacheNode_UptimeSuccessCount(info.Reputation.UptimeSuccessCount),
			},
		)
		if err != nil {
			return Error.Wrap(errs.Combine(err, tx.Rollback()))
		}
	}
	return Error.Wrap(tx.Commit())
}

func (cache *overlaycache) Delete(ctx context.Context, id storj.NodeID) error {
	_, err := cache.db.Delete_OverlayCacheNode_By_NodeId(ctx,
		dbx.OverlayCacheNode_NodeId(id.Bytes()),
	)
	return err
}

func convertOverlayNode(node *dbx.OverlayCacheNode) *pb.Node {
	if node == nil {
		return nil
	}
	// TODO:
	return nil
}
