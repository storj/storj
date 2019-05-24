// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"database/sql"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type containment struct {
	db *dbx.DB
}

// Get gets the pending audit by node id
func (containment *containment) Get(ctx context.Context, id pb.NodeID) (*audit.PendingAudit, error) {
	if id.IsZero() {
		return nil, audit.ContainError.New("node ID empty")
	}

	pending, err := containment.db.Get_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(id.Bytes()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, audit.ErrContainedNotFound.New(id.String())
		}
		return nil, audit.ContainError.Wrap(err)
	}

	return convertDBPending(pending)
}

// IncrementPending creates a new pending audit entry, or increases its reverify count if it already exists
func (containment *containment) IncrementPending(ctx context.Context, pendingAudit *audit.PendingAudit) error {
	tx, err := containment.db.Open(ctx)
	if err != nil {
		return audit.ContainError.Wrap(err)
	}

	existingAudit, err := tx.Get_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(pendingAudit.NodeID.Bytes()))
	if err == sql.ErrNoRows {
		statement := containment.db.Rebind(
			`INSERT INTO pending_audits (node_id, piece_id, stripe_index, share_size, expected_share_hash, reverify_count)
			VALUES (?, ?, ?, ?, ?, ?)`,
		)
		_, err = tx.Tx.ExecContext(ctx, statement,
			pendingAudit.NodeID.Bytes(), pendingAudit.PieceID.Bytes(), pendingAudit.StripeIndex, pendingAudit.ShareSize, pendingAudit.ExpectedShareHash, pendingAudit.ReverifyCount,
		)
		if err != nil {
			return audit.ContainError.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else if err == nil {
		if !bytes.Equal(existingAudit.ExpectedShareHash, pendingAudit.ExpectedShareHash) {
			return audit.ContainError.Wrap(errs.Combine(audit.ErrAlreadyExists.New(pendingAudit.NodeID.String()), tx.Rollback()))
		}
		statement := containment.db.Rebind(
			`UPDATE pending_audits SET reverify_count = pending_audits.reverify_count + 1
			WHERE pending_audits.node_id=?`,
		)
		_, err = tx.Tx.ExecContext(ctx, statement, pendingAudit.NodeID.Bytes())
		if err != nil {
			return audit.ContainError.Wrap(errs.Combine(err, tx.Rollback()))
		}
	} else {
		return audit.ContainError.Wrap(errs.Combine(err, tx.Rollback()))
	}

	updateContained := dbx.Node_Update_Fields{
		Contained: dbx.Node_Contained(true),
	}

	_, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(pendingAudit.NodeID.Bytes()), updateContained)
	if err != nil {
		return audit.ContainError.Wrap(errs.Combine(err, tx.Rollback()))
	}
	return audit.ContainError.Wrap(tx.Commit())
}

// Delete deletes the pending audit
func (containment *containment) Delete(ctx context.Context, id pb.NodeID) (bool, error) {
	if id.IsZero() {
		return false, audit.ContainError.New("node ID empty")
	}

	tx, err := containment.db.Open(ctx)
	if err != nil {
		return false, audit.ContainError.Wrap(err)
	}

	isDeleted, err := tx.Delete_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(id.Bytes()))
	if err != nil {
		return isDeleted, audit.ContainError.Wrap(errs.Combine(err, tx.Rollback()))
	}

	updateContained := dbx.Node_Update_Fields{
		Contained: dbx.Node_Contained(false),
	}

	_, err = tx.Update_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()), updateContained)
	if err != nil {
		return isDeleted, audit.ContainError.Wrap(errs.Combine(err, tx.Rollback()))
	}
	return isDeleted, audit.ContainError.Wrap(tx.Commit())
}

func convertDBPending(info *dbx.PendingAudits) (*audit.PendingAudit, error) {
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
	}
	return pending, nil
}
