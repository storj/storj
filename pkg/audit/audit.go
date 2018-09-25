// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"io"

	"github.com/vivint/infectious"
	"google.golang.org/grpc"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/provider"
)

func setupPSClient(ctx context.Context) (psClient client.PSClient, err error) {
	// TODO(nat): get this info from the satellite somehow
	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	identOpt, err := identity.DialOption()
	if err != nil {
		return nil, err
	}

	var conn *grpc.ClientConn
	conn, err = grpc.Dial(":7777", identOpt)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	psClient, err = client.NewPSClient(conn, 1024*32, identity.Key.(*ecdsa.PrivateKey))
	if err != nil {
		return nil, err
	}
	return psClient, nil
}

func getShare(ctx context.Context, stripeIndex, required, shareSize int,
	id client.PieceID, segSize int64) (share infectious.Share, err error) {
	psClient, err := setupPSClient(ctx)
	if err != nil {
		return infectious.Share{}, err
	}

	rr, err := psClient.Get(ctx, id, segSize, &pb.PayerBandwidthAllocation{})
	if err != nil {
		return infectious.Share{}, err
	}

	offset := shareSize*stripeIndex - 1

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
		Number: stripeIndex,
		Data:   buf,
	}
	return share, nil
}

func audit(ctx context.Context, required, total int, originals []infectious.Share) (badStripes []int, err error) {
	f, err := infectious.NewFEC(required, total)
	if err != nil {
		return nil, err
	}
	shares := make([]infectious.Share, len(originals))
	for _, share := range shares {
		originals[share.Number] = share.DeepCopy()
	}
	err = f.Correct(shares)
	if err != nil {
		return nil, err
	}

	for _, share := range shares {
		if !bytes.Equal(originals[share.Number].Data, share.Data) {
			badStripes = append(badStripes, share.Number)
		}
	}
	return badStripes, nil
}

// runAudit takes a slice of pointers and runs an audit on shares at a given stripe index
// TODO(nat): code for randomizing the stripe index maybe can be inserted?
func runAudit(ctx context.Context, pointers []*pb.Pointer, stripeIndex, required, total int) (badStripes []int, err error) {
	var shares []infectious.Share
	for _, p := range pointers {
		ess := int(p.Remote.Redundancy.GetErasureShareSize())
		pieceID := client.PieceID(p.Remote.GetPieceId())
		// TODO(nat): this should probably be concurrent?
		share, err := getShare(ctx, stripeIndex, required, ess, pieceID, p.GetSize())
		if err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	badStripes, err = audit(ctx, required, total, shares)
	if err != nil {
		return nil, err
	}
	return badStripes, nil
}
