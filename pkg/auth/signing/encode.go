// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
)

// EncodeOrderLimit encodes order limit into bytes for signing. Removes signature from serialized limit.
func EncodeOrderLimit(ctx context.Context, limit *pb.OrderLimit) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := limit.SatelliteSignature
	limit.SatelliteSignature = nil
	out, err := proto.Marshal(limit)
	limit.SatelliteSignature = signature
	return out, err
}

// EncodeOrder encodes order into bytes for signing. Removes signature from serialized order.
func EncodeOrder(ctx context.Context, order *pb.Order) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := order.UplinkSignature
	order.UplinkSignature = nil
	out, err := proto.Marshal(order)
	order.UplinkSignature = signature
	return out, err
}

// EncodePieceHash encodes piece hash into bytes for signing. Removes signature from serialized hash.
func EncodePieceHash(ctx context.Context, hash *pb.PieceHash) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := hash.Signature
	hash.Signature = nil
	out, err := proto.Marshal(hash)
	hash.Signature = signature
	return out, err
}

// EncodeVoucher encodes voucher into bytes for signing.
func EncodeVoucher(ctx context.Context, voucher *pb.Voucher) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := voucher.SatelliteSignature
	voucher.SatelliteSignature = nil
	out, err := proto.Marshal(voucher)
	voucher.SatelliteSignature = signature
	return out, err
}
