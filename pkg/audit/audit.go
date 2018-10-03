// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"io"

	"github.com/vivint/infectious"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
)

var mon = monkit.Package()

// Auditor struct
type Auditor struct {
	t        transport.Client
	o        overlay.Client
	identity provider.FullIdentity
}

// NewAuditor creates a new instance of an Auditor struct
func NewAuditor(tc transport.Client, id provider.FullIdentity) *Auditor {
	return &Auditor{t: tc, identity: id}
}

func (a *Auditor) dial(ctx context.Context, node *pb.Node) (ps client.PSClient, err error) {
	defer mon.Task()(&ctx)(&err)
	c, err := a.t.DialNode(ctx, node)
	if err != nil {
		return nil, err
	}

	return client.NewPSClient(c, 0, a.identity.Key)
}

func (a *Auditor) getShare(ctx context.Context, stripeIndex, shareSize, pieceNumber int,
	id client.PieceID, pieceSize int64, node *pb.Node) (share infectious.Share, err error) {
	defer mon.Task()(&ctx)(&err)

	ps, err := a.dial(ctx, node)
	if err != nil {
		return infectious.Share{}, err
	}
	rr, err := ps.Get(ctx, id, pieceSize, &pb.PayerBandwidthAllocation{})
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
	if err != nil {
		return infectious.Share{}, err
	}

	share = infectious.Share{
		Number: pieceNumber,
		Data:   buf,
	}
	return share, nil
}

func (a *Auditor) audit(ctx context.Context, required, total int, originals []infectious.Share) (badStripes []int, err error) {
	defer mon.Task()(&ctx)(&err)
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

	for i, share := range copies {
		if !bytes.Equal(originals[i].Data, share.Data) {
			badStripes = append(badStripes, share.Number)
		}
	}
	return badStripes, nil
}

// lookupNodes calls BulkLookup to get node addresses from the overlay
func (a *Auditor) lookupNodes(ctx context.Context, pieces []*pb.RemotePiece) (nodes []*pb.Node, err error) {
	var nodeIds []dht.NodeID
	for _, p := range pieces {
		nodeIds = append(nodeIds, kademlia.StringToNodeID(p.GetNodeId()))
	}
	nodes, err = a.o.BulkLookup(ctx, nodeIds)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func calcPadded(size int64, blockSize int) int64 {
	mod := size % int64(blockSize)
	if mod == 0 {
		return size
	}
	return size + int64(blockSize) - mod
}

// runAudit gets remote segments from a pointer and runs an audit on shares
// at a given stripe index
func (a *Auditor) runAudit(ctx context.Context, p *pb.Pointer, stripeIndex, required, total int) (badStripes []int, err error) {
	defer mon.Task()(&ctx)(&err)
	nodes, err := a.lookupNodes(ctx, p.Remote.GetRemotePieces())
	if err != nil {
		return nil, err
	}
	var shares []infectious.Share
	shareSize := int(p.Remote.Redundancy.GetErasureShareSize())
	pieceID := client.PieceID(p.Remote.GetPieceId())

	for i, rp := range p.Remote.RemotePieces {
		paddedSize := calcPadded(p.GetSize(), shareSize)
		pieceSize := paddedSize / int64(required)

		node := &pb.Node{
			Id:      rp.GetNodeId(),
			Address: nodes[i].GetAddress(),
		}
		share, err := a.getShare(ctx, stripeIndex, shareSize, i, pieceID, pieceSize, node)
		if err != nil {
			// TODO(nat): update the statdb here to indicate this node failed the audit
		}
		shares = append(shares, share)
	}

	badStripes, err = a.audit(ctx, required, total, shares)
	if err != nil {
		return nil, err
	}
	return badStripes, nil
}
