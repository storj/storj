// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package forgetsatellite

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// Cleaner is responsible for cleaning up satellite data.
type Cleaner struct {
	log *zap.Logger

	store *pieces.Store
	trust *trust.Pool

	satelliteDB   satellites.DB
	reputationDB  reputation.DB
	v0PieceInfoDB pieces.V0PieceInfoDB
	usageCache    *pieces.BlobsUsageCache
	hsb           *piecestore.HashStoreBackend
}

// NewCleaner creates a new Cleaner.
func NewCleaner(log *zap.Logger, store *pieces.Store, trust *trust.Pool, usageCache *pieces.BlobsUsageCache, satelliteDB satellites.DB, reputationDB reputation.DB, v0PieceInfoDB pieces.V0PieceInfoDB, hsb *piecestore.HashStoreBackend) *Cleaner {
	return &Cleaner{
		log:           log,
		store:         store,
		trust:         trust,
		satelliteDB:   satelliteDB,
		reputationDB:  reputationDB,
		v0PieceInfoDB: v0PieceInfoDB,
		usageCache:    usageCache,
		hsb:           hsb,
	}
}

// Run runs the cleaner.
func (c *Cleaner) Run(ctx context.Context, satelliteID storj.NodeID) (err error) {
	defer mon.Task()(&ctx, satelliteID)(&err)

	logger := c.log.With(zap.Stringer("satelliteID", satelliteID))

	defer func() {
		if err != nil {
			logger.Error("cleanup failed", zap.Error(err))
			if err := c.satelliteDB.UpdateSatelliteStatus(ctx, satelliteID, satellites.CleanupFailed); err != nil {
				logger.Error("Failed to update satellite status.", zap.Error(err))
			}
		}
	}()

	satellite, err := c.satelliteDB.GetSatellite(ctx, satelliteID)
	if err != nil {
		logger.Error("failed to get satellite info", zap.Error(err))
		return err
	}

	if satellite.Status != satellites.CleanupInProgress {
		switch satellite.Status {
		case satellites.CleanupSucceeded:
			return errs.New("cleanup already completed for satellite")
		case satellites.CleanupFailed:
			return errs.New("cleanup already failed for satellite")
		default:
			return errs.New("forget-satellite not initiated for satellite")
		}
	}

	logger.Info("removing satellite from trust cache")
	err = c.trust.DeleteSatellite(ctx, satellite.SatelliteID)
	if err != nil {
		logger.Error("failed to remove satellite from trust cache", zap.Error(err))
		return err
	}

	logger.Info("cleaning up satellite data")
	if err := c.store.DeleteSatelliteBlobs(ctx, satellite.SatelliteID); err != nil {
		return err
	}

	logger.Info("cleaning up the trash")
	// To be sure that we update the usage cache before deleting the trash, we first
	// delete the trash by calling EmptyTrash because it updates the usage cache.
	// Then we delete the trash folder for the satellite.
	err = c.store.EmptyTrash(ctx, satellite.SatelliteID, time.Now())
	if err != nil {
		return err
	}
	err = c.usageCache.DeleteTrashNamespace(ctx, satellite.SatelliteID.Bytes())
	if err != nil {
		return err
	}

	logger.Info("removing satellite info from reputation DB")
	err = c.reputationDB.Delete(ctx, satellite.SatelliteID)
	if err != nil {
		return err
	}

	// delete v0 pieces for the satellite, if any.
	logger.Info("removing satellite v0 pieces if any")
	err = c.v0PieceInfoDB.WalkSatelliteV0Pieces(ctx, c.usageCache, satellite.SatelliteID, func(access pieces.StoredPieceAccess) error {
		return c.store.Delete(ctx, satelliteID, access.PieceID())
	})
	if err != nil {
		return err
	}

	err = c.hsb.ForgetSatellite(ctx, satellite.SatelliteID)
	if err != nil {
		return err
	}

	err = c.satelliteDB.UpdateSatelliteStatus(ctx, satellite.SatelliteID, satellites.CleanupSucceeded)
	if err != nil {
		return err
	}

	logger.Info("cleanup completed")

	return nil
}

// ListSatellites lists all satellites that are being cleaned up.
func (c *Cleaner) ListSatellites(ctx context.Context) (satelliteIDs []storj.NodeID, err error) {
	defer mon.Task()(&ctx)(&err)

	sats, err := c.satelliteDB.GetSatellites(ctx)
	if err != nil {
		return nil, err
	}

	for _, sat := range sats {
		if sat.Status == satellites.CleanupInProgress {
			satelliteIDs = append(satelliteIDs, sat.SatelliteID)
		}
	}

	return satelliteIDs, nil
}
