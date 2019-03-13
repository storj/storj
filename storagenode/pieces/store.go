// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"storj.io/storj/internal/memory"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"

	_ "storj.io/storj/storage/filestore"
)

const (
	readBufferSize  = 256 * memory.KiB
	writeBufferSize = 256 * memory.KiB
	preallocSize    = 4 * memory.MiB
)

// Error is the default error class.
var Error = errs.Class("pieces error")

type Info struct {
	SatelliteID storj.NodeID

	PieceID         storj.PieceID2
	PieceSize       int64
	PieceExpiration time.Time

	UplinkPieceHash *pb.PieceHash
	UplinkIdentity  *identity.PeerIdentity
}

// Store implements storing pieces onto a blob storage implementation.
type Store struct {
	log   *zap.Logger
	blobs storage.Blobs
}

// NewStore creates a new piece store
func NewStore(log *zap.Logger, blobs storage.Blobs) *Store {
	return &Store{
		log:   log,
		blobs: blobs,
	}
}

// Writer returns a new piece writer.
func (store *Store) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) (*Writer, error) {
	blob, err := store.blobs.Create(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	}, preallocSize.Int64())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	writer, err := NewWriter(blob, writeBufferSize.Int())
	return writer, Error.Wrap(err)
}

// Reader returns a new piece reader.
func (store *Store) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) (*Reader, error) {
	blob, err := store.blobs.Open(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	reader, err := NewReader(blob, readBufferSize.Int())
	return reader, Error.Wrap(err)
}

// Delete deletes the specified piece.
func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) error {
	err := store.blobs.Delete(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	return Error.Wrap(err)
}

type StorageStatus struct {
	DiskUsed int64
	DiskFree int64
}

func (store *Store) StorageStatus() StorageStatus {
	panic("TODO")
}
