// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/uplink/ecclient"
)

// Worker is responsible for completing the graceful exit for a given satellite.
type Worker struct {
	log                *zap.Logger
	store              *pieces.Store
	satelliteDB        satellites.DB
	dialer             rpc.Dialer
	limiter            *sync2.Limiter
	satelliteID        storj.NodeID
	satelliteAddr      string
	ecclient           ecclient.Client
	minBytesPerSecond  memory.Size
	minDownloadTimeout time.Duration
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, store *pieces.Store, satelliteDB satellites.DB, dialer rpc.Dialer, satelliteID storj.NodeID, satelliteAddr string, config Config) *Worker {
	return &Worker{
		log:                log,
		store:              store,
		satelliteDB:        satelliteDB,
		dialer:             dialer,
		limiter:            sync2.NewLimiter(config.NumConcurrentTransfers),
		satelliteID:        satelliteID,
		satelliteAddr:      satelliteAddr,
		ecclient:           ecclient.NewClient(log, dialer, 0),
		minBytesPerSecond:  config.MinBytesPerSecond,
		minDownloadTimeout: config.MinDownloadTimeout,
	}
}

// Run calls the satellite endpoint, transfers pieces, validates, and responds with success or failure.
// It also marks the satellite finished once all the pieces have been transferred
func (worker *Worker) Run(ctx context.Context, done func()) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer done()

	worker.log.Debug("running worker")

	conn, err := worker.dialer.DialAddressID(ctx, worker.satelliteAddr, worker.satelliteID)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	client := pb.NewDRPCSatelliteGracefulExitClient(conn.Raw())

	c, err := client.Process(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	for {
		response, err := c.Recv()
		if errs.Is(err, io.EOF) {
			// Done
			return nil
		}
		if err != nil {
			// TODO what happened
			return errs.Wrap(err)
		}

		switch msg := response.GetMessage().(type) {
		case *pb.SatelliteMessage_NotReady:
			return nil

		case *pb.SatelliteMessage_TransferPiece:
			transferPieceMsg := msg.TransferPiece
			worker.limiter.Go(ctx, func() {
				err = worker.transferPiece(ctx, transferPieceMsg, c)
				if err != nil {
					worker.log.Error("failed to transfer piece.",
						zap.Stringer("Satellite ID", worker.satelliteID),
						zap.Error(errs.Wrap(err)))
				}
			})

		case *pb.SatelliteMessage_DeletePiece:
			deletePieceMsg := msg.DeletePiece
			worker.limiter.Go(ctx, func() {
				pieceID := deletePieceMsg.OriginalPieceId
				err := worker.deleteOnePieceOrAll(ctx, &pieceID)
				if err != nil {
					worker.log.Error("failed to delete piece.",
						zap.Stringer("Satellite ID", worker.satelliteID),
						zap.Stringer("Piece ID", pieceID),
						zap.Error(errs.Wrap(err)))
				}
			})

		case *pb.SatelliteMessage_ExitFailed:
			worker.log.Error("graceful exit failed.",
				zap.Stringer("Satellite ID", worker.satelliteID),
				zap.Stringer("reason", msg.ExitFailed.Reason))

			exitFailedBytes, err := proto.Marshal(msg.ExitFailed)
			if err != nil {
				worker.log.Error("failed to marshal exit failed message.")
			}
			err = worker.satelliteDB.CompleteGracefulExit(ctx, worker.satelliteID, time.Now(), satellites.ExitFailed, exitFailedBytes)
			return errs.Wrap(err)

		case *pb.SatelliteMessage_ExitCompleted:
			worker.log.Info("graceful exit completed.", zap.Stringer("Satellite ID", worker.satelliteID))

			exitCompletedBytes, err := proto.Marshal(msg.ExitCompleted)
			if err != nil {
				worker.log.Error("failed to marshal exit completed message.")
			}

			err = worker.satelliteDB.CompleteGracefulExit(ctx, worker.satelliteID, time.Now(), satellites.ExitSucceeded, exitCompletedBytes)
			if err != nil {
				return errs.Wrap(err)
			}
			// delete all remaining pieces
			err = worker.deleteOnePieceOrAll(ctx, nil)
			return errs.Wrap(err)

		default:
			// TODO handle err
			worker.log.Error("unknown graceful exit message.", zap.Stringer("Satellite ID", worker.satelliteID))
		}
	}
}

type gracefulExitStream interface {
	Context() context.Context
	Send(*pb.StorageNodeMessage) error
	Recv() (*pb.SatelliteMessage, error)
}

func (worker *Worker) transferPiece(ctx context.Context, transferPiece *pb.TransferPiece, c gracefulExitStream) error {
	pieceID := transferPiece.OriginalPieceId
	reader, err := worker.store.Reader(ctx, worker.satelliteID, pieceID)
	if err != nil {
		transferErr := pb.TransferFailed_UNKNOWN
		if errs.Is(err, os.ErrNotExist) {
			transferErr = pb.TransferFailed_NOT_FOUND
		}
		worker.log.Error("failed to get piece reader.",
			zap.Stringer("Satellite ID", worker.satelliteID),
			zap.Stringer("Piece ID", pieceID),
			zap.Error(errs.Wrap(err)))
		worker.handleFailure(ctx, transferErr, pieceID, c.Send)
		return err
	}

	addrLimit := transferPiece.GetAddressedOrderLimit()
	pk := transferPiece.PrivateKey

	originalHash, originalOrderLimit, err := worker.store.GetHashAndLimit(ctx, worker.satelliteID, pieceID, reader)
	if err != nil {
		worker.log.Error("failed to get piece hash and order limit.",
			zap.Stringer("Satellite ID", worker.satelliteID),
			zap.Stringer("Piece ID", pieceID),
			zap.Error(errs.Wrap(err)))
		worker.handleFailure(ctx, pb.TransferFailed_UNKNOWN, pieceID, c.Send)
		return err
	}

	if worker.minBytesPerSecond == 0 {
		// set minBytesPerSecond to default 128B if set to 0
		worker.minBytesPerSecond = 128 * memory.B
	}
	maxTransferTime := time.Duration(int64(time.Second) * originalHash.PieceSize / worker.minBytesPerSecond.Int64())
	if maxTransferTime < worker.minDownloadTimeout {
		maxTransferTime = worker.minDownloadTimeout
	}
	putCtx, cancel := context.WithTimeout(ctx, maxTransferTime)
	defer cancel()

	pieceHash, peerID, err := worker.ecclient.PutPiece(putCtx, ctx, addrLimit, pk, reader)
	if err != nil {
		if piecestore.ErrVerifyUntrusted.Has(err) {
			worker.log.Error("failed hash verification.",
				zap.Stringer("Satellite ID", worker.satelliteID),
				zap.Stringer("Piece ID", pieceID),
				zap.Error(errs.Wrap(err)))
			worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		} else {
			worker.log.Error("failed to put piece.",
				zap.Stringer("Satellite ID", worker.satelliteID),
				zap.Stringer("Piece ID", pieceID),
				zap.Error(errs.Wrap(err)))
			// TODO look at error type to decide on the transfer error
			worker.handleFailure(ctx, pb.TransferFailed_STORAGE_NODE_UNAVAILABLE, pieceID, c.Send)
		}
		return err
	}

	if !bytes.Equal(originalHash.Hash, pieceHash.Hash) {
		worker.log.Error("piece hash from new storagenode does not match",
			zap.Stringer("Storagenode ID", addrLimit.Limit.StorageNodeId),
			zap.Stringer("Satellite ID", worker.satelliteID),
			zap.Stringer("Piece ID", pieceID))
		worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		return Error.New("piece hash from new storagenode does not match")
	}
	if pieceHash.PieceId != addrLimit.Limit.PieceId {
		worker.log.Error("piece id from new storagenode does not match order limit",
			zap.Stringer("Storagenode ID", addrLimit.Limit.StorageNodeId),
			zap.Stringer("Satellite ID", worker.satelliteID),
			zap.Stringer("Piece ID", pieceID))
		worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		return Error.New("piece id from new storagenode does not match order limit")
	}

	signee := signing.SigneeFromPeerIdentity(peerID)
	err = signing.VerifyPieceHashSignature(ctx, signee, pieceHash)
	if err != nil {
		worker.log.Error("invalid piece hash signature from new storagenode",
			zap.Stringer("Storagenode ID", addrLimit.Limit.StorageNodeId),
			zap.Stringer("Satellite ID", worker.satelliteID),
			zap.Stringer("Piece ID", pieceID),
			zap.Error(errs.Wrap(err)))
		worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		return err
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
	worker.log.Info("piece transferred to new storagenode",
		zap.Stringer("Storagenode ID", addrLimit.Limit.StorageNodeId),
		zap.Stringer("Satellite ID", worker.satelliteID),
		zap.Stringer("Piece ID", pieceID))
	return c.Send(success)
}

// deleteOnePieceOrAll deletes pieces stored for a satellite. When no piece ID are specified, all pieces stored by a satellite will be deleted.
func (worker *Worker) deleteOnePieceOrAll(ctx context.Context, pieceID *storj.PieceID) error {
	// get piece size
	pieceMap := make(map[pb.PieceID]int64)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	err := worker.store.WalkSatellitePieces(ctxWithCancel, worker.satelliteID, func(piece pieces.StoredPieceAccess) error {
		_, size, err := piece.Size(ctxWithCancel)
		if err != nil {
			worker.log.Debug("failed to retrieve piece info", zap.Stringer("Satellite ID", worker.satelliteID), zap.Error(err))
		}
		if pieceID == nil {
			pieceMap[piece.PieceID()] = size
			return nil
		}
		if piece.PieceID() == *pieceID {
			pieceMap[*pieceID] = size
			cancel()
		}
		return nil
	})

	if err != nil && !errs.Is(err, context.Canceled) {
		worker.log.Debug("failed to retrieve piece info", zap.Stringer("Satellite ID", worker.satelliteID), zap.Error(err))
	}

	var totalDeleted int64
	for id, size := range pieceMap {
		if size == 0 {
			continue
		}
		err := worker.store.Delete(ctx, worker.satelliteID, id)
		if err != nil {
			worker.log.Debug("failed to delete a piece",
				zap.Stringer("Satellite ID", worker.satelliteID),
				zap.Stringer("Piece ID", id),
				zap.Error(err))
			err = worker.store.DeleteFailed(ctx, pieces.ExpiredInfo{
				SatelliteID: worker.satelliteID,
				PieceID:     id,
				InPieceInfo: true,
			}, time.Now().UTC())
			if err != nil {
				worker.log.Debug("failed to mark a deletion failure for a piece",
					zap.Stringer("Satellite ID", worker.satelliteID),
					zap.Stringer("Piece ID", id),
					zap.Error(err))
			}
			continue
		}
		worker.log.Debug("delete piece",
			zap.Stringer("Satellite ID", worker.satelliteID),
			zap.Stringer("Piece ID", id))
		totalDeleted += size
	}

	// update transfer progress
	return worker.satelliteDB.UpdateGracefulExit(ctx, worker.satelliteID, totalDeleted)
}

func (worker *Worker) handleFailure(ctx context.Context, transferError pb.TransferFailed_Error, pieceID pb.PieceID, send func(*pb.StorageNodeMessage) error) {
	failure := &pb.StorageNodeMessage{
		Message: &pb.StorageNodeMessage_Failed{
			Failed: &pb.TransferFailed{
				OriginalPieceId: pieceID,
				Error:           transferError,
			},
		},
	}

	sendErr := send(failure)
	if sendErr != nil {
		worker.log.Error("unable to send failure.", zap.Stringer("Satellite ID", worker.satelliteID))
	}
}

// Close halts the worker.
func (worker *Worker) Close() error {
	worker.limiter.Wait()
	return nil
}
