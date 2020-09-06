// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"bytes"
	"context"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
)

func (endpoint *Endpoint) validatePendingTransfer(ctx context.Context, transfer *PendingTransfer) error {
	if transfer.SatelliteMessage == nil {
		return Error.New("Satellite message cannot be nil")
	}
	if transfer.SatelliteMessage.GetTransferPiece() == nil {
		return Error.New("Satellite message transfer piece cannot be nil")
	}
	if transfer.SatelliteMessage.GetTransferPiece().GetAddressedOrderLimit() == nil {
		return Error.New("Addressed order limit on transfer piece cannot be nil")
	}
	if transfer.SatelliteMessage.GetTransferPiece().GetAddressedOrderLimit().GetLimit() == nil {
		return Error.New("Addressed order limit on transfer piece cannot be nil")
	}
	if transfer.Key == nil {
		return Error.New("Transfer key cannot be nil")
	}
	if transfer.OriginalPointer == nil || transfer.OriginalPointer.GetRemote() == nil {
		return Error.New("could not get remote pointer from transfer item")
	}

	return nil
}

func (endpoint *Endpoint) verifyPieceTransferred(ctx context.Context, message *pb.StorageNodeMessage_Succeeded, transfer *PendingTransfer, receivingNodePeerID *identity.PeerIdentity) error {
	originalOrderLimit := message.Succeeded.GetOriginalOrderLimit()
	if originalOrderLimit == nil {
		return ErrInvalidArgument.New("Original order limit cannot be nil")
	}
	originalPieceHash := message.Succeeded.GetOriginalPieceHash()
	if originalPieceHash == nil {
		return ErrInvalidArgument.New("Original piece hash cannot be nil")
	}
	replacementPieceHash := message.Succeeded.GetReplacementPieceHash()
	if replacementPieceHash == nil {
		return ErrInvalidArgument.New("Replacement piece hash cannot be nil")
	}

	// verify that the original piece hash and replacement piece hash match
	if !bytes.Equal(originalPieceHash.Hash, replacementPieceHash.Hash) {
		return ErrInvalidArgument.New("Piece hashes for transferred piece don't match")
	}

	// verify that the satellite signed the original order limit
	err := signing.VerifyOrderLimitSignature(ctx, endpoint.signer, originalOrderLimit)
	if err != nil {
		return ErrInvalidArgument.New("Could not validate signature from satellite on claimed original order limit: %v", err)
	}

	// verify that the public key on the order limit signed the original piece hash
	err = signing.VerifyUplinkPieceHashSignature(ctx, originalOrderLimit.UplinkPublicKey, originalPieceHash)
	if err != nil {
		return ErrInvalidArgument.New("Could not validate signature from original order limit's public key on original piece hash: %v", err)
	}

	if originalOrderLimit.PieceId != message.Succeeded.OriginalPieceId {
		return ErrInvalidArgument.New("Original piece ID in piece-transfer success message does not match piece ID in original order limit")
	}

	receivingNodeID := transfer.SatelliteMessage.GetTransferPiece().GetAddressedOrderLimit().GetLimit().StorageNodeId
	calculatedNewPieceID := transfer.OriginalPointer.GetRemote().RootPieceId.Derive(receivingNodeID, transfer.PieceNum)
	if calculatedNewPieceID != replacementPieceHash.PieceId {
		return ErrInvalidArgument.New("Replacement piece ID does not match derived piece ID for receivingNodeID %s, PieceNum %d: %q != %q", receivingNodeID.String(), transfer.PieceNum, calculatedNewPieceID.String(), replacementPieceHash.PieceId.String())
	}

	signer := signing.SigneeFromPeerIdentity(receivingNodePeerID)

	// verify that the new node signed the replacement piece hash
	err = signing.VerifyPieceHashSignature(ctx, signer, replacementPieceHash)
	if err != nil {
		return ErrInvalidArgument.New("Could not validate signature from receiving node on replacement piece hash: %v", err)
	}
	return nil
}
