// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Signee is able to verify that the data signature belongs to the signee.
type Signee interface {
	ID() storj.NodeID
	HashAndVerifySignature(ctx context.Context, data, signature []byte) error
}

// VerifyOrderLimitSignature verifies that the signature inside order limit belongs to the satellite.
func VerifyOrderLimitSignature(ctx context.Context, satellite Signee, signed *pb.OrderLimit) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrderLimit(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.HashAndVerifySignature(ctx, bytes, signed.SatelliteSignature)
}

// VerifyOrderSignature verifies that the signature inside order belongs to the uplink.
func VerifyOrderSignature(ctx context.Context, uplink Signee, signed *pb.Order) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeOrder(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return uplink.HashAndVerifySignature(ctx, bytes, signed.UplinkSignature)
}

// VerifyPieceHashSignature verifies that the signature inside piece hash belongs to the signer, which is either uplink or storage node.
func VerifyPieceHashSignature(ctx context.Context, signee Signee, signed *pb.PieceHash) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodePieceHash(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return signee.HashAndVerifySignature(ctx, bytes, signed.Signature)
}

// VerifyVoucher verifies that the signature inside voucher belongs to the satellite
func VerifyVoucher(ctx context.Context, satellite Signee, signed *pb.Voucher) (err error) {
	defer mon.Task()(&ctx)(&err)
	bytes, err := EncodeVoucher(ctx, signed)
	if err != nil {
		return Error.Wrap(err)
	}

	return satellite.HashAndVerifySignature(ctx, bytes, signed.SatelliteSignature)
}
