// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/satellite/audit"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type containment struct {
	db *satelliteDB
}

// Get gets the pending audit by node id
func (containment *containment) Get(ctx context.Context, id pb.NodeID) (_ *audit.PendingAudit, err error) {
	defer mon.Task()(&ctx)(&err)
	if id.IsZero() {
		return nil, audit.ContainError.New("node ID empty")
	}

	pending, err := containment.db.Get_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(id.Bytes()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, audit.ErrContainedNotFound.New("%v", id)
		}
		return nil, audit.ContainError.Wrap(err)
	}

	return convertDBPending(ctx, pending)
}

// IncrementPending creates a new pending audit entry, or increases its reverify count if it already exists
func (containment *containment) IncrementPending(ctx context.Context, pendingAudit *audit.PendingAudit) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = containment.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		existingAudit, err := tx.Get_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(pendingAudit.NodeID.Bytes()))
		switch err {
		case sql.ErrNoRows:
			statement := containment.db.Rebind(
				`INSERT INTO pending_audits (node_id, piece_id, stripe_index, share_size, expected_share_hash, reverify_count, path)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement, pendingAudit.NodeID.Bytes(), pendingAudit.PieceID.Bytes(), pendingAudit.StripeIndex,
				pendingAudit.ShareSize, pendingAudit.ExpectedShareHash, pendingAudit.ReverifyCount, []byte(pendingAudit.Path))
			if err != nil {
				return err
			}
		case nil:
			if !bytes.Equal(existingAudit.ExpectedShareHash, pendingAudit.ExpectedShareHash) {
				return audit.ErrAlreadyExists.New("%v", pendingAudit.NodeID)
			}
			statement := containment.db.Rebind(
				`UPDATE pending_audits SET reverify_count = pending_audits.reverify_count + 1
			WHERE pending_audits.node_id=?`,
			)
			_, err = tx.Tx.ExecContext(ctx, statement, pendingAudit.NodeID.Bytes())
			if err != nil {
				return err
			}
		default:
			return err
		}

		updateContained := dbx.Node_Update_Fields{
			Contained: dbx.Node_Contained(true),
		}

		return tx.UpdateNoReturn_Node_By_Id(ctx, dbx.Node_Id(pendingAudit.NodeID.Bytes()), updateContained)
	})
	return audit.ContainError.Wrap(err)
}

// Delete deletes the pending audit
func (containment *containment) Delete(ctx context.Context, id pb.NodeID) (isDeleted bool, err error) {
	defer mon.Task()(&ctx)(&err)
	if id.IsZero() {
		return false, audit.ContainError.New("node ID empty")
	}

	err = containment.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) (err error) {
		isDeleted, err = tx.Delete_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(id.Bytes()))
		if err != nil {
			return err
		}

		updateContained := dbx.Node_Update_Fields{
			Contained: dbx.Node_Contained(false),
		}

		return tx.UpdateNoReturn_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()), updateContained)
	})
	return isDeleted, audit.ContainError.Wrap(err)
}

func convertDBPending(ctx context.Context, info *dbx.PendingAudits) (_ *audit.PendingAudit, err error) {
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

	pending := &audit.PendingAudit{
		NodeID:            nodeID,
		PieceID:           pieceID,
		StripeIndex:       info.StripeIndex,
		ShareSize:         int32(info.ShareSize),
		ExpectedShareHash: info.ExpectedShareHash,
		ReverifyCount:     int32(info.ReverifyCount),
		Path:              string(info.Path),
	}
	return pending, nil
}
