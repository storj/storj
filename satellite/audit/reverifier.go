// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/uplink/private/piecestore"
)

// PieceLocator specifies all information necessary to look up a particular piece
// on a particular satellite.
type PieceLocator struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition
	NodeID   storj.NodeID
	PieceNum int
}

// ReverificationJob represents a job as received from the reverification
// audit queue.
type ReverificationJob struct {
	Locator       PieceLocator
	InsertedAt    time.Time
	ReverifyCount int
	LastAttempt   *time.Time
}

// Reverifier pulls jobs from the reverification queue and fulfills them
// by performing the requested reverifications.
//
// architecture: Worker
type Reverifier struct {
	*Verifier

	log *zap.Logger
	db  ReverifyQueue

	// retryInterval defines a limit on how frequently we will retry
	// reverification audits. At least this long should elapse between
	// attempts.
	retryInterval time.Duration
}

// Outcome enumerates the possible results of a piecewise audit.
//
// Note that it is very similar to reputation.AuditType, but it is
// different in scope and needs a slightly different set of values.
type Outcome int

const (
	// OutcomeNotPerformed indicates an audit was not performed, for any of a
	// variety of reasons, but that it should be reattempted later.
	OutcomeNotPerformed Outcome = iota
	// OutcomeNotNecessary indicates that an audit is no longer required,
	// for example because the segment has been updated or no longer exists.
	OutcomeNotNecessary
	// OutcomeSuccess indicates that an audit took place and the piece was
	// fully validated.
	OutcomeSuccess
	// OutcomeFailure indicates that an audit took place but that the node
	// failed the audit, either because it did not have the piece or the
	// data was incorrect.
	OutcomeFailure
	// OutcomeTimedOut indicates the audit could not be completed because
	// it took too long. The audit should be retried later.
	OutcomeTimedOut
	// OutcomeNodeOffline indicates that the audit could not be completed
	// because the node could not be contacted. The audit should be
	// retried later.
	OutcomeNodeOffline
	// OutcomeUnknownError indicates that the audit could not be completed
	// because of an error not otherwise expected or recognized. The
	// audit should be retried later.
	OutcomeUnknownError
)

// NewReverifier creates a Reverifier.
func NewReverifier(log *zap.Logger, verifier *Verifier, db ReverifyQueue, config Config) *Reverifier {
	return &Reverifier{
		log:           log,
		Verifier:      verifier,
		db:            db,
		retryInterval: config.ReverificationRetryInterval,
	}
}

// ReverifyPiece acquires a piece from a single node and verifies its
// contents, its hash, and its order limit.
func (reverifier *Reverifier) ReverifyPiece(ctx context.Context, logger *zap.Logger, locator *PieceLocator) (outcome Outcome, reputation overlay.ReputationStatus) {
	defer mon.Task()(&ctx)(nil)

	outcome, reputation, err := reverifier.DoReverifyPiece(ctx, logger, locator)
	if err != nil {
		logger.Error("could not perform reverification due to error", zap.Error(err))
		return outcome, reputation
	}

	var (
		successes int
		offlines  int
		fails     int
		pending   int
		unknown   int
	)
	switch outcome {
	case OutcomeNotPerformed, OutcomeNotNecessary:
	case OutcomeSuccess:
		successes++
	case OutcomeFailure:
		fails++
	case OutcomeTimedOut:
		pending++
	case OutcomeNodeOffline:
		offlines++
	case OutcomeUnknownError:
		unknown++
	}
	mon.Meter("reverify_successes_global").Mark(successes)
	mon.Meter("reverify_offlines_global").Mark(offlines)
	mon.Meter("reverify_fails_global").Mark(fails)
	mon.Meter("reverify_contained_global").Mark(pending)
	mon.Meter("reverify_unknown_global").Mark(unknown)

	return outcome, reputation
}

// DoReverifyPiece acquires a piece from a single node and verifies its
// contents, its hash, and its order limit.
func (reverifier *Reverifier) DoReverifyPiece(ctx context.Context, logger *zap.Logger, locator *PieceLocator) (outcome Outcome, reputation overlay.ReputationStatus, err error) {
	defer mon.Task()(&ctx)(&err)

	// First, we must ensure that the specified node still holds the indicated piece.
	segment, err := reverifier.metabase.GetSegmentByPositionForAudit(ctx, metabase.GetSegmentByPosition{
		StreamID: locator.StreamID,
		Position: locator.Position,
	})
	if err != nil {
		if metabase.ErrSegmentNotFound.Has(err) {
			logger.Debug("segment no longer exists")
			return OutcomeNotNecessary, reputation, nil
		}
		return OutcomeNotPerformed, reputation, Error.Wrap(err)
	}
	if segment.Expired(reverifier.nowFn()) {
		logger.Debug("segment expired before ReverifyPiece")
		return OutcomeNotNecessary, reputation, nil
	}
	piece, found := segment.Pieces.FindByNum(locator.PieceNum)
	if !found || piece.StorageNode != locator.NodeID {
		logger.Debug("piece is no longer held by the indicated node")
		return OutcomeNotNecessary, reputation, nil
	}

	// TODO remove this when old entries with empty StreamID will be deleted
	if locator.StreamID.IsZero() {
		logger.Debug("ReverifyPiece: skip pending audit with empty StreamID")
		return OutcomeNotNecessary, reputation, nil
	}

	pieceSize := segment.PieceSize()

	limit, piecePrivateKey, cachedNodeInfo, err := reverifier.orders.CreateAuditPieceOrderLimit(ctx, locator.NodeID, uint16(locator.PieceNum), segment.RootPieceID, int32(pieceSize))
	if err != nil {
		if overlay.ErrNodeDisqualified.Has(err) {
			logger.Debug("ReverifyPiece: order limit not created (node is already disqualified)")
			return OutcomeNotNecessary, reputation, nil
		}
		if overlay.ErrNodeFinishedGE.Has(err) {
			logger.Debug("ReverifyPiece: order limit not created (node has completed graceful exit)")
			return OutcomeNotNecessary, reputation, nil
		}
		if overlay.ErrNodeOffline.Has(err) {
			logger.Debug("ReverifyPiece: order limit not created (node considered offline)")
			return OutcomeNodeOffline, reputation, nil
		}
		return OutcomeNotPerformed, reputation, Error.Wrap(err)
	}

	reputation = cachedNodeInfo.Reputation
	pieceData, pieceHash, pieceOriginalLimit, err := reverifier.GetPiece(ctx, limit, piecePrivateKey, cachedNodeInfo.LastIPPort, int32(pieceSize))
	if err != nil {
		if rpc.Error.Has(err) {
			if errs.Is(err, context.DeadlineExceeded) {
				// dial timeout
				return OutcomeTimedOut, reputation, nil
			}
			if errs2.IsRPC(err, rpcstatus.Unknown) {
				// dial failed -- offline node
				return OutcomeNodeOffline, reputation, nil
			}
			// unknown transport error
			logger.Info("ReverifyPiece: unknown transport error", zap.Error(err))
			return OutcomeUnknownError, reputation, nil
		}
		if errs2.IsRPC(err, rpcstatus.NotFound) {
			// Fetch the segment metadata again and see if it has been altered in the interim
			err := reverifier.checkIfSegmentAltered(ctx, segment)
			if err != nil {
				// if so, we skip this audit
				logger.Debug("ReverifyPiece: audit source segment changed during reverification", zap.Error(err))
				return OutcomeNotNecessary, reputation, nil
			}
			// missing share
			logger.Info("ReverifyPiece: audit failure; node indicates piece not found")
			return OutcomeFailure, reputation, nil
		}
		if errs2.IsRPC(err, rpcstatus.DeadlineExceeded) {
			// dial successful, but download timed out
			return OutcomeTimedOut, reputation, nil
		}
		// unknown error
		logger.Info("ReverifyPiece: unknown error from node", zap.Error(err))
		return OutcomeUnknownError, reputation, nil
	}

	// We have successfully acquired the piece from the node. Now, we must verify its contents.

	if pieceHash == nil {
		logger.Info("ReverifyPiece: audit failure; node did not send piece hash as requested")
		return OutcomeFailure, reputation, nil
	}
	if pieceOriginalLimit == nil {
		logger.Info("ReverifyPiece: audit failure; node did not send original order limit as requested")
		return OutcomeFailure, reputation, nil
	}
	// check for the correct size
	if int64(len(pieceData)) != pieceSize {
		logger.Info("ReverifyPiece: audit failure; downloaded piece has incorrect size", zap.Int64("expected-size", pieceSize), zap.Int("received-size", len(pieceData)))
		outcome = OutcomeFailure
		// continue to run, so we can check if the piece was legitimately changed before
		// blaming the node
	} else {
		// check for a matching hash
		downloadedHash := hashWithAlgo(pieceHash.HashAlgorithm, pieceData)
		if !bytes.Equal(downloadedHash, pieceHash.Hash) {
			logger.Info("ReverifyPiece: audit failure; downloaded piece does not match hash", zap.ByteString("downloaded", downloadedHash), zap.ByteString("expected", pieceHash.Hash))
			outcome = OutcomeFailure
			// continue to run, so we can check if the piece was legitimately changed
			// before blaming the node
		} else {
			// check that the order limit and hash sent by the storagenode were
			// correctly signed (order limit signed by this satellite, hash signed
			// by the uplink public key in the order limit)
			signer := signing.SigneeFromPeerIdentity(reverifier.auditor)
			if err := signing.VerifyOrderLimitSignature(ctx, signer, pieceOriginalLimit); err != nil {
				return OutcomeFailure, reputation, nil
			}
			if err := signing.VerifyUplinkPieceHashSignature(ctx, pieceOriginalLimit.UplinkPublicKey, pieceHash); err != nil {
				return OutcomeFailure, reputation, nil
			}
		}
	}

	if err := reverifier.checkIfSegmentAltered(ctx, segment); err != nil {
		logger.Debug("ReverifyPiece: audit source segment changed during reverification", zap.Error(err))
		return OutcomeNotNecessary, reputation, nil
	}
	if outcome == OutcomeFailure {
		return OutcomeFailure, reputation, nil
	}

	return OutcomeSuccess, reputation, nil
}

func hashWithAlgo(algo pb.PieceHashAlgorithm, data []byte) []byte {
	h := pb.NewHashFromAlgorithm(algo)
	_, err := h.Write(data)
	if err != nil {
		// sha256 and blake3 hash writers never return errors. we could just ignore
		// the error return value (many callers do), but that seems unwise.
		panic(err)
	}
	return h.Sum(nil)
}

// GetPiece uses the piecestore client to download a piece (and the associated
// original OrderLimit and PieceHash) from a node.
func (reverifier *Reverifier) GetPiece(ctx context.Context, limit *pb.AddressedOrderLimit, piecePrivateKey storj.PiecePrivateKey, cachedIPAndPort string, pieceSize int32) (pieceData []byte, hash *pb.PieceHash, origLimit *pb.OrderLimit, err error) {
	defer mon.Task()(&ctx)(&err)

	// determines number of seconds allotted for receiving data from a storage node
	timedCtx := ctx
	if reverifier.minBytesPerSecond > 0 {
		maxTransferTime := time.Duration(int64(time.Second) * int64(pieceSize) / reverifier.minBytesPerSecond.Int64())
		if maxTransferTime < reverifier.minDownloadTimeout {
			maxTransferTime = reverifier.minDownloadTimeout
		}
		var cancel func()
		timedCtx, cancel = context.WithTimeout(ctx, maxTransferTime)
		defer cancel()
	}

	targetNodeID := limit.GetLimit().StorageNodeId
	log := reverifier.log.With(zap.Stringer("node-id", targetNodeID), zap.Stringer("piece-id", limit.GetLimit().PieceId))
	var ps *piecestore.Client

	// if cached IP is given, try connecting there first
	if cachedIPAndPort != "" {
		nodeAddr := storj.NodeURL{
			ID:      targetNodeID,
			Address: cachedIPAndPort,
		}
		ps, err = piecestore.Dial(timedCtx, reverifier.dialer, nodeAddr, piecestore.DefaultConfig)
		if err != nil {
			log.Debug("failed to connect to audit target node at cached IP", zap.String("cached-ip-and-port", cachedIPAndPort), zap.Error(err))
		}
	}

	// if no cached IP was given, or connecting to cached IP failed, use node address
	if ps == nil {
		nodeAddr := storj.NodeURL{
			ID:      targetNodeID,
			Address: limit.GetStorageNodeAddress().Address,
		}
		ps, err = piecestore.Dial(timedCtx, reverifier.dialer, nodeAddr, piecestore.DefaultConfig)
		if err != nil {
			return nil, nil, nil, Error.Wrap(err)
		}
	}

	defer func() {
		err := ps.Close()
		if err != nil {
			log.Error("audit reverifier failed to close conn to node", zap.Error(err))
		}
	}()

	downloader, err := ps.Download(timedCtx, limit.GetLimit(), piecePrivateKey, 0, int64(pieceSize))
	if err != nil {
		return nil, nil, nil, Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, Error.Wrap(downloader.Close())) }()

	buf := make([]byte, pieceSize)
	_, err = io.ReadFull(downloader, buf)
	if err != nil {
		return nil, nil, nil, Error.Wrap(err)
	}
	hash, originLimit := downloader.GetHashAndLimit()

	return buf, hash, originLimit, nil
}
