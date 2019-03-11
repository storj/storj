// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"

	"storj.io/storj/internal/memory"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"

	_ "storj.io/storj/storage/filestore"
)

const (
	readBufferSize  = 256 * memory.KiB
	writeBufferSize = 256 * memory.KiB
	preallocSize    = 4 * memory.MiB
)

var Error = errs.Class("pieces error")

type Store struct {
	log   *zap.Logger
	blobs storage.Blobs
}

func NewStore(log *zap.Logger, blobs storage.Blobs) *Store {
	return &Store{
		log:   log,
		blobs: blobs,
	}
}

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
