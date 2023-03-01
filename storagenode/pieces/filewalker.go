// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/filestore"
)

// FileWalker implements methods to walk over pieces in a storage directory.
type FileWalker struct {
	log *zap.Logger

	blobs       storage.Blobs
	v0PieceInfo V0PieceInfoDB
}

// NewFileWalker creates a new FileWalker.
func NewFileWalker(log *zap.Logger, blobs storage.Blobs, db V0PieceInfoDB) *FileWalker {
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
	err = fw.blobs.WalkNamespace(ctx, satellite.Bytes(), func(blobInfo storage.BlobInfo) error {
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
