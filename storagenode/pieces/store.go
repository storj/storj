// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"

	_ "storj.io/storj/storage/filestore"
)

type Writer interface {
	Write(data []byte) (int64, error)
	Size() (int64, error)

	Hash() []byte
	Commit() error

	Cancel()
}

type Reader interface {
	ReadAt(offset int64, data []byte) error
	Size() (int64, error)
	Close() error
}

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

func (store *Store) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) (Writer, error) {
	panic("TODO")
}

func (store *Store) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) (Reader, error) {
	panic("TODO")
}

func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID2) error {
	panic("TODO")
}

type StorageStatus struct {
	DiskUsed int64
	DiskFree int64
}

func (store *Store) StorageStatus() StorageStatus {
	panic("TODO")
}
