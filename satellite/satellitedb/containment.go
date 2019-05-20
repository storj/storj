// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"

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
	statement := containment.db.Rebind(
		`INSERT INTO pending_audits (node_id, piece_id, stripe_index, share_size, expected_share_hash, reverify_count)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(node_id)
		DO UPDATE SET reverify_count = pending_audits.reverify_count + 1`,
	)
	_, err := containment.db.ExecContext(ctx, statement,
		pendingAudit.NodeID.Bytes(), pendingAudit.PieceID.Bytes(), pendingAudit.StripeIndex, pendingAudit.ShareSize, pendingAudit.ExpectedShareHash, pendingAudit.ReverifyCount,
	)
	return audit.ContainError.Wrap(err)
}

// Delete deletes the pending audit
func (containment *containment) Delete(ctx context.Context, id pb.NodeID) (bool, error) {
	if id.IsZero() {
		return false, audit.ContainError.New("node ID empty")
	}
	isDeleted, err := containment.db.Delete_PendingAudits_By_NodeId(ctx, dbx.PendingAudits_NodeId(id.Bytes()))
	return isDeleted, audit.ContainError.Wrap(err)
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
		StripeIndex:       int(info.StripeIndex),
		ShareSize:         info.ShareSize,
		ExpectedShareHash: info.ExpectedShareHash,
		ReverifyCount:     int(info.ReverifyCount),
	}
	return pending, nil
}
