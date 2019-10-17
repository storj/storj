// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
	"storj.io/storj/uplink/ecclient"
)

// Worker is responsible for completing the graceful exit for a given satellite.
type Worker struct {
	log         *zap.Logger
	store       *pieces.Store
	satelliteDB satellites.DB
	trust       *trust.Pool
	dialer      rpc.Dialer
	identity    *identity.FullIdentity
	satelliteID storj.NodeID
	ecclient    ecclient.Client
}

type Sender interface {
	Send(*pb.StorageNodeMessage) error
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, store *pieces.Store, satelliteDB satellites.DB, trust *trust.Pool, dialer rpc.Dialer, identity *identity.FullIdentity, satelliteID storj.NodeID) *Worker {
	return &Worker{
		log:         log,
		store:       store,
		satelliteDB: satelliteDB,
		trust:       trust,
		dialer:      dialer,
		//identity:    identity,
		satelliteID: satelliteID,
		ecclient:    ecclient.NewClient(log, dialer, 0),
	}
}

// Run calls the satellite endpoint, transfers pieces, validates, and responds with success or failure.
// It also marks the satellite finished once all the pieces have been transferred
func (worker *Worker) Run(ctx context.Context, satelliteID storj.NodeID, done func()) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer done()

	worker.log.Debug("running worker")

	addr, err := worker.trust.GetAddress(ctx, satelliteID)
	if err != nil {
		return errs.Wrap(err)
	}

	conn, err := worker.dialer.DialAddressID(ctx, addr, satelliteID)
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
			pieceID := msg.TransferPiece.OriginalPieceId
			reader, err := worker.store.Reader(ctx, satelliteID, pieceID)

			if err != nil {
				transferErr := pb.TransferFailed_UNKNOWN
				if errs.Is(err, os.ErrNotExist) {
					transferErr = pb.TransferFailed_NOT_FOUND
				}
				worker.log.Error("failed to get piece reader.", zap.String("satellite ID", satelliteID.String()), zap.String("piece ID", pieceID.String()), zap.Error(errs.Wrap(err)))
				worker.handleFailure(ctx, transferErr, pieceID, c.Send)
				continue
			}

			limit := msg.TransferPiece.GetAddressedOrderLimit()
			pk := msg.TransferPiece.PrivateKey

			putCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			// TODO set real expiration
			pieceHash, err := worker.ecclient.PutPiece(putCtx, ctx, limit, pk, reader, time.Now().Add(time.Second*60))
			if err != nil {
				worker.log.Error("failed to put piece.", zap.String("satellite ID", satelliteID.String()), zap.String("piece ID", pieceID.String()), zap.Error(errs.Wrap(err)))
				// TODO look at error type to decide on the transfer error
				worker.handleFailure(ctx, pb.TransferFailed_STORAGE_NODE_UNAVAILABLE, pieceID, c.Send)

				continue
			}

			success := &pb.StorageNodeMessage{
				Message: &pb.StorageNodeMessage_Succeeded{
					Succeeded: &pb.TransferSucceeded{
						OriginalPieceId:      msg.TransferPiece.OriginalPieceId,
						OriginalPieceHash:    &pb.PieceHash{PieceId: msg.TransferPiece.OriginalPieceId},
						ReplacementPieceHash: pieceHash,
						AddressedOrderLimit:  msg.TransferPiece.AddressedOrderLimit,
					},
				},
			}
			err = c.Send(success)
			if err != nil {
				return errs.Wrap(err)
			}
		case *pb.SatelliteMessage_DeletePiece:
			pieceID := msg.DeletePiece.OriginalPieceId
			err := worker.store.Delete(ctx, satelliteID, pieceID)
			if err != nil {
				worker.log.Error("failed to delete piece.", zap.String("satellite ID", satelliteID.String()), zap.String("piece ID", pieceID.String()), zap.Error(errs.Wrap(err)))
			}
		case *pb.SatelliteMessage_ExitFailed:
			worker.log.Error("graceful exit failed.", zap.String("satellite ID", satelliteID.String()), zap.String("reason", msg.ExitFailed.Reason.String()))

			err = worker.satelliteDB.CompleteGracefulExit(ctx, satelliteID, time.Now(), satellites.ExitFailed, msg.ExitFailed.GetExitFailureSignature())
			if err != nil {
				return errs.Wrap(err)
			}
			break
		case *pb.SatelliteMessage_ExitCompleted:
			worker.log.Info("graceful exit completed.", zap.String("satellite ID", satelliteID.String()))

			err = worker.satelliteDB.CompleteGracefulExit(ctx, satelliteID, time.Now(), satellites.ExitSucceeded, msg.ExitCompleted.GetExitCompleteSignature())
			if err != nil {
				return errs.Wrap(err)
			}
			break
		default:
			// TODO handle err
		}

	}

	return errs.Wrap(err)
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
		// log it
	}
}

// Close halts the worker.
func (worker *Worker) Close() error {
	// TODO not sure this is needed yet.
	return nil
}
