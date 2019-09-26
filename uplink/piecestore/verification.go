// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"bytes"
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/signing"
)

var (
	// ErrInternal is an error class for internal errors.
	ErrInternal = errs.Class("internal")
	// ErrProtocol is an error class for unexpected protocol sequence.
	ErrProtocol = errs.Class("protocol")
	// ErrVerifyUntrusted is an error in case there is a trust issue.
	ErrVerifyUntrusted = errs.Class("untrusted")
)

// VerifyPieceHash verifies piece hash which is sent by peer.
func (client *Client) VerifyPieceHash(ctx context.Context, peer *identity.PeerIdentity, limit *pb.OrderLimit, hash *pb.PieceHash, expectedHash []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	if peer == nil || limit == nil || hash == nil || len(expectedHash) == 0 {
		return ErrProtocol.New("invalid arguments")
	}
	if limit.PieceId != hash.PieceId {
		return ErrProtocol.New("piece id changed") // TODO: report rpc status bad message
	}
	if !bytes.Equal(hash.Hash, expectedHash) {
		return ErrVerifyUntrusted.New("hashes don't match") // TODO: report rpc status bad message
	}

	if err := signing.VerifyPieceHashSignature(ctx, signing.SigneeFromPeerIdentity(peer), hash); err != nil {
		return ErrVerifyUntrusted.New("invalid hash signature: %v", err) // TODO: report rpc status bad message
	}

	return nil
}
