// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
)

// EncodeOrderLimit encodes order limit into bytes for signing.
func EncodeOrderLimit(ctx context.Context, limit *pb.OrderLimit2) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := limit.SatelliteSignature
	limit.SatelliteSignature = nil
	defer func() { limit.SatelliteSignature = signature }()
	return proto.Marshal(limit)
}

// EncodeOrder encodes order into bytes for signing.
func EncodeOrder(ctx context.Context, order *pb.Order2) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := order.UplinkSignature
	order.UplinkSignature = nil
	defer func() { order.UplinkSignature = signature }()
	return proto.Marshal(order)
}

// EncodePieceHash encodes piece hash into bytes for signing.
func EncodePieceHash(ctx context.Context, hash *pb.PieceHash) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := hash.Signature
	hash.Signature = nil
	defer func() { hash.Signature = signature }()
	return proto.Marshal(hash)
}
