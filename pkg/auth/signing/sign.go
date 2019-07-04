// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Error is the default error class for signing package.
var Error = errs.Class("signing")

// Signer is able to sign data and verify own signature belongs.
type Signer interface {
	ID() storj.NodeID
	HashAndSign(ctx context.Context, data []byte) ([]byte, error)
	HashAndVerifySignature(ctx context.Context, data, signature []byte) error
}

// SignOrderLimit signs the order limit using the specified signer.
// Signer is a satellite.
func SignOrderLimit(ctx context.Context, satellite Signer, unsigned *pb.OrderLimit) (_ *pb.OrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrderLimit(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = satellite.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignOrder signs the order using the specified signer.
// Signer is an uplink.
func SignOrder(ctx context.Context, uplink Signer, unsigned *pb.Order) (_ *pb.Order, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrder(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.UplinkSignature, err = uplink.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignPieceHash signs the piece hash using the specified signer.
// Signer is either uplink or storage node.
func SignPieceHash(ctx context.Context, signer Signer, unsigned *pb.PieceHash) (_ *pb.PieceHash, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodePieceHash(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.Signature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignVoucher signs the voucher using the specified signer
// Signer is a satellite
func SignVoucher(ctx context.Context, signer Signer, unsigned *pb.Voucher) (_ *pb.Voucher, err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeVoucher(ctx, unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = signer.HashAndSign(ctx, bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}
