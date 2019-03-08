package piecestore

import (
	"bytes"
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

var (
	ErrInternal               = errs.Class("internal")
	ErrProtocol               = errs.Class("protocol")
	ErrVerifyNotAuthorized    = errs.Class("not authorized")
	ErrVerifyUntrusted        = errs.Class("untrusted")
	ErrVerifyDuplicateRequest = errs.Class("duplicate request")
)

func (client *Client) VerifyPieceHash(ctx context.Context, peer *identity.PeerIdentity, limit *pb.OrderLimit2, hash *pb.PieceHash, expectedHash []byte) error {
	if peer == nil || limit == nil || hash == nil || len(expectedHash) == 0 {
		return ErrProtocol.New("invalid arguments")
	}
	if limit.PieceId != hash.PieceId {
		return ErrProtocol.New("piece id changed") // TODO: report grpc status bad message
	}
	if !bytes.Equal(hash.Hash, expectedHash) {
		return ErrProtocol.New("hashes don't match") // TODO: report grpc status bad message
	}

	if err := signing.VerifyPieceHashSignature(signing.SigneeFromPeerIdentity(peer), hash); err != nil {
		return ErrVerifyUntrusted.New("invalid hash signature: %v", err) // TODO: report grpc status bad message
	}

	return nil
}
