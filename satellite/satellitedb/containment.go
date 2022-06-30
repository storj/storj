// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"
	"errors"

	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type containment struct {
	db *satelliteDB
}

// Get gets the pending audit by node id.
func (containment *containment) Get(ctx context.Context, id pb.NodeID) (_ *audit.PendingAudit, err error) {
	defer mon.Task()(&ctx)(&err)
	if id.IsZero() {
		return nil, audit.ContainError.New("node ID empty")
	}

	pending, err := containment.db.Get_SegmentPendingAudits_By_NodeId(ctx, dbx.SegmentPendingAudits_NodeId(id.Bytes()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, audit.ErrContainedNotFound.New("%v", id)
		}
		return nil, audit.ContainError.Wrap(err)
	}

	return convertDBPending(ctx, pending)
}

// IncrementPending creates a new pending audit entry, or increases its reverify count if it already exists.
func (containment *containment) IncrementPending(ctx context.Context, pendingAudit *audit.PendingAudit) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = containment.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		existingAudit, err := tx.Get_SegmentPendingAudits_By_NodeId(ctx, dbx.SegmentPendingAudits_NodeId(pendingAudit.NodeID.Bytes()))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			statement := containment.db.Rebind(
				`INSERT INTO segment_pending_audits (
					node_id, piece_id, stripe_index, share_size, expected_share_hash, reverify_count, stream_id, position
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement,
				pendingAudit.NodeID.Bytes(), pendingAudit.PieceID.Bytes(), pendingAudit.StripeIndex,
				pendingAudit.ShareSize, pendingAudit.ExpectedShareHash, pendingAudit.ReverifyCount,
				pendingAudit.StreamID, pendingAudit.Position.Encode())
			if err != nil {
				return err
			}
		case err == nil:
			if !bytes.Equal(existingAudit.ExpectedShareHash, pendingAudit.ExpectedShareHash) {
				containment.db.log.Info("pending audit already exists",
					zap.String("node id", pendingAudit.NodeID.String()),
					zap.String("segment streamid", pendingAudit.StreamID.String()),
					zap.Uint64("segment position", pendingAudit.Position.Encode()),
				)
				return nil
			}
			statement := tx.Rebind(
				`UPDATE segment_pending_audits SET reverify_count = segment_pending_audits.reverify_count + 1
				WHERE segment_pending_audits.node_id=?`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement, pendingAudit.NodeID.Bytes())
			if err != nil {
				return err
			}
		default:
			return err
		}
		return nil
	})
	return audit.ContainError.Wrap(err)
}

// Delete deletes the pending audit.
func (containment *containment) Delete(ctx context.Context, id pb.NodeID) (isDeleted bool, err error) {
	defer mon.Task()(&ctx)(&err)
	if id.IsZero() {
		return false, audit.ContainError.New("node ID empty")
	}

	err = containment.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		isDeleted, err = tx.Delete_SegmentPendingAudits_By_NodeId(ctx, dbx.SegmentPendingAudits_NodeId(id.Bytes()))
		return err
	})
	return isDeleted, audit.ContainError.Wrap(err)
}

func convertDBPending(ctx context.Context, info *dbx.SegmentPendingAudits) (_ *audit.PendingAudit, err error) {
	defer mon.Task()(&ctx)(&err)
	if info == nil {
		return nil, Error.New("missing info")
	}

	nodeID, err := storj.NodeIDFromBytes(info.NodeId)
	if err != nil {
		return nil, audit.ContainError.Wrap(err)
	}

	pieceID, err := storj.PieceIDFromBytes(info.PieceId)
	if err != nil {
		return nil, audit.ContainError.Wrap(err)
	}

	streamID, err := uuid.FromBytes(info.StreamId)
	if err != nil {
		return nil, audit.ContainError.Wrap(err)
	}

	position := metabase.SegmentPositionFromEncoded(info.Position)

	pending := &audit.PendingAudit{
		NodeID:            nodeID,
		PieceID:           pieceID,
		StripeIndex:       int32(info.StripeIndex),
		ShareSize:         int32(info.ShareSize),
		ExpectedShareHash: info.ExpectedShareHash,
		ReverifyCount:     int32(info.ReverifyCount),
		StreamID:          streamID,
		Position:          position,
	}
	return pending, nil
}
