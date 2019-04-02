// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	readBufferSize  = 256 * memory.KiB
	writeBufferSize = 256 * memory.KiB
	preallocSize    = 4 * memory.MiB
)

// Error is the default error class.
var Error = errs.Class("pieces error")

// Info contains all the information we need to know about a Piece to manage them.
type Info struct {
	SatelliteID storj.NodeID

	PieceID         storj.PieceID
	PieceSize       int64
	PieceExpiration *time.Time

	UplinkPieceHash *pb.PieceHash
	Uplink          *identity.PeerIdentity
}

// DB stores meta information about a piece, the actual piece is stored in storage.Blobs
type DB interface {
	// Add inserts Info to the database.
	Add(context.Context, *Info) error
	// Get returns Info about a piece.
	Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (*Info, error)
	// Delete deletes Info about a piece.
	Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	// SpaceUsed calculates disk space used by all pieces
	SpaceUsed(ctx context.Context) (int64, error)
	// GetExpired gets orders that are expired and were created before some time
	GetExpired(ctx context.Context, expiredAt time.Time) ([]Info, error)
	// DeleteExpired deletes pieces that are expired
	DeleteExpired(context.Context, time.Time) error
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
func (store *Store) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (*Writer, error) {
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
func (store *Store) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (*Reader, error) {
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
func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) error {
	err := store.blobs.Delete(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	return Error.Wrap(err)
}

// StorageStatus contains information about the disk store is using.
type StorageStatus struct {
	DiskUsed int64
	DiskFree int64
}

// StorageStatus returns information about the disk.
func (store *Store) StorageStatus() (StorageStatus, error) {
	diskFree, err := store.blobs.FreeSpace()
	if err != nil {
		return StorageStatus{}, err
	}
	return StorageStatus{
		DiskUsed: -1, // TODO set value
		DiskFree: diskFree,
	}, nil
}
