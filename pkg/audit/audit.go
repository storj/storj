// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"io"

	"github.com/vivint/infectious"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
)

// Auditor struct
type Auditor struct {
	ps client.PSClient
}

// NewAuditor creates a new instance of an Auditor struct
func NewAuditor(ps client.PSClient) *Auditor {
	return &Auditor{ps: ps}
}

func (a *Auditor) getShare(ctx context.Context, stripeIndex, shareSize, stripeNumber int,
	id client.PieceID, pieceSize int64) (share infectious.Share, err error) {

	rr, err := a.ps.Get(ctx, id, pieceSize, &pb.PayerBandwidthAllocation{})
	if err != nil {
		return infectious.Share{}, err
	}

	offset := shareSize * stripeIndex

	rc, err := rr.Range(ctx, int64(offset), int64(shareSize))
	if err != nil {
		return infectious.Share{}, err
	}

	buf := make([]byte, shareSize)
	_, err = io.ReadFull(rc, buf)
	if err != io.EOF || err != io.ErrUnexpectedEOF {
		return infectious.Share{}, err
	}

	share = infectious.Share{
		Number: stripeNumber,
		Data:   buf,
	}
	return share, nil
}

func (a *Auditor) audit(ctx context.Context, required, total int, originals []infectious.Share) (badStripes []int, err error) {
	f, err := infectious.NewFEC(required, total)
	if err != nil {
		return nil, err
	}
	copies := make([]infectious.Share, len(originals))
	for _, copy := range originals {
		copies[copy.Number] = copy.DeepCopy()
	}
	err = f.Correct(copies)
	if err != nil {
		return nil, err
	}

	for _, share := range copies {
		if !bytes.Equal(originals[share.Number].Data, share.Data) {
			badStripes = append(badStripes, share.Number)
		}
	}
	return badStripes, nil
}

// runAudit gets remote segments from a pointer and runs an audit on shares
// at a given stripe index
func (a *Auditor) runAudit(ctx context.Context, p *pb.Pointer, stripeIndex, required, total int) (badStripes []int, err error) {
	var shares []infectious.Share
	shareSize := int(p.Remote.Redundancy.GetErasureShareSize())
	pieceID := client.PieceID(p.Remote.GetPieceId())

	for i, rp := range p.Remote.RemotePieces {
		derivedPieceID, err := pieceID.Derive([]byte(rp.NodeId))
		if err != nil {
			return nil, err
		}
		pieceSummary, err := a.ps.Meta(ctx, derivedPieceID)
		if err != nil {
			return nil, err
		}
		share, err := a.getShare(ctx, stripeIndex, shareSize, i, pieceID, pieceSummary.GetSize())
		if err != nil {
			// TODO(nat): update the statdb here to indicate this node failed the audit
			return nil, err
		}
		shares = append(shares, share)
	}

	badStripes, err = a.audit(ctx, required, total, shares)
	if err != nil {
		return nil, err
	}
	return badStripes, nil
}
