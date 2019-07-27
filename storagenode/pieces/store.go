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
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

const (
	preallocSize = 4 * memory.MiB
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

	OrderLimit      *pb.OrderLimit
	UplinkPieceHash *pb.PieceHash
}

// ExpiredInfo is a fully namespaced piece id
type ExpiredInfo struct {
	SatelliteID storj.NodeID
	PieceID     storj.PieceID
	PieceSize   int64
	InPieceInfo bool
}

// PieceExpirationDB stores information about pieces with expiration dates.
type PieceExpirationDB interface {
	// GetExpired gets piece IDs that expire or have expired before the given time
	GetExpired(ctx context.Context, expiredAt time.Time, limit int64) ([]ExpiredInfo, error)
	// SetExpiration sets an expiration time for the given piece ID on the given satellite
	SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time) error
	// DeleteExpiration removes an expiration record for the given piece ID on the given satellite
	DeleteExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (found bool, err error)
	// DeleteFailed marks an expiration record as having experienced a failure in deleting the
	// piece from the disk
	DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, failedAt time.Time) error
}

// V0PieceInfoDB stores meta information about pieces stored with storage format V0 (where
// metadata goes in the "pieceinfo" table in the storagenodedb). The actual pieces are stored
// behind something providing the storage.Blobs interface.
type V0PieceInfoDB interface {
	// Get returns Info about a piece.
	Get(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) (*Info, error)
	// Delete deletes Info about a piece.
	Delete(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID) error
	// DeleteFailed marks piece deletion from disk failed
	DeleteFailed(ctx context.Context, satelliteID storj.NodeID, pieceID storj.PieceID, failedAt time.Time) error
	// GetExpired gets piece IDs stored with storage format V0 that expire or have expired
	// before the given time
	GetExpired(ctx context.Context, expiredAt time.Time, limit int64) ([]ExpiredInfo, error)
	// ForAllV0PieceIDsOwnedBySatellite executes doForEach for each locally stored piece, stored
	// with storage format V0 in the namespace of the given satellite. If doForEach returns a
	// non-nil error, ForAllV0PieceIDsOwnedBySatellite will stop iterating and return the error
	// immediately.
	ForAllV0PieceIDsOwnedBySatellite(ctx context.Context, blobStore storage.Blobs, satellite storj.NodeID, doForEach func(StoredPieceAccess) error) error
}

// V0PieceInfoDBForTest is like V0PieceInfoDB, but adds on the Add() method so
// that test environments with V0 piece data can be set up.
type V0PieceInfoDBForTest interface {
	V0PieceInfoDB

	// Add inserts Info to the database. This is only a valid thing to do, now,
	// during tests, to replicate the environment of a storage node not yet fully
	// migrated to V1 storage.
	Add(context.Context, *Info) error
}

// StoredPieceAccess allows inspection and manipulation of a piece during iteration with
// ForAllPieceIDsOwnedBySatellite-type methods
type StoredPieceAccess interface {
	storage.StoredBlobAccess

	// PieceID gives the pieceID of the piece
	PieceID() storj.PieceID
	// Satellite gives the nodeID of the satellite which owns the piece
	Satellite() (storj.NodeID, error)
	// ContentSize gives the size of the piece content (not including the piece header, if
	// applicable)
	ContentSize(ctx context.Context) (int64, error)
	// CreationTime returns the piece creation time as given in the original PieceHash (which is
	// likely not the same as the file mtime). For non-FormatV0 pieces, this requires opening
	// the file and unmarshaling the piece header. If exact precision is not required, ModTime()
	// may be a better solution.
	CreationTime(ctx context.Context) (time.Time, error)
	// ModTime returns a less-precise piece creation time than CreationTime, but is generally
	// much faster. For non-FormatV0 pieces, this gets the piece creation time from to the
	// filesystem instead of the piece header.
	ModTime(ctx context.Context) (time.Time, error)
}

// Store implements storing pieces onto a blob storage implementation.
type Store struct {
	log            *zap.Logger
	blobs          storage.Blobs
	v0PieceInfo    V0PieceInfoDB
	expirationInfo PieceExpirationDB
}

// NewStore creates a new piece store
func NewStore(log *zap.Logger, blobs storage.Blobs, v0PieceInfo V0PieceInfoDB, expirationInfo PieceExpirationDB) *Store {
	return &Store{
		log:            log,
		blobs:          blobs,
		v0PieceInfo:    v0PieceInfo,
		expirationInfo: expirationInfo,
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

	writer, err := NewWriter(blob, storage.MaxStorageFormatVersionSupported)
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

	reader, err := NewReader(blob)
	return reader, Error.Wrap(err)
}

// readerLocated returns a new piece reader for a located piece, which avoids the potential
// need to check multiple storage formats to find the right blob.
func (store *Store) readerLocated(ctx context.Context, pieceAccess StoredPieceAccess) (_ *Reader, err error) {
	defer mon.Task()(&ctx)(&err)
	blob, err := store.blobs.OpenLocated(ctx, pieceAccess)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, Error.Wrap(err)
	}

	reader, err := NewReader(blob)
	return reader, Error.Wrap(err)
}

// Delete deletes the specified piece.
func (store *Store) Delete(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID) (err error) {
	defer mon.Task()(&ctx)(&err)
	err = store.blobs.Delete(ctx, storage.BlobRef{
		Namespace: satellite.Bytes(),
		Key:       pieceID.Bytes(),
	})
	if err != nil {
		return Error.Wrap(err)
	}
	// delete records in both the piece_expirations and pieceinfo DBs, wherever we find it.
	// both of these calls should return no error if the requested record is not found.
	if store.expirationInfo != nil {
		_, err = store.expirationInfo.DeleteExpiration(ctx, satellite, pieceID)
	}
	if store.v0PieceInfo != nil {
		err = errs.Combine(err, store.v0PieceInfo.Delete(ctx, satellite, pieceID))
	}
	return Error.Wrap(err)
}

// GetV0PieceInfoDB returns this piece-store's reference to the V0 piece info DB (or nil,
// if this piece-store does not have one). This is ONLY intended for use with testing
// functionality.
func (store *Store) GetV0PieceInfoDB() V0PieceInfoDB {
	return store.v0PieceInfo
}

// ForAllPieceIDsOwnedBySatellite executes doForEach for each locally stored piece in the namespace of
// the given satellite which was created before the specified time. If doForEach returns a non-nil
// error, ForAllPieceIDsInNamespace will stop iterating and return the error immediately.
//
// Note that this method includes all locally stored pieces, both V0 and higher.
func (store *Store) ForAllPieceIDsOwnedBySatellite(ctx context.Context, satellite storj.NodeID, doForEach func(StoredPieceAccess) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	// first iterate over all in V1 storage, then all in V0
	err = store.blobs.ForAllKeysInNamespace(ctx, satellite.Bytes(), func(blobAccess storage.StoredBlobAccess) error {
		if blobAccess.StorageFormatVersion() < storage.FormatV1 {
			// we'll address this piece while iterating over the V0 pieces below.
			return nil
		}
		pieceAccess, err := newStoredPieceAccess(store, blobAccess)
		if err != nil {
			// something is wrong with internals; blob storage thinks this key was stored, but
			// it is not a valid PieceID.
			return err
		}
		return doForEach(pieceAccess)
	})
	if err == nil && store.v0PieceInfo != nil {
		err = store.v0PieceInfo.ForAllV0PieceIDsOwnedBySatellite(ctx, store.blobs, satellite, doForEach)
	}
	return err
}

// GetExpired gets piece IDs that are expired and were created before the given time
func (store *Store) GetExpired(ctx context.Context, expiredAt time.Time, limit int64) (_ []ExpiredInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	v1expired, err := store.expirationInfo.GetExpired(ctx, expiredAt, limit)
	if err != nil {
		return nil, err
	}
	v0expired, err := store.v0PieceInfo.GetExpired(ctx, expiredAt, limit)
	if err != nil {
		return nil, err
	}
	return append(v1expired, v0expired...), nil
}

// SetExpiration records an expiration time for the specified piece ID owned by the specified satellite
func (store *Store) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time) (err error) {
	return store.expirationInfo.SetExpiration(ctx, satellite, pieceID, expiresAt)
}

// DeleteFailed marks piece as a failed deletion.
func (store *Store) DeleteFailed(ctx context.Context, expired ExpiredInfo, when time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	if expired.InPieceInfo {
		return store.v0PieceInfo.DeleteFailed(ctx, expired.SatelliteID, expired.PieceID, when)
	}
	return store.expirationInfo.DeleteFailed(ctx, expired.SatelliteID, expired.PieceID, when)
}

// SpaceUsedForPieces returns the disk space used by all local pieces (both V0 and later).
// Important note: this metric does not include space used by piece headers, whereas
// storj/filestore/store.(*Store).SpaceUsed() includes all space used by the blobs.
func (store *Store) SpaceUsedForPieces(ctx context.Context) (int64, error) {
	satellites, err := store.getAllStoringSatellites(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, satellite := range satellites {
		spaceUsed, err := store.SpaceUsedBySatellite(ctx, satellite)
		if err != nil {
			return 0, err
		}
		total += spaceUsed
	}
	return total, nil
}

func (store *Store) getAllStoringSatellites(ctx context.Context) ([]storj.NodeID, error) {
	namespaces, err := store.blobs.GetAllNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	satellites := make([]storj.NodeID, len(namespaces))
	for i, namespace := range namespaces {
		satellites[i], err = storj.NodeIDFromBytes(namespace)
		if err != nil {
			return nil, err
		}
	}
	return satellites, nil
}

// SpaceUsedBySatellite calculates disk space used for local piece storage in the given
// satellite's namespace. Important note: this metric does not include space used by
// piece headers, whereas storj/filestore/store.(*Store).SpaceUsedInNamespace() does
// include all space used by the blobs.
func (store *Store) SpaceUsedBySatellite(ctx context.Context, satelliteID storj.NodeID) (int64, error) {
	var totalUsed int64
	err := store.ForAllPieceIDsOwnedBySatellite(ctx, satelliteID, func(access StoredPieceAccess) error {
		contentSize, statErr := access.ContentSize(ctx)
		if statErr != nil {
			store.log.Error("failed to stat", zap.Error(statErr), zap.String("pieceID", access.PieceID().String()), zap.String("satellite", satelliteID.String()))
			// keep iterating; we want a best effort total here.
			return nil
		}
		totalUsed += contentSize
		return nil
	})
	if err != nil {
		return 0, err
	}
	return totalUsed, nil
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

type storedPieceAccess struct {
	storage.StoredBlobAccess
	store   *Store
	pieceID storj.PieceID
}

func newStoredPieceAccess(store *Store, blobAccess storage.StoredBlobAccess) (storedPieceAccess, error) {
	pieceID, err := storj.PieceIDFromBytes(blobAccess.BlobRef().Key)
	if err != nil {
		return storedPieceAccess{}, err
	}
	return storedPieceAccess{
		StoredBlobAccess: blobAccess,
		store:            store,
		pieceID:          pieceID,
	}, nil
}

// PieceID returns the piece ID of the piece
func (access storedPieceAccess) PieceID() storj.PieceID {
	return access.pieceID
}

// Satellite returns the satellite ID that owns the piece
func (access storedPieceAccess) Satellite() (storj.NodeID, error) {
	return storj.NodeIDFromBytes(access.BlobRef().Namespace)
}

// ContentSize gives the size of the piece content (not including the piece header, if applicable)
func (access storedPieceAccess) ContentSize(ctx context.Context) (size int64, err error) {
	defer mon.Task()(&ctx)(&err)
	stat, err := access.Stat(ctx)
	if err != nil {
		return 0, err
	}
	size = stat.Size()
	if access.StorageFormatVersion() >= storage.FormatV1 {
		size -= V1PieceHeaderSize
	}
	return size, nil
}

// CreationTime returns the piece creation time as given in the original PieceHash (which is likely
// not the same as the file mtime). This requires opening the file and unmarshaling the piece
// header. If exact precision is not required, ModTime() may be a better solution.
func (access storedPieceAccess) CreationTime(ctx context.Context) (cTime time.Time, err error) {
	defer mon.Task()(&ctx)(&err)
	reader, err := access.store.readerLocated(ctx, access)
	if err != nil {
		return time.Time{}, err
	}
	header, err := reader.GetPieceHeader()
	if err != nil {
		return time.Time{}, err
	}
	return header.CreationTime, nil
}

// ModTime returns a less-precise piece creation time than CreationTime, but is generally
// much faster. This gets the piece creation time from to the filesystem instead of the
// piece header.
func (access storedPieceAccess) ModTime(ctx context.Context) (mTime time.Time, err error) {
	defer mon.Task()(&ctx)(&err)
	stat, err := access.Stat(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return stat.ModTime(), nil
}
