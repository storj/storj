// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// Error is the default error class for signing package.
var Error = errs.Class("signing")

// Signer is able to sign data and verify own signature belongs.
type Signer interface {
	ID() storj.NodeID
	HashAndSign(data []byte) ([]byte, error)
	HashAndVerifySignature(data, signature []byte) error
}

// SignOrderLimit signs the order limit using the specified signer.
// Signer is a satellite.
func SignOrderLimit(satellite Signer, unsigned *pb.OrderLimit2) (*pb.OrderLimit2, error) {
	bytes, err := EncodeOrderLimit(unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.SatelliteSignature, err = satellite.HashAndSign(bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignOrder signs the order using the specified signer.
// Signer is an uplink.
func SignOrder(uplink Signer, unsigned *pb.Order2) (*pb.Order2, error) {
	bytes, err := EncodeOrder(unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.UplinkSignature, err = uplink.HashAndSign(bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}

// SignOrder signs the piece hash using the specified signer.
// Signer is either uplink or storage node.
func SignPieceHash(signer Signer, unsigned *pb.PieceHash) (*pb.PieceHash, error) {
	bytes, err := EncodePieceHash(unsigned)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	signed := *unsigned
	signed.Signature, err = signer.HashAndSign(bytes)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &signed, nil
}
