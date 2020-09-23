// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package gracefulexit

import (
	"context"
	"database/sql"
	"errors"
	"os"

	"go.uber.org/zap"

	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

// BlobsCleaner checks for satellites that the node has completed exit successfully and clear blobs of it.
//
// architecture: Chore
type BlobsCleaner struct {
	log         *zap.Logger
	store       *pieces.Store
	satelliteDB satellites.DB
	trust       *trust.Pool
}

// NewBlobsCleaner instantiates BlobsCleaner.
func NewBlobsCleaner(log *zap.Logger, store *pieces.Store, trust *trust.Pool, satelliteDB satellites.DB) *BlobsCleaner {
	return &BlobsCleaner{
		log:         log,
		store:       store,
		satelliteDB: satelliteDB,
		trust:       trust,
	}
}

// RemoveBlobs runs blobs cleaner for satellites on which GE is completed.
// On node's restart checks if any of trusted satellites has GE status "successfully exited"
// Deletes blobs/satellite folder if exists, so if garbage collector didn't clean all SNO won't keep trash.
func (blobsCleaner *BlobsCleaner) RemoveBlobs(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	satelliteIDs := blobsCleaner.trust.GetSatellites(ctx)
	for i := 0; i < len(satelliteIDs); i++ {
		stats, err := blobsCleaner.satelliteDB.GetSatellite(ctx, satelliteIDs[i])
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			blobsCleaner.log.Error("couldn't receive satellite's GE status", zap.Error(err))
			return nil
		}

		if stats.Status == satellites.ExitSucceeded {
			err = blobsCleaner.store.DeleteSatelliteBlobs(ctx, satelliteIDs[i])
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}

				blobsCleaner.log.Error("couldn't delete blobs/satelliteID folder", zap.Error(err))
			}
		}
	}

	return nil
}
