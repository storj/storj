// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testblobs

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/blobstore"
)

// ensures that limitedSpaceDB implements storagenode.DB.
var _ storagenode.DB = (*limitedSpaceDB)(nil)

// limitedSpaceDB implements storage node DB with limited free space.
type limitedSpaceDB struct {
	storagenode.DB
	log   *zap.Logger
	blobs *LimitedSpaceBlobs
}

// NewLimitedSpaceDB creates a new storage node DB with limited free space.
func NewLimitedSpaceDB(log *zap.Logger, db storagenode.DB, freeSpace int64) storagenode.DB {
	return &limitedSpaceDB{
		DB:    db,
		blobs: newLimitedSpaceBlobs(log, db.Pieces(), freeSpace),
		log:   log,
	}
}

// Pieces returns the blob store.
func (lim *limitedSpaceDB) Pieces() blobstore.Blobs {
	return lim.blobs
}

// LimitedSpaceBlobs implements a limited space blob store.
type LimitedSpaceBlobs struct {
	blobstore.Blobs
	log       *zap.Logger
	freeSpace int64
}

// newLimitedSpaceBlobs creates a new limited space blob store wrapping the provided blobs.
func newLimitedSpaceBlobs(log *zap.Logger, blobs blobstore.Blobs, freeSpace int64) *LimitedSpaceBlobs {
	return &LimitedSpaceBlobs{
		log:       log,
		Blobs:     blobs,
		freeSpace: freeSpace,
	}
}

// FreeSpace returns how much free space left for writing.
func (limspace *LimitedSpaceBlobs) FreeSpace(ctx context.Context) (int64, error) {
	return limspace.freeSpace, nil
}

// DiskInfo returns the disk info.
func (limspace *LimitedSpaceBlobs) DiskInfo(ctx context.Context) (blobstore.DiskInfo, error) {
	return blobstore.DiskInfo{
		TotalSpace:     limspace.freeSpace,
		AvailableSpace: limspace.freeSpace,
	}, nil
}
