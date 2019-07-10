// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

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

var (
	// Error is the default error class.
	Error = errs.Class("pieces error")

	mon = monkit.Package()
)

// Info contains all the information we need to know about a Piece to manage them.
type Info struct {
	SatelliteID storj.NodeID

	PieceID         storj.PieceID
	PieceSize       int64
	PieceCreation   time.Time
	PieceExpiration time.Time

	UplinkPieceHash *pb.PieceHash
	Uplink          *identity.PeerIdentity
}

// ExpiredInfo is a fully namespaced piece id
type ExpiredInfo struct {
	SatelliteID storj.NodeID
	PieceID     storj.PieceID
	PieceSize   int64
}

// DB stores meta information about a piece, the actual piece is stored in storage.Blobs
type DB interface {
	// Add inserts Info to the database.
	Add(context.Context, *Info) error
	// Get returns Info about a piece.
	Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (*Info, error)
	// GetPieceIDs gets pieceIDs using the satelliteID
	GetPieceIDs(ctx context.Context, satelliteID storj.NodeID, createdBefore time.Time, limit, offset int) (pieceIDs []storj.PieceID, err error)
	// Delete deletes Info about a piece.
	Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	// DeleteFailed marks piece deletion from disk failed
	DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, failedAt time.Time) error
	// SpaceUsed returns the in memory value for disk space used by all pieces
	SpaceUsed(ctx context.Context) (int64, error)
	// CalculatedSpaceUsed calculates disk space used by all pieces
	CalculatedSpaceUsed(ctx context.Context) (int64, error)
	// SpaceUsedBySatellite calculates disk space used by all pieces by satellite
	SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (int64, error)
	// GetExpired gets orders that are expired and were created before some time
	GetExpired(ctx context.Context, expiredAt time.Time, limit int64) ([]ExpiredInfo, error)
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
func (store *Store) Writer(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ *Writer, err error) {
	defer mon.Task()(&ctx)(&err)
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
func (store *Store) Reader(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)
	blob, err := store.blobs.Open(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}

	reader, err := NewReader(blob, readBufferSize.Int())
	return reader, Error.Wrap(err)
}

// Delete deletes the specified piece.
func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.blobs.Delete(ctx, storage.BlobRef{
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
func (store *Store) StorageStatus(ctx context.Context) (_ StorageStatus, err error) {
	defer mon.Task()(&ctx)(&err)
	diskFree, err := store.blobs.FreeSpace()
	if err != nil {
		return StorageStatus{}, err
	}
	return StorageStatus{
		DiskUsed: -1, // TODO set value
		DiskFree: diskFree,
	}, nil
}
