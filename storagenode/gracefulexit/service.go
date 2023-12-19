// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Service exposes methods to manage GE progress.
//
// architecture: Service
type Service struct {
	log         *zap.Logger
	store       *pieces.Store
	trust       *trust.Pool
	satelliteDB satellites.DB

	nowFunc func() time.Time
}

// NewService is a constructor for a GE service.
func NewService(log *zap.Logger, store *pieces.Store, trust *trust.Pool, satelliteDB satellites.DB, dialer rpc.Dialer, config Config) *Service {
	return &Service{
		log:         log,
		store:       store,
		trust:       trust,
		satelliteDB: satelliteDB,
		nowFunc:     func() time.Time { return time.Now().UTC() },
	}
}

// ExitingSatellite encapsulates a node address with its graceful exit progress.
type ExitingSatellite struct {
	satellites.ExitProgress
	NodeURL storj.NodeURL
}

// ListPendingExits returns a slice with one record for every satellite
// from which this node is gracefully exiting. Each record includes the
// satellite's ID/address and information about the graceful exit status
// and progress.
func (c *Service) ListPendingExits(ctx context.Context) (_ []ExitingSatellite, err error) {
	defer mon.Task()(&ctx)(&err)

	exitProgress, err := c.satelliteDB.ListGracefulExits(ctx)
	if err != nil {
		return nil, err
	}
	exitingSatellites := make([]ExitingSatellite, 0, len(exitProgress))
	for _, sat := range exitProgress {
		if sat.FinishedAt != nil {
			continue
		}
		nodeURL, err := c.trust.GetNodeURL(ctx, sat.SatelliteID)
		if err != nil {
			c.log.Error("failed to get satellite address", zap.Stringer("Satellite ID", sat.SatelliteID), zap.Error(err))
			continue
		}
		exitingSatellites = append(exitingSatellites, ExitingSatellite{ExitProgress: sat, NodeURL: nodeURL})
	}
	return exitingSatellites, nil
}

// DeleteSatelliteData deletes all pieces and blobs stored for a satellite.
//
// Note: this should only ever be called after exit has finished.
func (c *Service) DeleteSatelliteData(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	// delete all remaining pieces
	err = c.deleteSatellitePieces(ctx, satelliteID)
	if err != nil {
		return errs.Wrap(err)
	}

	// delete everything left in blobs folder of specific satellites
	return c.store.DeleteSatelliteBlobs(ctx, satelliteID)
}

// deleteSatellitePieces deletes all pieces stored for a satellite, and updates
// the deleted byte count for the corresponding graceful exit operation.
func (c *Service) deleteSatellitePieces(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	var totalDeleted int64
	logger := c.log.With(zap.Stringer("Satellite ID", satelliteID), zap.String("action", "delete all pieces"))
	err = c.store.WalkSatellitePieces(ctx, satelliteID, func(piece pieces.StoredPieceAccess) error {
		err := c.store.Delete(ctx, satelliteID, piece.PieceID())
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			logger.Error("failed to delete piece",
				zap.Stringer("Piece ID", piece.PieceID()), zap.Error(err))
			// but continue
		}
		_, size, err := piece.Size(ctx)
		if err != nil {
			logger.Warn("failed to get piece size",
				zap.Stringer("Piece ID", piece.PieceID()), zap.Error(err))
			return nil
		}
		totalDeleted += size
		return nil
	})
	if err != nil && !errs2.IsCanceled(err) {
		logger.Error("failed to delete all pieces", zap.Error(err))
	}
	// update graceful exit progress
	return c.satelliteDB.UpdateGracefulExit(ctx, satelliteID, totalDeleted)
}

// ExitFailed updates the database when a graceful exit has failed.
func (c *Service) ExitFailed(ctx context.Context, satelliteID storj.NodeID, reason pb.ExitFailed_Reason, exitFailedBytes []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errs.Wrap(c.satelliteDB.CompleteGracefulExit(ctx, satelliteID, c.nowFunc(), satellites.ExitFailed, exitFailedBytes))
}

// ExitCompleted updates the database when a graceful exit is completed.
func (c *Service) ExitCompleted(ctx context.Context, satelliteID storj.NodeID, completionReceipt []byte) (err error) {
	defer mon.Task()(&ctx)(&err)
	return errs.Wrap(c.satelliteDB.CompleteGracefulExit(ctx, satelliteID, c.nowFunc(), satellites.ExitSucceeded, completionReceipt))
}

// ExitNotPossible deletes the entry for the corresponding graceful exit operation.
// This is intended to be called when a graceful exit operation was initiated but
// the satellite rejected it.
func (c *Service) ExitNotPossible(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return c.satelliteDB.CancelGracefulExit(ctx, satelliteID)
}
