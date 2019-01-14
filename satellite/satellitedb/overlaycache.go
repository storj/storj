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
	if err != nil {
		return nil, err
	}
	return convertOverlayNode(node)
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
		infos[i], err = convertOverlayNode(dbxInfo)
		if err != nil {
			return nil, err
		}
	}
	return infos, nil
}

func (cache *overlaycache) Update(ctx context.Context, info *pb.Node) (err error) {
	if info == nil || info.Id.IsZero() {
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

	metadata := info.Metadata
	if metadata == nil {
		metadata = &pb.NodeMetadata{}
	}

	restrictions := info.Restrictions
	if restrictions == nil {
		restrictions = &pb.NodeRestrictions{
			FreeBandwidth: -1,
			FreeDisk:      -1,
		}
	}

	if err != nil {
		_, err = tx.Create_OverlayCacheNode(
			ctx,
			dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),

			dbx.OverlayCacheNode_NodeType(int(info.Type)),
			dbx.OverlayCacheNode_Address(info.Address.Address),
			dbx.OverlayCacheNode_Protocol(int(info.Address.Transport)),

			dbx.OverlayCacheNode_OperatorEmail(metadata.Email),
			dbx.OverlayCacheNode_OperatorWallet(metadata.Wallet),

			dbx.OverlayCacheNode_FreeBandwidth(restrictions.FreeBandwidth),
			dbx.OverlayCacheNode_FreeDisk(restrictions.FreeDisk),

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
		update := dbx.OverlayCacheNode_Update_Fields{
			Address:  dbx.OverlayCacheNode_Address(info.Address.Address),
			Protocol: dbx.OverlayCacheNode_Protocol(int(info.Address.Transport)),

			Latency90:          dbx.OverlayCacheNode_Latency90(info.Reputation.Latency_90),
			AuditSuccessRatio:  dbx.OverlayCacheNode_AuditSuccessRatio(info.Reputation.AuditSuccessRatio),
			AuditUptimeRatio:   dbx.OverlayCacheNode_AuditUptimeRatio(info.Reputation.UptimeRatio),
			AuditCount:         dbx.OverlayCacheNode_AuditCount(info.Reputation.AuditCount),
			AuditSuccessCount:  dbx.OverlayCacheNode_AuditSuccessCount(info.Reputation.AuditSuccessCount),
			UptimeCount:        dbx.OverlayCacheNode_UptimeCount(info.Reputation.UptimeCount),
			UptimeSuccessCount: dbx.OverlayCacheNode_UptimeSuccessCount(info.Reputation.UptimeSuccessCount),
		}

		if info.Metadata != nil {
			update.OperatorEmail = dbx.OverlayCacheNode_OperatorEmail(info.Metadata.Email)
			update.OperatorWallet = dbx.OverlayCacheNode_OperatorWallet(info.Metadata.Wallet)
		}

		if info.Restrictions != nil {
			update.FreeBandwidth = dbx.OverlayCacheNode_FreeBandwidth(restrictions.FreeBandwidth)
			update.FreeDisk = dbx.OverlayCacheNode_FreeDisk(restrictions.FreeDisk)
		}

		_, err := tx.Update_OverlayCacheNode_By_NodeId(ctx,
			dbx.OverlayCacheNode_NodeId(info.Id.Bytes()),
			update,
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

func convertOverlayNode(info *dbx.OverlayCacheNode) (*pb.Node, error) {
	if info == nil {
		return nil, Error.New("missing info")
	}

	id, err := storj.NodeIDFromBytes(info.NodeId)
	if err != nil {
		return nil, err
	}

	node := &pb.Node{
		Id:   id,
		Type: pb.NodeType(info.NodeType),
		Address: &pb.NodeAddress{
			Address:   info.Address,
			Transport: pb.NodeTransport(info.Protocol),
		},
		Metadata: &pb.NodeMetadata{
			Email:  info.OperatorEmail,
			Wallet: info.OperatorWallet,
		},
		Restrictions: &pb.NodeRestrictions{
			FreeBandwidth: info.FreeBandwidth,
			FreeDisk:      info.FreeDisk,
		},
		Reputation: &pb.NodeStats{
			Latency_90:         info.Latency90,
			AuditSuccessRatio:  info.AuditSuccessRatio,
			UptimeRatio:        info.AuditUptimeRatio,
			AuditCount:         info.AuditCount,
			AuditSuccessCount:  info.AuditSuccessCount,
			UptimeCount:        info.UptimeCount,
			UptimeSuccessCount: info.UptimeSuccessCount,
		},
	}

	if node.Metadata.Email == "" && node.Metadata.Wallet == "" {
		node.Metadata = nil
	}
	if node.Restrictions.FreeBandwidth < 0 && node.Restrictions.FreeDisk < 0 {
		node.Restrictions = nil
	}

	return node, nil
}
