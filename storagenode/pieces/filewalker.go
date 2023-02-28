// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"os"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
)

// FileWalker implements methods to walk over pieces in a storage directory.
type FileWalker struct {
	log *zap.Logger

	blobs       blobstore.Blobs
	v0PieceInfo V0PieceInfoDB
}

// NewFileWalker creates a new FileWalker.
func NewFileWalker(log *zap.Logger, blobs blobstore.Blobs, db V0PieceInfoDB) *FileWalker {
	return &FileWalker{
		log:         log,
		blobs:       blobs,
		v0PieceInfo: db,
	}
}

// WalkSatellitePieces executes walkFunc for each locally stored piece in the namespace of the
// given satellite. If walkFunc returns a non-nil error, WalkSatellitePieces will stop iterating
// and return the error immediately. The ctx parameter is intended specifically to allow canceling
// iteration early.
//
// Note that this method includes all locally stored pieces, both V0 and higher.
func (fw *FileWalker) WalkSatellitePieces(ctx context.Context, satellite storj.NodeID, fn func(StoredPieceAccess) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	// iterate over all in V1 storage, skipping v0 pieces
	err = fw.blobs.WalkNamespace(ctx, satellite.Bytes(), func(blobInfo blobstore.BlobInfo) error {
		if blobInfo.StorageFormatVersion() < filestore.FormatV1 {
			// skip v0 pieces, which are handled separately
			return nil
		}
		pieceAccess, err := newStoredPieceAccess(fw.blobs, blobInfo)
		if err != nil {
			// this is not a real piece blob. the blob store can't distinguish between actual piece
			// blobs and stray files whose names happen to decode as valid base32. skip this
			// "blob".
			return nil //nolint: nilerr // we ignore other files
		}
		return fn(pieceAccess)
	})

	if err == nil && fw.v0PieceInfo != nil {
		// iterate over all in V0 storage
		err = fw.v0PieceInfo.WalkSatelliteV0Pieces(ctx, fw.blobs, satellite, fn)
	}

	return err
}

// WalkAndComputeSpaceUsedBySatellite walks over all pieces for a given satellite, adds up and returns the total space used.
func (fw *FileWalker) WalkAndComputeSpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (satPiecesTotal int64, satPiecesContentSize int64, err error) {
	err = fw.WalkSatellitePieces(ctx, satelliteID, func(access StoredPieceAccess) error {
		pieceTotal, pieceContentSize, err := access.Size(ctx)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		satPiecesTotal += pieceTotal
		satPiecesContentSize += pieceContentSize
		return nil
	})

	return satPiecesTotal, satPiecesContentSize, err
}
