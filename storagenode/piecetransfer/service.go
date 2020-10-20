// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecetransfer

import (
	"bytes"
	"context"
	"os"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/trust"
	"storj.io/uplink/private/ecclient"
)

var (
	// Error is the default error class for graceful exit package.
	Error = errs.Class("internode transfer")

	mon = monkit.Package()
)

// Service allows for transfer of pieces from one storage node to
// another, as directed by the satellite that owns the piece.
type Service interface {
	// TransferPiece validates a transfer order, validates the locally stored
	// piece, and then (if appropriate) transfers the piece to the specified
	// destination node, obtaining a signed receipt. TransferPiece returns a
	// message appropriate for responding to the transfer order (whether the
	// transfer succeeded or failed).
	TransferPiece(ctx context.Context, satelliteID storj.NodeID, transferPiece *pb.TransferPiece) *pb.StorageNodeMessage
}

type service struct {
	log      *zap.Logger
	store    *pieces.Store
	trust    *trust.Pool
	ecClient ecclient.Client

	minDownloadTimeout time.Duration
	minBytesPerSecond  memory.Size
}

// NewService is a constructor for Service.
func NewService(log *zap.Logger, store *pieces.Store, trust *trust.Pool, dialer rpc.Dialer, minDownloadTimeout time.Duration, minBytesPerSecond memory.Size) Service {
	ecClient := ecclient.NewClient(log, dialer, 0)
	return &service{
		log:                log,
		store:              store,
		trust:              trust,
		ecClient:           ecClient,
		minDownloadTimeout: minDownloadTimeout,
		minBytesPerSecond:  minBytesPerSecond,
	}
}

// TransferPiece validates a transfer order, validates the locally stored
// piece, and then (if appropriate) transfers the piece to the specified
// destination node, obtaining a signed receipt. TransferPiece returns a
// message appropriate for responding to the transfer order (whether the
// transfer succeeded or failed).
func (c *service) TransferPiece(ctx context.Context, satelliteID storj.NodeID, transferPiece *pb.TransferPiece) *pb.StorageNodeMessage {
	// errForMonkit doesn't get returned, but we'd still like for monkit to be able
	// to differentiate between counts of failures returned and successes returned.
	var errForMonkit error
	defer mon.Task()(&ctx)(&errForMonkit)

	pieceID := transferPiece.OriginalPieceId
	logger := c.log.With(zap.Stringer("Satellite ID", satelliteID), zap.Stringer("Piece ID", pieceID))

	failMessage := func(errString string, err error, transferErr pb.TransferFailed_Error) *pb.StorageNodeMessage {
		logger.Error(errString, zap.Error(err))
		errForMonkit = err
		return &pb.StorageNodeMessage{
			Message: &pb.StorageNodeMessage_Failed{
				Failed: &pb.TransferFailed{
					OriginalPieceId: pieceID,
					Error:           transferErr,
				},
			},
		}
	}

	reader, err := c.store.Reader(ctx, satelliteID, pieceID)
	if err != nil {
		transferErr := pb.TransferFailed_UNKNOWN
		if errs.Is(err, os.ErrNotExist) {
			transferErr = pb.TransferFailed_NOT_FOUND
		}
		return failMessage("failed to get piece reader", err, transferErr)
	}

	addrLimit := transferPiece.GetAddressedOrderLimit()
	pk := transferPiece.PrivateKey

	originalHash, originalOrderLimit, err := c.store.GetHashAndLimit(ctx, satelliteID, pieceID, reader)
	if err != nil {
		return failMessage("failed to get piece hash and order limit.", err, pb.TransferFailed_UNKNOWN)
	}

	satelliteSigner, err := c.trust.GetSignee(ctx, satelliteID)
	if err != nil {
		return failMessage("failed to get satellite signer identity from trust store!", err, pb.TransferFailed_UNKNOWN)
	}

	// verify the satellite signature on the original order limit; if we hand in something
	// with an invalid signature, the satellite will assume we're cheating and disqualify
	// immediately.
	err = signing.VerifyOrderLimitSignature(ctx, satelliteSigner, &originalOrderLimit)
	if err != nil {
		msg := "The order limit stored for this piece does not have a valid signature from the owning satellite! It was verified before storing, so something went wrong in storage. We have to report this to the satellite as a missing piece."
		return failMessage(msg, err, pb.TransferFailed_NOT_FOUND)
	}

	// verify that the public key on the order limit signed the original piece hash; if we
	// hand in something with an invalid signature, the satellite will assume we're cheating
	// and disqualify immediately.
	err = signing.VerifyUplinkPieceHashSignature(ctx, originalOrderLimit.UplinkPublicKey, &originalHash)
	if err != nil {
		msg := "The piece hash stored for this piece does not have a valid signature from the public key stored in the order limit! It was verified before storing, so something went wrong in storage. We have to report this to the satellite as a missing piece."
		return failMessage(msg, err, pb.TransferFailed_NOT_FOUND)
	}

	// after this point, the destination storage node ID is relevant
	logger = logger.With(zap.Stringer("Storagenode ID", addrLimit.Limit.StorageNodeId))

	if c.minBytesPerSecond == 0 {
		// set minBytesPerSecond to default 5KiB if set to 0
		c.minBytesPerSecond = 5 * memory.KiB
	}
	maxTransferTime := time.Duration(int64(time.Second) * originalHash.PieceSize / c.minBytesPerSecond.Int64())
	if maxTransferTime < c.minDownloadTimeout {
		maxTransferTime = c.minDownloadTimeout
	}
	putCtx, cancel := context.WithTimeout(ctx, maxTransferTime)
	defer cancel()

	pieceHash, peerID, err := c.ecClient.PutPiece(putCtx, ctx, addrLimit, pk, reader)
	if err != nil {
		if piecestore.ErrVerifyUntrusted.Has(err) {
			return failMessage("failed hash verification", err, pb.TransferFailed_HASH_VERIFICATION)
		}
		// TODO look at error type to decide on the transfer error
		return failMessage("failed to put piece", err, pb.TransferFailed_STORAGE_NODE_UNAVAILABLE)
	}

	if !bytes.Equal(originalHash.Hash, pieceHash.Hash) {
		msg := "piece hash from new storagenode does not match"
		return failMessage(msg, Error.New(msg), pb.TransferFailed_HASH_VERIFICATION)
	}
	if pieceHash.PieceId != addrLimit.Limit.PieceId {
		msg := "piece id from new storagenode does not match order limit"
		return failMessage(msg, Error.New(msg), pb.TransferFailed_HASH_VERIFICATION)
	}

	signee := signing.SigneeFromPeerIdentity(peerID)
	err = signing.VerifyPieceHashSignature(ctx, signee, pieceHash)
	if err != nil {
		return failMessage("invalid piece hash signature from new storagenode", err, pb.TransferFailed_HASH_VERIFICATION)
	}

	success := &pb.StorageNodeMessage{
		Message: &pb.StorageNodeMessage_Succeeded{
			Succeeded: &pb.TransferSucceeded{
				OriginalPieceId:      transferPiece.OriginalPieceId,
				OriginalPieceHash:    &originalHash,
				OriginalOrderLimit:   &originalOrderLimit,
				ReplacementPieceHash: pieceHash,
			},
		},
	}
	logger.Info("piece transferred to new storagenode")
	return success
}
