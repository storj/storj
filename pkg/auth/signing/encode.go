// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
)

// EncodeOrderLimit encodes order limit into bytes for signing. Removes signature from serialized limit.
func EncodeOrderLimit(limit *pb.OrderLimit2) ([]byte, error) {
	signature := limit.SatelliteSignature
	limit.SatelliteSignature = nil
	out, err := proto.Marshal(limit)
	limit.SatelliteSignature = signature
	return out, err
}

// EncodeOrder encodes order into bytes for signing. Removes signature from serialized order.
func EncodeOrder(order *pb.Order2) ([]byte, error) {
	signature := order.UplinkSignature
	order.UplinkSignature = nil
	out, err := proto.Marshal(order)
	order.UplinkSignature = signature
	return out, err
}

// EncodePieceHash encodes piece hash into bytes for signing. Removes signature from serialized hash.
func EncodePieceHash(hash *pb.PieceHash) ([]byte, error) {
	signature := hash.Signature
	hash.Signature = nil
	out, err := proto.Marshal(hash)
	hash.Signature = signature
	return out, err
}

// EncodeVoucher encodes voucher into bytes for signing.
func EncodeVoucher(voucher *pb.Voucher) ([]byte, error) {
	signature := voucher.SatelliteSignature
	voucher.SatelliteSignature = nil
	out, err := proto.Marshal(voucher)
	voucher.SatelliteSignature = signature
	return out, err
}
