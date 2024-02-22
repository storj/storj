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
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
)

// Worker is responsible for completing the graceful exit for a given satellite.
type Worker struct {
	log *zap.Logger

	service *Service

	dialer              rpc.Dialer
	satelliteURL        storj.NodeURL
	concurrentTransfers int
}

// NewWorker instantiates Worker.
func NewWorker(log *zap.Logger, service *Service, dialer rpc.Dialer, satelliteURL storj.NodeURL, config Config) *Worker {
	return &Worker{
		log:                 process.NamedLog(log, satelliteURL.String()),
		service:             service,
		dialer:              dialer,
		satelliteURL:        satelliteURL,
		concurrentTransfers: config.NumConcurrentTransfers,
	}
}

// Run calls the satellite endpoint, transfers pieces, validates, and responds with success or failure.
// It also marks the satellite finished once all the pieces have been transferred.
func (worker *Worker) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	worker.log.Debug("started")
	defer worker.log.Debug("finished")

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
			return errs.New("satellite has requested piece transfer, but piece-transfer-based graceful exit is no longer supported")

		case *pb.SatelliteMessage_DeletePiece:
			return errs.New("satellite has requested piece deletion, but piece-transfer-based graceful exit is no longer supported")

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

			err = worker.service.ExitCompleted(ctx, worker.satelliteURL.ID, exitCompletedBytes)
			if err != nil {
				return errs.Wrap(err)
			}

			return errs.Wrap(worker.service.DeleteSatelliteData(ctx, worker.satelliteURL.ID))
		default:
			// TODO handle err
			worker.log.Error("unknown graceful exit message.", zap.Stringer("Satellite ID", worker.satelliteURL.ID))
		}
	}
}
