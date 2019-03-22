// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
)

// EncodeOrderLimit encodes order limit into bytes for signing.
func EncodeOrderLimit(limit *pb.OrderLimit2) ([]byte, error) {
	signature := limit.SatelliteSignature
	limit.SatelliteSignature = nil
	defer func() { limit.SatelliteSignature = signature }()
	return proto.Marshal(limit)
}

// EncodeOrder encodes order into bytes for signing.
func EncodeOrder(order *pb.Order2) ([]byte, error) {
	signature := order.UplinkSignature
	order.UplinkSignature = nil
	defer func() { order.UplinkSignature = signature }()
	return proto.Marshal(order)
}

// EncodePieceHash encodes piece hash into bytes for signing.
func EncodePieceHash(hash *pb.PieceHash) ([]byte, error) {
	signature := hash.Signature
	hash.Signature = nil
	defer func() { hash.Signature = signature }()
	return proto.Marshal(hash)
}
