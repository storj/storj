// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
)

func EncodeOrderLimit(limit *pb.OrderLimit2) ([]byte, error) {
	signature := limit.SatelliteSignature
	defer func() { limit.SatelliteSignature = signature }()
	return proto.Marshal(limit)
}

func EncodeOrder(order *pb.Order2) ([]byte, error) {
	signature := order.UplinkSignature
	defer func() { order.UplinkSignature = signature }()
	return proto.Marshal(order)
}

func EncodePieceHash(hash *pb.PieceHash) ([]byte, error) {
	signature := hash.Signature
	defer func() { hash.Signature = signature }()
	return proto.Marshal(hash)
}
