// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
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
	satelliteID        storj.NodeID
	satelliteAddr      string
	ecclient           ecclient.Client
	minBytesPerSecond  memory.Size
	minDownloadTimeout time.Duration
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, store *pieces.Store, satelliteDB satellites.DB, dialer rpc.Dialer, satelliteID storj.NodeID, satelliteAddr string, choreConfig Config) *Worker {
	return &Worker{
		log:                log,
		store:              store,
		satelliteDB:        satelliteDB,
		dialer:             dialer,
		satelliteID:        satelliteID,
		satelliteAddr:      satelliteAddr,
		ecclient:           ecclient.NewClient(log, dialer, 0),
		minBytesPerSecond:  choreConfig.MinBytesPerSecond,
		minDownloadTimeout: choreConfig.MinDownloadTimeout,
	}
}

// Run calls the satellite endpoint, transfers pieces, validates, and responds with success or failure.
// It also marks the satellite finished once all the pieces have been transferred
// TODO handle transfers in parallel
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

	client := conn.SatelliteGracefulExitClient()

	c, err := client.Process(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	for {
		response, err := c.Recv()
		if errs.Is(err, io.EOF) {
			// Done
			break
		}
		if err != nil {
			// TODO what happened
			return errs.Wrap(err)
		}

		switch msg := response.GetMessage().(type) {
		case *pb.SatelliteMessage_NotReady:
			break // wait until next worker execution
		case *pb.SatelliteMessage_TransferPiece:
			err = worker.transferPiece(ctx, msg.TransferPiece, c)
			if err != nil {
				continue
			}
		case *pb.SatelliteMessage_DeletePiece:
			pieceID := msg.DeletePiece.OriginalPieceId
			err := worker.deleteOnePieceOrAll(ctx, &pieceID)
			if err != nil {
				worker.log.Error("failed to delete piece.", zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID), zap.Error(errs.Wrap(err)))
			}

		case *pb.SatelliteMessage_ExitFailed:
			worker.log.Error("graceful exit failed.", zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("reason", msg.ExitFailed.Reason))

			err = worker.satelliteDB.CompleteGracefulExit(ctx, worker.satelliteID, time.Now(), satellites.ExitFailed, msg.ExitFailed.GetExitFailureSignature())
			if err != nil {
				return errs.Wrap(err)
			}
			break
		case *pb.SatelliteMessage_ExitCompleted:
			worker.log.Info("graceful exit completed.", zap.Stringer("satellite ID", worker.satelliteID))

			err = worker.satelliteDB.CompleteGracefulExit(ctx, worker.satelliteID, time.Now(), satellites.ExitSucceeded, msg.ExitCompleted.GetExitCompleteSignature())
			if err != nil {
				return errs.Wrap(err)
			}
			// delete all remaining pieces
			err = worker.deleteOnePieceOrAll(ctx, nil)
			if err != nil {
				return errs.Wrap(err)
			}
			break
		default:
			// TODO handle err
			worker.log.Error("unknown graceful exit message.", zap.Stringer("satellite ID", worker.satelliteID))
		}

	}

	return errs.Wrap(err)
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
		worker.log.Error("failed to get piece reader.", zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID), zap.Error(errs.Wrap(err)))
		worker.handleFailure(ctx, transferErr, pieceID, c.Send)
		return err
	}

	addrLimit := transferPiece.GetAddressedOrderLimit()
	pk := transferPiece.PrivateKey

	originalHash, originalOrderLimit, err := worker.getHashAndLimit(ctx, reader, addrLimit.GetLimit())
	if err != nil {
		worker.log.Error("failed to get piece hash and order limit.", zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID), zap.Error(errs.Wrap(err)))
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
			worker.log.Error("failed hash verification.", zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID), zap.Error(errs.Wrap(err)))
			worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		} else {
			worker.log.Error("failed to put piece.", zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID), zap.Error(errs.Wrap(err)))
			// TODO look at error type to decide on the transfer error
			worker.handleFailure(ctx, pb.TransferFailed_STORAGE_NODE_UNAVAILABLE, pieceID, c.Send)
		}
		return err
	}

	if !bytes.Equal(originalHash.Hash, pieceHash.Hash) {
		worker.log.Error("piece hash from new storagenode does not match", zap.Stringer("storagenode ID", addrLimit.Limit.StorageNodeId), zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID))
		worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		return Error.New("piece hash from new storagenode does not match")
	}
	if pieceHash.PieceId != addrLimit.Limit.PieceId {
		worker.log.Error("piece id from new storagenode does not match order limit", zap.Stringer("storagenode ID", addrLimit.Limit.StorageNodeId), zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID))
		worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		return Error.New("piece id from new storagenode does not match order limit")
	}

	signee := signing.SigneeFromPeerIdentity(peerID)
	err = signing.VerifyPieceHashSignature(ctx, signee, pieceHash)
	if err != nil {
		worker.log.Error("invalid piece hash signature from new storagenode", zap.Stringer("storagenode ID", addrLimit.Limit.StorageNodeId), zap.Stringer("satellite ID", worker.satelliteID), zap.Stringer("piece ID", pieceID), zap.Error(errs.Wrap(err)))
		worker.handleFailure(ctx, pb.TransferFailed_HASH_VERIFICATION, pieceID, c.Send)
		return err
	}

	success := &pb.StorageNodeMessage{
		Message: &pb.StorageNodeMessage_Succeeded{
			Succeeded: &pb.TransferSucceeded{
				OriginalPieceId:      transferPiece.OriginalPieceId,
				OriginalPieceHash:    originalHash,
				OriginalOrderLimit:   originalOrderLimit,
				ReplacementPieceHash: pieceHash,
			},
		},
	}
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
			worker.log.Debug("failed to delete a piece", zap.Stringer("Satellite ID", worker.satelliteID), zap.Stringer("Piece ID", id), zap.Error(err))
			err = worker.store.DeleteFailed(ctx, pieces.ExpiredInfo{
				SatelliteID: worker.satelliteID,
				PieceID:     id,
				InPieceInfo: true,
			}, time.Now().UTC())
			if err != nil {
				worker.log.Debug("failed to mark a deletion failure for a piece", zap.Stringer("Satellite ID", worker.satelliteID), zap.Stringer("Piece ID", id), zap.Error(err))
			}
			continue
		}
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
		worker.log.Error("unable to send failure.", zap.Stringer("satellite ID", worker.satelliteID))
	}
}

// Close halts the worker.
func (worker *Worker) Close() error {
	// TODO not sure this is needed yet.
	return nil
}

// TODO This comes from piecestore.Endpoint. It should probably be an exported method so I don't have to duplicate it here.
func (worker *Worker) getHashAndLimit(ctx context.Context, pieceReader *pieces.Reader, limit *pb.OrderLimit) (pieceHash *pb.PieceHash, orderLimit *pb.OrderLimit, err error) {

	if pieceReader.StorageFormatVersion() == 0 {
		// v0 stores this information in SQL
		info, err := worker.store.GetV0PieceInfoDB().Get(ctx, limit.SatelliteId, limit.PieceId)
		if err != nil {
			worker.log.Error("error getting piece from v0 pieceinfo db", zap.Error(err))
			return nil, nil, err
		}
		orderLimit = info.OrderLimit
		pieceHash = info.UplinkPieceHash
	} else {
		//v1+ stores this information in the file
		header, err := pieceReader.GetPieceHeader()
		if err != nil {
			worker.log.Error("error getting header from piecereader", zap.Error(err))
			return nil, nil, err
		}
		orderLimit = &header.OrderLimit
		pieceHash = &pb.PieceHash{
			PieceId:   orderLimit.PieceId,
			Hash:      header.GetHash(),
			PieceSize: pieceReader.Size(),
			Timestamp: header.GetCreationTime(),
			Signature: header.GetSignature(),
		}
	}

	return
}
