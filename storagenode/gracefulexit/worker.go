// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"io"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/piecetransfer"
)

// Worker is responsible for completing the graceful exit for a given satellite.
type Worker struct {
	log *zap.Logger

	service         Service
	transferService piecetransfer.Service

	dialer       rpc.Dialer
	limiter      *sync2.Limiter
	satelliteURL storj.NodeURL
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, service Service, transferService piecetransfer.Service, dialer rpc.Dialer, satelliteURL storj.NodeURL, config Config) *Worker {
	return &Worker{
		log:             log,
		service:         service,
		transferService: transferService,
		dialer:          dialer,
		limiter:         sync2.NewLimiter(config.NumConcurrentTransfers),
		satelliteURL:    satelliteURL,
	}
}

// Run calls the satellite endpoint, transfers pieces, validates, and responds with success or failure.
// It also marks the satellite finished once all the pieces have been transferred.
func (worker *Worker) Run(ctx context.Context, done func()) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer done()

	worker.log.Debug("running worker")

	conn, err := worker.dialer.DialNodeURL(ctx, worker.satelliteURL)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	client := pb.NewDRPCSatelliteGracefulExitClient(conn)

	c, err := client.Process(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = c.CloseSend() }()

	for {
		response, err := c.Recv()
		if errs.Is(err, io.EOF) {
			// Done
			return nil
		}
		if errs2.IsRPC(err, rpcstatus.FailedPrecondition) {
			// delete the entry from satellite table and inform graceful exit has failed to start
			deleteErr := worker.service.ExitNotPossible(ctx, worker.satelliteURL.ID)
			if deleteErr != nil {
				// TODO: what to do now?
				return errs.Combine(deleteErr, err)
			}
			return errs.Wrap(err)
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
				err := worker.service.DeletePiece(ctx, worker.satelliteURL.ID, pieceID)
				if err != nil {
					worker.log.Error("failed to delete piece.",
						zap.Stringer("Satellite ID", worker.satelliteURL.ID),
						zap.Stringer("Piece ID", pieceID),
						zap.Error(errs.Wrap(err)))
				}
			})

		case *pb.SatelliteMessage_ExitFailed:
			worker.log.Error("graceful exit failed.",
				zap.Stringer("Satellite ID", worker.satelliteURL.ID),
				zap.Stringer("reason", msg.ExitFailed.Reason))

			exitFailedBytes, err := pb.Marshal(msg.ExitFailed)
			if err != nil {
				worker.log.Error("failed to marshal exit failed message.")
			}

			return errs.Wrap(worker.service.ExitFailed(ctx, worker.satelliteURL.ID, msg.ExitFailed.Reason, exitFailedBytes))

		case *pb.SatelliteMessage_ExitCompleted:
			worker.log.Info("graceful exit completed.", zap.Stringer("Satellite ID", worker.satelliteURL.ID))

			exitCompletedBytes, err := pb.Marshal(msg.ExitCompleted)
			if err != nil {
				worker.log.Error("failed to marshal exit completed message.")
			}

			return errs.Wrap(worker.service.ExitCompleted(ctx, worker.satelliteURL.ID, exitCompletedBytes, worker.limiter.Wait))
		default:
			// TODO handle err
			worker.log.Error("unknown graceful exit message.", zap.Stringer("Satellite ID", worker.satelliteURL.ID))
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
		size, err := piece.ContentSize(ctxWithCancel)
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
